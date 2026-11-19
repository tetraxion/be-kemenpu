package asset

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{pool: pool}
}

const columns = `code, nomenclature, kind, canal_name, condition,
	to_char(condition_date, 'YYYY-MM-DD'), ST_X(geom), ST_Y(geom), flags`

func scan(row pgx.Row) (Asset, error) {
	var a Asset
	err := row.Scan(&a.Code, &a.Nomenclature, &a.Kind, &a.CanalName, &a.Condition, &a.ConditionDate, &a.Lon, &a.Lat, &a.Flags)
	return a, err
}

func (r *repository) List(ctx context.Context, f ListFilter) ([]Asset, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+columns+` FROM irrigation.asset
		WHERE ($1 = '' OR kind = $1) AND ($2 = '' OR condition = $2) AND ($3 = '' OR lower(canal_name) = lower($3))
		ORDER BY code COLLATE "C"`, f.Kind, f.Condition, f.CanalName)
	if err != nil {
		return nil, fmt.Errorf("asset: list: %w", err)
	}
	defer rows.Close()
	var out []Asset
	for rows.Next() {
		a, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("asset: scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *repository) Get(ctx context.Context, code string) (Asset, error) {
	a, err := scan(r.pool.QueryRow(ctx, `SELECT `+columns+` FROM irrigation.asset WHERE code = $1`, code))
	if errors.Is(err, pgx.ErrNoRows) {
		return Asset{}, ErrNotFound
	}
	if err != nil {
		return Asset{}, fmt.Errorf("asset: get: %w", err)
	}
	return a, nil
}

func (r *repository) Reports(ctx context.Context, code string) ([]Report, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, to_char(reported_on, 'YYYY-MM-DD'), officer, nomenclature, damage_type, condition, lat, lon, status, note
		FROM irrigation.report WHERE asset_code = $1 AND NOT is_duplicate
		ORDER BY reported_on DESC NULLS LAST, id COLLATE "C" DESC`, code)
	if err != nil {
		return nil, fmt.Errorf("asset: reports: %w", err)
	}
	defer rows.Close()
	var out []Report
	for rows.Next() {
		var rep Report
		if err := rows.Scan(&rep.ID, &rep.ReportedOn, &rep.Officer, &rep.Nomenclature, &rep.DamageType, &rep.Condition,
			&rep.Lat, &rep.Lon, &rep.Status, &rep.Note); err != nil {
			return nil, fmt.Errorf("asset: scan report: %w", err)
		}
		out = append(out, rep)
	}
	return out, rows.Err()
}

func (r *repository) ImpactedPlots(ctx context.Context, code string) ([]ImpactedPlot, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.code, p.area_ha FROM irrigation.impact i JOIN irrigation.plot p ON p.code = i.plot_code
		WHERE i.asset_code = $1 ORDER BY p.code COLLATE "C"`, code)
	if err != nil {
		return nil, fmt.Errorf("asset: impacted plots: %w", err)
	}
	defer rows.Close()
	var out []ImpactedPlot
	for rows.Next() {
		var p ImpactedPlot
		if err := rows.Scan(&p.Code, &p.AreaHa); err != nil {
			return nil, fmt.Errorf("asset: scan impacted plot: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *repository) PriorityCandidates(ctx context.Context) ([]PriorityCandidate, error) {
	rows, err := r.pool.Query(ctx, `SELECT code, condition, impacted_area_ha FROM irrigation.asset
		WHERE condition IS NOT NULL AND impacted_area_ha > 0`)
	if err != nil {
		return nil, fmt.Errorf("asset: priority candidates: %w", err)
	}
	defer rows.Close()
	var out []PriorityCandidate
	for rows.Next() {
		var c PriorityCandidate
		if err := rows.Scan(&c.Code, &c.Condition, &c.ImpactedAreaHa); err != nil {
			return nil, fmt.Errorf("asset: scan priority candidate: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
