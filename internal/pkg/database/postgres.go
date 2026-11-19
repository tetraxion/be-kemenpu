// Package database provides the PostgreSQL (pgx) connection and transaction helper.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is satisfied by both *pgxpool.Pool and pgx.Tx, so a query helper can
// run inside or outside a transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// NewPool creates the pool without connecting. The HTTP server has to answer
// GET /api/health with "memuat" from the first second, so it must not wait
// for PostgreSQL; WaitReady does that in the background.
func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("database: parse url: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database: create pool: %w", err)
	}
	return pool, nil
}

// WaitReady pings until PostgreSQL accepts connections or the timeout passes.
func WaitReady(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		pingCtx, cancelPing := context.WithTimeout(ctx, 3*time.Second)
		err := pool.Ping(pingCtx)
		cancelPing()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("database: not ready after %s: %w", timeout, err)
		case <-time.After(time.Second):
		}
	}
}

// WithTx runs fn in one transaction and rolls back when fn returns an error.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("database: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
