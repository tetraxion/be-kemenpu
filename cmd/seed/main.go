// Command seed fills the database from the data folder.
//
//	go run ./cmd/seed               # import the four files read at startup
//	go run ./cmd/seed -follow-up    # then also import laporan_susulan.xlsx
//
// It runs the same import as the server does on startup: the schema content
// is replaced by what the files describe, so it is safe to repeat. Use it to
// load data without starting the HTTP server, or to bring a database to the
// state after the follow-up import.
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

	svc := ingest.NewService(ingest.NewRepository(pool), cfg.DataDir, log)
	return seed(ctx, svc, followUp)
}

// seed runs the initial import and, when asked, the follow-up import. The
// service logs the outcome of each; this only decides the exit status.
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
