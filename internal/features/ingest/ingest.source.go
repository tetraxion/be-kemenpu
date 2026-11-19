package ingest

import (
	"os"
	"path/filepath"
)

// sourceFiles are the files the import knows by name.
var sourceFiles = []string{FileCanals, FileAssets, FilePlots, FileReports, FileFollowUp}

// ResolveDataDir picks the folder to import from.
//
// The source files ship inside the service (dataDir). The assignment's
// checker, however, puts its own data set in data/ at the repo root before
// starting the application. That folder (overrideDir) therefore wins as soon
// as it holds at least one source file; otherwise the bundled folder is used.
// The choice is all-or-nothing: files from the two folders are never mixed.
func ResolveDataDir(dataDir, overrideDir string) string {
	if overrideDir == "" {
		return dataDir
	}
	for _, name := range sourceFiles {
		if info, err := os.Stat(filepath.Join(overrideDir, name)); err == nil && !info.IsDir() {
			return overrideDir
		}
	}
	return dataDir
}
