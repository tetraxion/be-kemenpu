// Command migrate applies or rolls back the database migrations.
//
//	go run ./cmd/migrate            # up
//	go run ./cmd/migrate -down 1    # roll back the last step
//
// The server applies pending migrations by itself on startup; this command
// is for doing it separately, and for rolling back.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"irrigation-server/internal/pkg/config"
	"irrigation-server/internal/pkg/database"
	"irrigation-server/internal/pkg/logger"
)

func main() {
	down := flag.Int("down", 0, "number of migration steps to roll back (0 = migrate up)")
	flag.Parse()

	if err := run(*down); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
}

func run(down int) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	slog.SetDefault(logger.New(cfg.AppEnv))

	// The database may still be starting when this runs right after `db-up`.
	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := database.WaitReady(ctx, pool, cfg.DatabaseWait); err != nil {
		return err
	}

	if down > 0 {
		if err := database.MigrateDown(cfg.DatabaseURL, down); err != nil {
			return err
		}
		slog.Info("migration rolled back", "steps", down)
		return nil
	}
	if err := database.MigrateUp(cfg.DatabaseURL); err != nil {
		return err
	}
	slog.Info("migration finished")
	return nil
}
