//go:build integration

package ingest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"irrigation-server/internal/pkg/database"
	"irrigation-server/internal/pkg/geo"
)

// These tests need a PostgreSQL with PostGIS:
//
//	TEST_DATABASE_URL=postgres://... go test -tags integration -p 1 ./...
//
// They own the irrigation schema of that database and rewrite its content.

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	require.NoError(t, database.MigrateUp(url))
	pool, err := database.NewPool(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// stateHash fingerprints everything the API can show. Finding ids are left
// out: findings are re-inserted on each import and only their content matters.
func stateHash(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var hash string
	err := pool.QueryRow(context.Background(), `
		SELECT md5(concat_ws('#',
			(SELECT string_agg(a::text, '|' ORDER BY a.code) FROM irrigation.asset a),
			(SELECT string_agg(r::text, '|' ORDER BY r.id) FROM irrigation.report r),
			(SELECT string_agg(f.line, '|' ORDER BY f.line) FROM (
				SELECT concat_ws('~', source_file, row_ref, category, problem, action) AS line FROM irrigation.finding) f),
			(SELECT string_agg(i::text, '|' ORDER BY i.asset_code, i.plot_code) FROM irrigation.impact i)))`).Scan(&hash)
	require.NoError(t, err)
	return hash
}

func TestMigrationsGoUpDownAndUpAgain(t *testing.T) {
	testPool(t)
	url := os.Getenv("TEST_DATABASE_URL")

	require.NoError(t, database.MigrateDown(url, 1))
	require.NoError(t, database.MigrateUp(url))
	require.NoError(t, database.MigrateUp(url), "a second up is a no-op")
}

func TestRepairPolygon(t *testing.T) {
	repo := NewRepository(testPool(t))
	ctx := context.Background()
	ring := func(offsets ...[2]float64) string {
		var positionsList [][2]float64
		for _, o := range offsets {
			p := wgs(o[0], o[1])
			positionsList = append(positionsList, [2]float64{p.Lon, p.Lat})
		}
		return geometryJSON("MultiPolygon", [][][][2]float64{{positionsList}})
	}

	square, err := repo.RepairPolygon(ctx, ring([2]float64{0, 0}, [2]float64{640, 0}, [2]float64{640, 640}, [2]float64{0, 640}, [2]float64{0, 0}))
	require.NoError(t, err)
	assert.True(t, square.WasValid)
	assert.InDelta(t, 409600, square.AreaM2, 1, "area is measured on EPSG:32749")
	assert.NotEmpty(t, square.GeomHex)

	// A bow-tie crosses itself at its centre; made valid it is two triangles.
	bowtie, err := repo.RepairPolygon(ctx, ring([2]float64{0, 0}, [2]float64{640, 640}, [2]float64{640, 0}, [2]float64{0, 640}, [2]float64{0, 0}))
	require.NoError(t, err)
	assert.False(t, bowtie.WasValid)
	assert.InDelta(t, 204800, bowtie.AreaM2, 1, "two triangles of 640 x 320 / 2")

	_, err = repo.RepairPolygon(ctx, `{"type":"MultiPolygon","coordinates":"broken"}`)
	assert.Error(t, err)
}

func conditionOf(t *testing.T, pool *pgxpool.Pool, code string) (condition, date string) {
	t.Helper()
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COALESCE(condition, ''), COALESCE(to_char(condition_date, 'YYYY-MM-DD'), '') FROM irrigation.asset WHERE code = $1`,
		code).Scan(&condition, &date))
	return
}

func duplicates(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT id FROM irrigation.report WHERE is_duplicate ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	return ids
}

func report(id, on, condition, status string) Report {
	return Report{ID: id, ReportedOn: on, Condition: condition, Status: status, AssetCode: "KW-0001",
		Nomenclature: "B.KW.1", SourceFile: FileReports}
}

func TestLatestConditionAndDuplicateRules(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	asset := &Asset{Code: "KW-0001", Nomenclature: "B.KW.1", Kind: "bangunan_sadap",
		InventoryCondition: "rusak_ringan", InventoryDate: "2026-05-01"}
	asset.SetLocation(geo.ToWGS84(origin).Lon, geo.ToWGS84(origin).Lat)
	other := &Asset{Code: "KW-0002", Nomenclature: "B.KW.2", Kind: "bangunan_sadap",
		InventoryCondition: "baik", InventoryDate: "2026-05-01"}

	replace := func(reports ...Report) {
		require.NoError(t, repo.ReplaceAll(ctx, Snapshot{Assets: []*Asset{asset, other}, Reports: reports}))
	}

	t.Run("no report: the inventory is the latest source", func(t *testing.T) {
		replace()
		condition, date := conditionOf(t, pool, "KW-0001")
		assert.Equal(t, "rusak_ringan", condition)
		assert.Equal(t, "2026-05-01", date)
	})

	t.Run("an older report does not override the inventory", func(t *testing.T) {
		replace(report("LP-1", "2026-04-30", "rusak_berat", "aktif"))
		condition, date := conditionOf(t, pool, "KW-0001")
		assert.Equal(t, "rusak_ringan", condition)
		assert.Equal(t, "2026-05-01", date)
	})

	t.Run("on the same date the report wins", func(t *testing.T) {
		replace(report("LP-1", "2026-05-01", "rusak_berat", "aktif"))
		condition, date := conditionOf(t, pool, "KW-0001")
		assert.Equal(t, "rusak_berat", condition)
		assert.Equal(t, "2026-05-01", date)
	})

	t.Run("the newest active report wins, cancelled ones are ignored", func(t *testing.T) {
		replace(
			report("LP-1", "2026-06-01", "rusak_sedang", "aktif"),
			report("LP-2", "2026-07-01", "rusak_berat", "aktif"),
			report("LP-3", "2026-08-01", "baik", "batal"),
		)
		condition, date := conditionOf(t, pool, "KW-0001")
		assert.Equal(t, "rusak_berat", condition)
		assert.Equal(t, "2026-07-01", date)
		condition, _ = conditionOf(t, pool, "KW-0002")
		assert.Equal(t, "baik", condition, "another asset is not affected")
	})

	t.Run("same asset, date and condition is a duplicate; the smallest id is kept", func(t *testing.T) {
		replace(
			report("LP-3", "2026-06-01", "rusak_sedang", "aktif"),
			report("LP-1", "2026-06-01", "rusak_sedang", "aktif"),
			report("LP-2", "2026-06-01", "rusak_berat", "aktif"),  // different condition: not a duplicate
			report("LP-4", "2026-06-02", "rusak_sedang", "aktif"), // different date: not a duplicate
		)
		assert.Equal(t, []string{"LP-3"}, duplicates(t, pool))

		var count int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM irrigation.finding WHERE category = $1`,
			CategoryDuplicateReport).Scan(&count))
		assert.Equal(t, 1, count)
	})

	t.Run("a report without an asset is recorded as a finding", func(t *testing.T) {
		orphan := report("LP-9", "2026-06-01", "rusak_sedang", "aktif")
		orphan.AssetCode = ""
		replace(orphan)
		var count int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM irrigation.finding WHERE category = $1`,
			CategoryReportNoAsset).Scan(&count))
		assert.Equal(t, 1, count)
	})

	t.Run("cancelling the winning report recomputes the condition without it", func(t *testing.T) {
		replace(
			report("LP-1", "2026-06-01", "rusak_sedang", "aktif"),
			report("LP-2", "2026-07-01", "rusak_berat", "aktif"),
		)
		cancelled := report("LP-2", "2026-07-01", "rusak_berat", "batal")
		cancelled.SourceFile = FileFollowUp
		apply := func() Summary {
			var summary Summary
			require.NoError(t, repo.FollowUp(ctx, func(_ Reference, existing map[string]Report) FollowUpChange {
				var upserts []Report
				summary, upserts = ClassifyFollowUp([]Report{cancelled}, existing)
				return FollowUpChange{SourceFile: FileFollowUp, Upserts: upserts}
			}))
			return summary
		}

		assert.Equal(t, Summary{Cancelled: 1}, apply())
		condition, date := conditionOf(t, pool, "KW-0001")
		assert.Equal(t, "rusak_sedang", condition)
		assert.Equal(t, "2026-06-01", date)

		before := stateHash(t, pool)
		assert.Equal(t, Summary{Unchanged: 1}, apply(), "the second call changes nothing")
		assert.Equal(t, before, stateHash(t, pool))
	})
}

// TestSampleDataEndToEnd imports the real data folder through the service.
// It asserts properties that hold for any data set with the same file names,
// not numbers that belong to the sample.
func TestSampleDataEndToEnd(t *testing.T) {
	dataDir := os.Getenv("TEST_DATA_DIR")
	if dataDir == "" {
		// The data folder sits at the repo root, next to backend/.
		dataDir = filepath.Join("..", "..", "..", "..", "data")
	}
	if _, err := os.Stat(filepath.Join(dataDir, FileAssets)); err != nil {
		t.Skipf("no data folder at %s", dataDir)
	}
	pool := testPool(t)
	ctx := context.Background()
	svc := NewService(NewRepository(pool), dataDir, quietLog())

	svc.Run(ctx)
	require.Equal(t, StatusReady, svc.Status())
	initial := stateHash(t, pool)

	count := func(query string) int {
		var n int
		require.NoError(t, pool.QueryRow(ctx, query).Scan(&n))
		return n
	}
	assert.Zero(t, count(`SELECT count(*) FROM irrigation.asset WHERE geom IS NOT NULL AND 'koordinat_tidak_valid' = ANY(flags)`))
	assert.Zero(t, count(`SELECT count(*) FROM irrigation.asset WHERE geom IS NULL AND NOT 'koordinat_tidak_valid' = ANY(flags)`))
	assert.Zero(t, count(`SELECT count(*) FROM irrigation.asset
		WHERE impacted_area_ha > 0 AND (geom IS NULL OR 'jauh_dari_saluran' = ANY(flags))`),
		"a structure without a position on the network has no impact")
	assert.Zero(t, count(`SELECT count(*) FROM irrigation.plot WHERE geom IS NOT NULL AND NOT ST_IsValid(geom)`))
	assert.Zero(t, count(`SELECT count(*) FROM irrigation.plot p WHERE 'tanpa_sadap' = ANY(flags)
		AND EXISTS (SELECT 1 FROM irrigation.impact i WHERE i.plot_code = p.code)`))
	assert.Zero(t, count(`SELECT count(*) FROM irrigation.asset a WHERE abs(a.impacted_area_ha - COALESCE(
		(SELECT sum(p.area_ha) FROM irrigation.impact i JOIN irrigation.plot p ON p.code = i.plot_code
		 WHERE i.asset_code = a.code), 0)) > 0.006`), "the stored total is the sum of the impacted plots")
	// Lengths computed in Go must agree with PostGIS on EPSG:32749.
	assert.Zero(t, count(`SELECT count(*) FROM irrigation.canal_segment
		WHERE abs(length_m - ST_Length(ST_Transform(geom, 32749))) > 0.01`))
	assert.Zero(t, count(`SELECT count(*) FROM irrigation.asset a WHERE a.geom IS NOT NULL AND abs(a.canal_distance_m -
		(SELECT min(ST_Distance(ST_Transform(a.geom, 32749), ST_Transform(c.geom, 32749))) FROM irrigation.canal_segment c)) > 0.01`))

	if _, err := os.Stat(filepath.Join(dataDir, FileFollowUp)); err != nil {
		return
	}
	first, err := svc.ImportFollowUp(ctx)
	require.NoError(t, err)
	total := first.Added + first.Updated + first.Cancelled + first.Unchanged
	afterFirst := stateHash(t, pool)

	for i := 0; i < 2; i++ {
		again, err := svc.ImportFollowUp(ctx)
		require.NoError(t, err)
		assert.Equal(t, Summary{Unchanged: total}, again, fmt.Sprintf("repeat %d", i+1))
		assert.Equal(t, afterFirst, stateHash(t, pool), "repeating the import leaves the same state")
	}

	svc.Run(ctx)
	assert.Equal(t, initial, stateHash(t, pool), "a restart returns to the state described by the files")
}
