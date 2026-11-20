// Command seed fills the database from the data folder.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"

	"irrigation-server/internal/features/ingest"
	"irrigation-server/internal/pkg/config"
	"irrigation-server/internal/pkg/database"
	"irrigation-server/internal/pkg/logger"
)

func main() {
	followUp := flag.Bool("follow-up", false, "also import "+ingest.FileFollowUp+" after the initial import")
	flag.Parse()

	if err := run(*followUp); err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}
}

func run(followUp bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New(cfg.AppEnv)
	slog.SetDefault(log)

	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := database.WaitReady(ctx, pool, cfg.DatabaseWait); err != nil {
		return err
	}
	if err := database.MigrateUp(cfg.DatabaseURL); err != nil {
		return err
	}

	dataDir := ingest.ResolveDataDir(cfg.DataDir, cfg.DataOverrideDir)
	log.Info("data folder selected", "dir", dataDir, "override", dataDir == cfg.DataOverrideDir)
	svc := ingest.NewService(ingest.NewRepository(pool), dataDir, log)
	return seed(ctx, svc, followUp)
}

func seed(ctx context.Context, svc ingest.Service, followUp bool) error {
	svc.Run(ctx)
	if svc.Status() != ingest.StatusReady {
		return errors.New("seed: initial import did not finish; see the log above")
	}
	if !followUp {
		return nil
	}
	_, err := svc.ImportFollowUp(ctx)
	return err
}
