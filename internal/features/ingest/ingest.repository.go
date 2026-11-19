package ingest

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"irrigation-server/internal/pkg/database"
)

type repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{pool: pool}
}

func (r *repository) RepairPolygon(ctx context.Context, geojson string) (PolygonRepair, error) {
	var out PolygonRepair
	// ST_MakeValid can return lines or points for degenerate parts; only the polygons are kept.
	err := r.pool.QueryRow(ctx, `
		WITH source AS (SELECT ST_SetSRID(ST_GeomFromGeoJSON($1::text), 4326) AS g),
		     fixed AS (SELECT ST_IsValid(g) AS was_valid, ST_Multi(ST_CollectionExtract(ST_MakeValid(g), 3)) AS m FROM source)
		SELECT was_valid, encode(ST_AsEWKB(m), 'hex'), ST_Area(ST_Transform(m, 32749)) FROM fixed`,
		geojson).Scan(&out.WasValid, &out.GeomHex, &out.AreaM2)
	if err != nil {
		return PolygonRepair{}, fmt.Errorf("ingest: repair polygon: %w", err)
	}
	return out, nil
}

// nullable turns "" into SQL NULL.
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// flagList keeps an empty list as '{}' instead of NULL.
func flagList(flags []string) []string {
	if flags == nil {
		return []string{}
	}
	return flags
}

func (r *repository) ReplaceAll(ctx context.Context, snap Snapshot) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `TRUNCATE irrigation.canal_segment, irrigation.impact,
			irrigation.report, irrigation.plot, irrigation.asset, irrigation.finding RESTART IDENTITY`); err != nil {
			return fmt.Errorf("ingest: truncate: %w", err)
		}
		for _, c := range snap.Canals {
			if _, err := tx.Exec(ctx, `
				INSERT INTO irrigation.canal_segment (segment_code, canal_name, level, condition, length_m, geom)
				VALUES ($1, $2, $3, $4, $5, ST_SetSRID(ST_GeomFromGeoJSON($6::text), 4326))`,
				c.SegmentCode, c.CanalName, nullable(c.Level), nullable(c.Condition), c.LengthM, c.GeoJSON()); err != nil {
				return fmt.Errorf("ingest: insert canal segment %s: %w", c.SegmentCode, err)
			}
		}
		for _, a := range snap.Assets {
			var lon, lat *float64
			if a.Location != nil {
				lon, lat = &a.Location.Lon, &a.Location.Lat
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO irrigation.asset (code, nomenclature, kind, canal_name, built_year, inventory_condition,
					inventory_date, geom, canal_distance_m, flags, impacted_area_ha)
				VALUES ($1, $2, $3, $4, $5, $6, $7::text::date,
					CASE WHEN $8::float8 IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($8::float8, $9::float8), 4326) END,
					$10, $11, $12)`,
				a.Code, nullable(a.Nomenclature), a.Kind, nullable(a.CanalName), a.BuiltYear, nullable(a.InventoryCondition),
				nullable(a.InventoryDate), lon, lat, a.CanalDistanceM, flagList(a.Flags), a.ImpactedAreaHa); err != nil {
				return fmt.Errorf("ingest: insert asset %s: %w", a.Code, err)
			}
		}
		for _, p := range snap.Plots {
			var geometryArea *float64
			if p.GeometryAreaHa > 0 {
				geometryArea = &p.GeometryAreaHa
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO irrigation.plot (code, name, attribute_area_ha, geometry_area_ha, area_ha,
					offtake_nomenclature, offtake_asset_code, flags, geom)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::text::geometry)`,
				p.Code, nullable(p.Name), p.AttributeAreaHa, geometryArea, p.AreaHa, nullable(p.OfftakeNomenclature),
				nullable(p.OfftakeAssetCode), flagList(p.Flags), nullable(p.GeomHex)); err != nil {
				return fmt.Errorf("ingest: insert plot %s: %w", p.Code, err)
			}
		}
		for _, a := range snap.Assets {
			for _, plotCode := range a.ImpactedPlots {
				if _, err := tx.Exec(ctx, `INSERT INTO irrigation.impact (asset_code, plot_code) VALUES ($1, $2)
					ON CONFLICT DO NOTHING`, a.Code, plotCode); err != nil {
					return fmt.Errorf("ingest: insert impact %s/%s: %w", a.Code, plotCode, err)
				}
			}
		}
		for _, report := range snap.Reports {
			if err := upsertReport(ctx, tx, report); err != nil {
				return err
			}
		}
		if err := insertFindings(ctx, tx, snap.Findings); err != nil {
			return err
		}
		return recompute(ctx, tx)
	})
}

