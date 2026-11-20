package canal

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"irrigation-server/internal/pkg/geojson"
)

type repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{pool: pool}
}

// unknown keeps unnormalised levels and conditions visible in the recap.
const unknown = "tidak_diketahui"

func (r *repository) Recap(ctx context.Context) ([]Recap, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT COALESCE(level, $1), COALESCE(condition, $1), sum(length_m) / 1000
		FROM irrigation.canal_segment
		GROUP BY level, condition
		ORDER BY COALESCE(level, $1),
			array_position(ARRAY['baik', 'rusak_ringan', 'rusak_sedang', 'rusak_berat'], condition) NULLS LAST`, unknown)
	if err != nil {
		return nil, fmt.Errorf("canal: recap: %w", err)
	}
	defer rows.Close()
	var out []Recap
	for rows.Next() {
		var rc Recap
		if err := rows.Scan(&rc.Level, &rc.Condition, &rc.LengthKm); err != nil {
			return nil, fmt.Errorf("canal: scan recap: %w", err)
		}
		out = append(out, rc)
	}
	return out, rows.Err()
}

func (r *repository) Features(ctx context.Context) ([]geojson.Feature, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT json_build_object('id_ruas', segment_code, 'nama_saluran', canal_name, 'tingkat', level,
			'kondisi', condition, 'panjang_m', round(length_m::numeric, 1))::text,
			ST_AsGeoJSON(geom, 7)
		FROM irrigation.canal_segment ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("canal: features: %w", err)
	}
	defer rows.Close()
	var out []geojson.Feature
	for rows.Next() {
		var properties string
		var geometry *string
		if err := rows.Scan(&properties, &geometry); err != nil {
			return nil, fmt.Errorf("canal: scan feature: %w", err)
		}
		out = append(out, geojson.NewFeature(properties, geometry))
	}
	return out, rows.Err()
}
