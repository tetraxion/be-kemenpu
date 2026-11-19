package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRepo keeps what the service asks to store, without a database.
type fakeRepo struct {
	snapshot    *Snapshot
	replaceErr  error
	reference   Reference
	reports     map[string]Report
	lastChange  FollowUpChange
	repairCalls atomic.Int32
}

func (f *fakeRepo) RepairPolygon(context.Context, string) (PolygonRepair, error) {
	f.repairCalls.Add(1)
	return PolygonRepair{WasValid: true, GeomHex: "00", AreaM2: 409600}, nil
}

func (f *fakeRepo) ReplaceAll(_ context.Context, snap Snapshot) error {
	if f.replaceErr != nil {
		return f.replaceErr
	}
	f.snapshot = &snap
	f.reports = map[string]Report{}
	for _, r := range snap.Reports {
		f.reports[r.ID] = r
	}
	f.reference = NewReference(snap.Assets)
	return nil
}

func (f *fakeRepo) FollowUp(_ context.Context, decide func(Reference, map[string]Report) FollowUpChange) error {
	f.lastChange = decide(f.reference, f.reports)
	for _, r := range f.lastChange.Upserts {
		f.reports[r.ID] = r
	}
	return nil
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// writeDataDir writes a minimal canal, asset and plot file. The report files
// are left out on purpose.
func writeDataDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name string, content []byte) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), content, 0o600))
	}
	collection := func(features ...Feature) []byte {
		type geometry struct {
			Type        string          `json:"type"`
			Coordinates json.RawMessage `json:"coordinates"`
		}
		type feature struct {
			Type       string         `json:"type"`
			Properties map[string]any `json:"properties"`
			Geometry   geometry       `json:"geometry"`
		}
		var out []feature
		for _, f := range features {
			out = append(out, feature{"Feature", f.Properties, geometry{f.GeometryType, f.Coordinates}})
		}
		b, err := json.Marshal(map[string]any{"type": "FeatureCollection", "features": out})
		require.NoError(t, err)
		return b
	}

	write(FileCanals, collection(lineFeature(t, "R1", "baik", [2]float64{0, 0}, [2]float64{2000, 0})))
	weirLon, weirLat := lonLatText(wgs(0, 0))
	offtakeLon, offtakeLat := lonLatText(wgs(1000, 5))
	write(FileAssets, []byte("kode_aset,nomenklatur,jenis,nama_saluran,x,y,kondisi,tahun_bangun,tgl_update\n"+
		fmt.Sprintf("KW-0001,B KW 0,Bendung,Primer Uji,%s,%s,RB,1990,2025-01-10\n", weirLon, weirLat)+
		fmt.Sprintf("KW-0002,bkw1,Bangunan Sadap,Primer Uji,%s,%s,rr,1990,10/01/2025\n", offtakeLon, offtakeLat)))
	write(FilePlots, collection(squareFeature(t, "KW.1.Ki", 41.0, "B.KW.1", 1000, 50, 640, true)))
	return dir
}

func TestRunImportsTheDataFolder(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, writeDataDir(t), quietLog())
	assert.Equal(t, StatusLoading, svc.Status(), "the status is memuat until the import has finished")

	svc.Run(context.Background())

	require.Equal(t, StatusReady, svc.Status())
	require.NotNil(t, repo.snapshot)
	snap := repo.snapshot
	assert.Len(t, snap.Canals, 1)
	require.Len(t, snap.Assets, 2)
	require.Len(t, snap.Plots, 1)
	assert.Empty(t, snap.Reports)
	assert.Equal(t, "KW-0002", snap.Plots[0].OfftakeAssetCode)
	assert.Equal(t, 41.0, snap.Assets[0].ImpactedAreaHa, "the weir affects the plot tapped downstream")
	assert.Equal(t, 41.0, snap.Assets[1].ImpactedAreaHa)

	// The missing report file is recorded, not fatal.
	var unreadable []string
	for _, f := range snap.Findings {
		if f.Category == categoryUnreadableFile {
			unreadable = append(unreadable, f.SourceFile)
		}
	}
	assert.Equal(t, []string{FileReports}, unreadable)
}

func TestRunWithAnEmptyFolderStillBecomesReady(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, t.TempDir(), quietLog())

	svc.Run(context.Background())

	assert.Equal(t, StatusReady, svc.Status())
	require.NotNil(t, repo.snapshot)
	assert.Empty(t, repo.snapshot.Assets)
}

