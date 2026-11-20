package ingest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"sync/atomic"
)

type service struct {
	repo    Repository
	dataDir string
	log     *slog.Logger
	// mu keeps the initial import and follow-up imports from overlapping.
	mu     sync.Mutex
	status atomic.Value
}

func NewService(repo Repository, dataDir string, log *slog.Logger) Service {
	s := &service{repo: repo, dataDir: dataDir, log: log}
	s.status.Store(StatusLoading)
	return s
}

func (s *service) Status() string { return s.status.Load().(string) }

func (s *service) MarkFailed() { s.status.Store(StatusFailed) }

// Run performs the initial import and moves the status to ready or failed.
func (s *service) Run(ctx context.Context) {
	// The import runs on its own goroutine, where an unrecovered panic would
	// take the whole server down. Report it as a failed import instead.
	defer func() {
		if rec := recover(); rec != nil {
			s.log.ErrorContext(ctx, "initial import panicked", "panic", rec, "stack", string(debug.Stack()))
			s.status.Store(StatusFailed)
		}
	}()
	if err := s.importInitial(ctx); err != nil {
		s.log.ErrorContext(ctx, "initial import failed", "error", err)
		s.status.Store(StatusFailed)
		return
	}
	s.status.Store(StatusReady)
	s.log.InfoContext(ctx, "initial import finished")
}

// sources holds the raw content of the four files imported at startup, each
// with its own read error so one unreadable file does not stop the others.
type sources struct {
	canals, plots                              []Feature
	assets, reports                            []Row
	canalsErr, plotsErr, assetsErr, reportsErr error
}

// readSources reads and parses the four files concurrently. They are
// independent of each other; only the cleaning that follows has an order.
func (s *service) readSources() sources {
	var src sources
	path := func(name string) string { return filepath.Join(s.dataDir, name) }
	// A parser that panics on a malformed file must not crash the server
	// from a worker goroutine; it becomes that file's read error.
	guard := func(err *error, read func()) func() {
		return func() {
			defer func() {
				if rec := recover(); rec != nil {
					*err = fmt.Errorf("ingest: reader panicked: %v", rec)
				}
			}()
			read()
		}
	}
	var wg sync.WaitGroup
	wg.Go(guard(&src.canalsErr, func() { src.canals, src.canalsErr = ReadGeoJSON(path(FileCanals)) }))
	wg.Go(guard(&src.assetsErr, func() { src.assets, src.assetsErr = ReadCSV(path(FileAssets)) }))
	wg.Go(guard(&src.plotsErr, func() { src.plots, src.plotsErr = ReadGeoJSON(path(FilePlots)) }))
	wg.Go(guard(&src.reportsErr, func() { src.reports, src.reportsErr = ReadXLSX(path(FileReports)) }))
	wg.Wait()
	return src
}

func (s *service) importInitial(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Every start imports the data folder from scratch, so the application
	// always begins from the state described by the files, whatever an earlier
	// run (including a follow-up import) left in the database.
	src := s.readSources()

	rec := &Recorder{}
	// A file that cannot be read is reported and skipped; the rest is still imported.
	for _, file := range []struct {
		name string
		err  error
	}{{FileCanals, src.canalsErr}, {FileAssets, src.assetsErr}, {FilePlots, src.plotsErr}, {FileReports, src.reportsErr}} {
		if file.err != nil {
			s.log.WarnContext(ctx, "source file skipped", "file", file.name, "error", file.err)
			rec.Add(file.name, "-", categoryUnreadableFile, "Berkas tidak ada atau tidak terbaca", "Berkas dilewati")
		}
	}

	// Cleaning runs in a fixed order because each step needs the one before:
	// assets are flagged by their distance to the cleaned canals, the analysis
	// needs canals, assets and plots, and reports are matched to the assets.
	var snap Snapshot
	snap.Canals = CleanCanals(src.canals, rec)
	snap.Assets = CleanAssets(src.assets, snap.Canals, rec)
	snap.Plots = CleanPlots(src.plots, func(geojson string) (PolygonRepair, error) {
		return s.repo.RepairPolygon(ctx, geojson)
	}, rec)
	Analyze(snap.Canals, snap.Assets, snap.Plots, rec)
	snap.Reports = CleanReports(src.reports, FileReports, NewReference(snap.Assets), rec)
	snap.Findings = rec.Findings

	if err := s.repo.ReplaceAll(ctx, snap); err != nil {
		return err
	}
	s.log.InfoContext(ctx, "data imported", "canal_segments", len(snap.Canals), "assets", len(snap.Assets),
		"plots", len(snap.Plots), "reports", len(snap.Reports), "findings", len(snap.Findings))
	return nil
}

func (s *service) ImportFollowUp(ctx context.Context) (Summary, error) {
	if s.Status() != StatusReady {
		return Summary{}, ErrNotReady
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dataDir, FileFollowUp)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Summary{}, ErrFollowUpMissing
		}
		// The file is there but cannot be opened (e.g. permissions).
		return Summary{}, ErrFollowUpUnreadable.Wrap(err)
	}
	rows, err := ReadXLSX(path)
	if err != nil {
		return Summary{}, ErrFollowUpUnreadable.Wrap(err)
	}
	return s.applyFollowUp(ctx, rows)
}

func (s *service) ImportFollowUpFrom(ctx context.Context, workbook io.Reader) (Summary, error) {
	if s.Status() != StatusReady {
		return Summary{}, ErrNotReady
	}
	// Parse before taking the lock: a slow upload must not hold up other imports.
	rows, err := ReadXLSXFrom(workbook, "uploaded workbook")
	if err != nil {
		return Summary{}, ErrUploadUnreadable.Wrap(err)
	}
	// Any workbook opens fine, so check that this one is a report file before
	// its rows are recorded as findings.
	if len(rows) > 0 && !hasColumn(rows, "id_laporan") {
		return Summary{}, ErrUploadNotReports
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.applyFollowUp(ctx, rows)
}

func hasColumn(rows []Row, name string) bool {
	for _, row := range rows {
		if _, ok := row[name]; ok {
			return true
		}
	}
	return false
}

// applyFollowUp cleans and stores follow-up rows. The caller holds s.mu. An
// uploaded file is recorded under the same source name as the one in the data
// folder: both are the follow-up file, so repeating either stays idempotent.
func (s *service) applyFollowUp(ctx context.Context, rows []Row) (Summary, error) {
	var summary Summary
	err := s.repo.FollowUp(ctx, func(ref Reference, existing map[string]Report) FollowUpChange {
		rec := &Recorder{}
		var upserts []Report
		summary, upserts = ClassifyFollowUp(CleanReports(rows, FileFollowUp, ref, rec), existing)
		return FollowUpChange{SourceFile: FileFollowUp, Upserts: upserts, Findings: rec.Findings}
	})
	if err != nil {
		return Summary{}, err
	}
	s.log.InfoContext(ctx, "follow-up imported", "added", summary.Added, "updated", summary.Updated,
		"cancelled", summary.Cancelled, "unchanged", summary.Unchanged)
	return summary, nil
}
