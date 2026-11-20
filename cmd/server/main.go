// Command server runs the irrigation network condition API.
//
//	@title				Irrigation Network Condition API
//	@version			1.0.0
//	@description		Combines the asset inventory, field reports and network map of an irrigation area into one picture of network condition, to help set maintenance priorities.
//	@BasePath			/
//	@schemes			http
//	@tag.name			health
//	@tag.description	Import status
//	@tag.name			assets
//	@tag.description	Irrigation structures: list, report history, downstream impact, maintenance priority
//	@tag.name			canals
//	@tag.description	Canal length recap
//	@tag.name			quality
//	@tag.description	Data-quality findings from the import
//	@tag.name			import
//	@tag.description	Follow-up report import
//	@tag.name			map
//	@tag.description	GeoJSON layers for the map (outside the assignment contract)
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"irrigation-server/internal/features/ingest"
	"irrigation-server/internal/pkg/config"
	"irrigation-server/internal/pkg/database"
	"irrigation-server/internal/pkg/logger"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New(cfg.AppEnv)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	dataDir := ingest.ResolveDataDir(cfg.DataDir, cfg.DataOverrideDir)
	log.Info("data folder selected", "dir", dataDir, "override", dataDir == cfg.DataOverrideDir)
	ingestSvc := ingest.NewService(ingest.NewRepository(pool), dataDir, log)
	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: newRouter(cfg, log, pool, ingestSvc),
		// Slow clients must not hold a connection forever.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if cfg.RequestTimeout > 0 {
		// Just behind RequestTimeout, so the 504 can still be sent.
		srv.WriteTimeout = cfg.RequestTimeout + 10*time.Second
	}

	importDone := make(chan struct{})
	go func() {
		defer close(importDone)
		startImport(ctx, log, cfg, pool, ingestSvc)
	}()

	errCh := make(chan error, 1)
	go func() {
		log.Info("server running", "addr", cfg.HTTPAddr, "env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	err = srv.Shutdown(shutdownCtx)

	// Wait for the import before the pool closes.
	select {
	case <-importDone:
	case <-shutdownCtx.Done():
		log.Warn("import did not stop before the shutdown deadline")
	}
	return err
}

func startImport(ctx context.Context, log *slog.Logger, cfg config.Config, pool *pgxpool.Pool, svc ingest.Service) {
	if err := database.WaitReady(ctx, pool, cfg.DatabaseWait); err != nil {
		log.Error("database unavailable", "error", err)
		svc.MarkFailed()
		return
	}
	if err := database.MigrateUp(cfg.DatabaseURL); err != nil {
		log.Error("migration failed", "error", err)
		svc.MarkFailed()
		return
	}
	svc.Run(ctx)
}