func TestRunReportsAStorageFailure(t *testing.T) {
	svc := NewService(&fakeRepo{replaceErr: assert.AnError}, writeDataDir(t), quietLog())

	svc.Run(context.Background())

	assert.Equal(t, StatusFailed, svc.Status())
}

func TestImportFollowUpPreconditions(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, writeDataDir(t), quietLog())

	_, err := svc.ImportFollowUp(context.Background())
	assert.ErrorIs(t, err, ErrNotReady, "a follow-up import must wait for the initial import")

	svc.Run(context.Background())
	_, err = svc.ImportFollowUp(context.Background())
	assert.ErrorIs(t, err, ErrFollowUpMissing)
}

func TestPolygonRepairsRunConcurrentlyButKeepFileOrder(t *testing.T) {
	const count = 40
	features := make([]Feature, count)
	for i := range features {
		features[i] = squareFeature(t, fmt.Sprintf("P.%02d", i), nil, "", 0, 0, 100, true)
	}

	var running, peak atomic.Int32
	repair := func(geojson string) (PolygonRepair, error) {
		now := running.Add(1)
		for {
			seen := peak.Load()
			if now <= seen || peak.CompareAndSwap(seen, now) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		running.Add(-1)
		return PolygonRepair{WasValid: true, GeomHex: "00", AreaM2: 10000}, nil
	}
	rec := &Recorder{}

	plots := CleanPlots(features, repair, rec)

	require.Len(t, plots, count)
	for i, p := range plots {
		assert.Equal(t, fmt.Sprintf("P.%02d", i), p.Code, "plots keep the order of the file")
	}
	assert.Greater(t, peak.Load(), int32(1), "repairs must overlap")
	assert.LessOrEqual(t, peak.Load(), int32(repairWorkers), "no more than repairWorkers at a time")

	// Findings are recorded in file order too, so repeated imports produce identical output.
	var order []string
	for _, f := range rec.Findings {
		if f.Category == CategoryImplausibleArea {
			order = append(order, f.RowRef)
		}
	}
	require.Len(t, order, count)
	assert.Equal(t, "fitur 1 (P.00)", order[0])
	assert.Equal(t, fmt.Sprintf("fitur %d (P.%02d)", count, count-1), order[count-1])
}

func TestImportFollowUpRejectsAnUnreadableFile(t *testing.T) {
	dir := writeDataDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, FileFollowUp), []byte("this is not a workbook"), 0o600))
	svc := NewService(&fakeRepo{}, dir, quietLog())
	svc.Run(context.Background())

	_, err := svc.ImportFollowUp(context.Background())

	require.ErrorIs(t, err, ErrFollowUpUnreadable)
	assert.NotNil(t, errors.Unwrap(err), "the parser error is kept for the log")
}

// panickyRepo panics where a real repository would talk to the database.
type panickyRepo struct {
	fakeRepo
	onRepair, onReplace bool
}

func (p *panickyRepo) RepairPolygon(ctx context.Context, geojson string) (PolygonRepair, error) {
	if p.onRepair {
		panic("repair exploded")
	}
	return p.fakeRepo.RepairPolygon(ctx, geojson)
}

func (p *panickyRepo) ReplaceAll(ctx context.Context, snap Snapshot) error {
	if p.onReplace {
		panic("replace exploded")
	}
	return p.fakeRepo.ReplaceAll(ctx, snap)
}

func TestAPanicInAPlotWorkerOnlyCostsThatGeometry(t *testing.T) {
	repo := &panickyRepo{onRepair: true}
	svc := NewService(repo, writeDataDir(t), quietLog())

	svc.Run(context.Background())

	require.Equal(t, StatusReady, svc.Status(), "one bad polygon must not stop the import")
	require.Len(t, repo.snapshot.Plots, 1)
	assert.Empty(t, repo.snapshot.Plots[0].GeomHex)
	assert.Equal(t, 41.0, repo.snapshot.Plots[0].AreaHa, "the attribute area is still used")
}

func TestAPanicDuringImportIsReportedAsFailed(t *testing.T) {
	svc := NewService(&panickyRepo{onReplace: true}, writeDataDir(t), quietLog())

	assert.NotPanics(t, func() { svc.Run(context.Background()) })

	assert.Equal(t, StatusFailed, svc.Status())
}
