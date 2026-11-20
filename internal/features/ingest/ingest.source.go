package ingest

import (
	"os"
	"path/filepath"
)

var sourceFiles = []string{FileCanals, FileAssets, FilePlots, FileReports, FileFollowUp}

// ResolveDataDir picks overrideDir when it holds a source file. The two folders are never mixed.
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
