package ingest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveDataDir(t *testing.T) {
	bundled := t.TempDir()

	t.Run("no override folder configured", func(t *testing.T) {
		assert.Equal(t, bundled, ResolveDataDir(bundled, ""))
	})

	t.Run("override folder does not exist", func(t *testing.T) {
		assert.Equal(t, bundled, ResolveDataDir(bundled, filepath.Join(t.TempDir(), "missing")))
	})

	t.Run("override folder is empty", func(t *testing.T) {
		assert.Equal(t, bundled, ResolveDataDir(bundled, t.TempDir()))
	})

	t.Run("override folder holds only unrelated files", func(t *testing.T) {
		override := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(override, "catatan.txt"), []byte("x"), 0o600))
		assert.Equal(t, bundled, ResolveDataDir(bundled, override))
	})

	t.Run("one source file is enough for the override to win", func(t *testing.T) {
		override := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(override, FileAssets), []byte("kode_aset\n"), 0o600))
		assert.Equal(t, override, ResolveDataDir(bundled, override))
	})

	t.Run("a folder named like a source file does not count", func(t *testing.T) {
		override := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(override, FileAssets), 0o700))
		assert.Equal(t, bundled, ResolveDataDir(bundled, override))
	})
}