func (r *repository) FollowUp(ctx context.Context, decide func(ref Reference, existing map[string]Report) FollowUpChange) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		// Serialise follow-up imports: two concurrent calls must not both see
		// the same "existing" reports and both count them as added.
		if _, err := tx.Exec(ctx, `LOCK TABLE irrigation.report IN SHARE ROW EXCLUSIVE MODE`); err != nil {
			return fmt.Errorf("ingest: lock report: %w", err)
		}
		ref, err := loadReference(ctx, tx)
		if err != nil {
			return err
		}
		existing, err := loadReports(ctx, tx)
		if err != nil {
			return err
		}
		change := decide(ref, existing)
		for _, report := range change.Upserts {
			if err := upsertReport(ctx, tx, report); err != nil {
				return err
			}
		}
		// The file's cleaning findings are replaced as a whole, so repeating
		// the import does not pile them up.
		if _, err := tx.Exec(ctx, `DELETE FROM irrigation.finding WHERE source_file = $1`, change.SourceFile); err != nil {
			return fmt.Errorf("ingest: delete findings: %w", err)
		}
		if err := insertFindings(ctx, tx, change.Findings); err != nil {
			return err
		}
		return recompute(ctx, tx)
	})
}

func upsertReport(ctx context.Context, db database.DBTX, r Report) error {
	_, err := db.Exec(ctx, `
		INSERT INTO irrigation.report (id, reported_on, officer, nomenclature, raw_nomenclature, damage_type,
			condition, lat, lon, status, note, asset_code, source_file)
		VALUES ($1, $2::text::date, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (id) DO UPDATE SET
			reported_on = EXCLUDED.reported_on, officer = EXCLUDED.officer, nomenclature = EXCLUDED.nomenclature,
			raw_nomenclature = EXCLUDED.raw_nomenclature, damage_type = EXCLUDED.damage_type,
			condition = EXCLUDED.condition, lat = EXCLUDED.lat, lon = EXCLUDED.lon, status = EXCLUDED.status,
			note = EXCLUDED.note, asset_code = EXCLUDED.asset_code, source_file = EXCLUDED.source_file`,
		r.ID, nullable(r.ReportedOn), r.Officer, r.Nomenclature, r.RawNomenclature, r.DamageType,
		nullable(r.Condition), r.Lat, r.Lon, r.Status, r.Note, nullable(r.AssetCode), r.SourceFile)
	if err != nil {
		return fmt.Errorf("ingest: upsert report %s: %w", r.ID, err)
	}
	return nil
}

func insertFindings(ctx context.Context, tx pgx.Tx, findings []Finding) error {
	rows := make([][]any, len(findings))
	for i, f := range findings {
		rows[i] = []any{f.SourceFile, f.RowRef, f.Category, f.Problem, f.Action}
	}
	_, err := tx.CopyFrom(ctx, pgx.Identifier{"irrigation", "finding"},
		[]string{"source_file", "row_ref", "category", "problem", "action"}, pgx.CopyFromRows(rows))
	if err != nil {
		return fmt.Errorf("ingest: insert findings: %w", err)
	}
	return nil
}

