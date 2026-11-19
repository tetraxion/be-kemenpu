package quality

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{pool: pool}
}

func (r *repository) Counts(ctx context.Context) (map[string]int, error) {
	rows, err := r.pool.Query(ctx, `SELECT category, count(*) FROM irrigation.finding GROUP BY category`)
	if err != nil {
		return nil, fmt.Errorf("quality: counts: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var category string
		var count int
		if err := rows.Scan(&category, &count); err != nil {
			return nil, fmt.Errorf("quality: scan count: %w", err)
		}
		out[category] = count
	}
	return out, rows.Err()
}

func (r *repository) Findings(ctx context.Context, category string) ([]Finding, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT source_file, row_ref, category, problem, action FROM irrigation.finding
		WHERE ($1 = '' OR category = $1) ORDER BY category, id`, category)
	if err != nil {
		return nil, fmt.Errorf("quality: findings: %w", err)
	}
	defer rows.Close()
	var out []Finding
	for rows.Next() {
		var f Finding
		if err := rows.Scan(&f.SourceFile, &f.RowRef, &f.Category, &f.Problem, &f.Action); err != nil {
			return nil, fmt.Errorf("quality: scan finding: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
