package plot

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

func (r *repository) Features(ctx context.Context) ([]geojson.Feature, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT json_build_object('kode_petak', code, 'nama_petak', name,
			'luas_ha', round(area_ha::numeric, 2),
			'luas_baku_ha', round(attribute_area_ha::numeric, 2),
			'luas_geometri_ha', round(geometry_area_ha::numeric, 2),
			'nomenklatur_sadap', offtake_nomenclature, 'kode_aset_sadap', offtake_asset_code, 'tanda', flags)::text,
			ST_AsGeoJSON(geom, 7)
		FROM irrigation.plot ORDER BY code COLLATE "C"`)
	if err != nil {
		return nil, fmt.Errorf("plot: features: %w", err)
	}
	defer rows.Close()
	var out []geojson.Feature
	for rows.Next() {
		var properties string
		var geometry *string
		if err := rows.Scan(&properties, &geometry); err != nil {
			return nil, fmt.Errorf("plot: scan feature: %w", err)
		}
		out = append(out, geojson.NewFeature(properties, geometry))
	}
	return out, rows.Err()
}