// recomputeSteps derive everything that depends on the report table. They run
// after the initial import and after every follow-up import, so the result
// depends only on the table content, never on the order of earlier imports.
var recomputeSteps = []string{
	// Duplicate reports: same asset, date and condition (within the same
	// status). The smallest id is kept.
	`UPDATE irrigation.report SET is_duplicate = false WHERE is_duplicate`,
	`UPDATE irrigation.report r SET is_duplicate = true FROM (
		SELECT id, row_number() OVER (
			PARTITION BY asset_code, reported_on, condition, status ORDER BY id COLLATE "C") AS position
		FROM irrigation.report
		WHERE asset_code IS NOT NULL AND reported_on IS NOT NULL AND condition IS NOT NULL) d
	 WHERE d.id = r.id AND d.position > 1`,
	// Latest condition: the most recent source between the inventory and the
	// active reports of the asset. On equal dates the report wins.
	`UPDATE irrigation.asset a SET
		condition = COALESCE(latest.condition, a.inventory_condition),
		condition_date = COALESCE(latest.reported_on, a.inventory_date)
	 FROM (
		SELECT a2.code, x.condition, x.reported_on
		FROM irrigation.asset a2 LEFT JOIN LATERAL (
			SELECT r.condition, r.reported_on FROM irrigation.report r
			WHERE r.asset_code = a2.code AND r.status = 'aktif' AND NOT r.is_duplicate
			  AND r.condition IS NOT NULL AND r.reported_on IS NOT NULL
			  AND (a2.inventory_date IS NULL OR a2.inventory_condition IS NULL OR r.reported_on >= a2.inventory_date)
			ORDER BY r.reported_on DESC, r.id COLLATE "C" DESC LIMIT 1) x ON true) latest
	 WHERE latest.code = a.code`,
	`DELETE FROM irrigation.finding WHERE category IN ('` + CategoryDuplicateReport + `', '` + CategoryReportNoAsset + `')`,
	`INSERT INTO irrigation.finding (source_file, row_ref, category, problem, action)
	 SELECT source_file, id, '` + CategoryDuplicateReport + `', 'Aset, tanggal, dan kondisi sama dengan laporan lain',
		'Dibuang, satu laporan disimpan'
	 FROM irrigation.report WHERE is_duplicate ORDER BY id COLLATE "C"`,
	`INSERT INTO irrigation.finding (source_file, row_ref, category, problem, action)
	 SELECT source_file, id, '` + CategoryReportNoAsset + `',
		'Nomenklatur ' || COALESCE(NULLIF(raw_nomenclature, ''), '(kosong)') ||
		' tidak dikenal dan tidak ada bangunan dalam 30 m dari koordinat laporan',
		'Disimpan dengan tanda laporan_tanpa_aset'
	 FROM irrigation.report WHERE asset_code IS NULL ORDER BY id COLLATE "C"`,
}

func recompute(ctx context.Context, db database.DBTX) error {
	for _, step := range recomputeSteps {
		if _, err := db.Exec(ctx, step); err != nil {
			return fmt.Errorf("ingest: recompute: %w", err)
		}
	}
	return nil
}

func loadReference(ctx context.Context, db database.DBTX) (Reference, error) {
	rows, err := db.Query(ctx, `SELECT code, COALESCE(nomenclature, ''), ST_X(geom), ST_Y(geom)
		FROM irrigation.asset ORDER BY code COLLATE "C"`)
	if err != nil {
		return Reference{}, fmt.Errorf("ingest: load assets: %w", err)
	}
	defer rows.Close()
	var assets []*Asset
	for rows.Next() {
		a := &Asset{}
		var lon, lat *float64
		if err := rows.Scan(&a.Code, &a.Nomenclature, &lon, &lat); err != nil {
			return Reference{}, fmt.Errorf("ingest: scan asset: %w", err)
		}
		if lon != nil && lat != nil {
			a.SetLocation(*lon, *lat)
		}
		assets = append(assets, a)
	}
	return NewReference(assets), rows.Err()
}

func loadReports(ctx context.Context, db database.DBTX) (map[string]Report, error) {
	rows, err := db.Query(ctx, `
		SELECT id, COALESCE(to_char(reported_on, 'YYYY-MM-DD'), ''), COALESCE(officer, ''),
			COALESCE(nomenclature, ''), COALESCE(damage_type, ''), COALESCE(condition, ''),
			lat, lon, status, COALESCE(note, ''), COALESCE(asset_code, '')
		FROM irrigation.report`)
	if err != nil {
		return nil, fmt.Errorf("ingest: load reports: %w", err)
	}
	defer rows.Close()
	reports := map[string]Report{}
	for rows.Next() {
		var r Report
		if err := rows.Scan(&r.ID, &r.ReportedOn, &r.Officer, &r.Nomenclature, &r.DamageType, &r.Condition,
			&r.Lat, &r.Lon, &r.Status, &r.Note, &r.AssetCode); err != nil {
			return nil, fmt.Errorf("ingest: scan report: %w", err)
		}
		reports[r.ID] = r
	}
	return reports, rows.Err()
}
