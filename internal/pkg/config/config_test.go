package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	_, err := Load()
	assert.Error(t, err)
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("DATA_DIR", "")
	t.Setenv("DATA_OVERRIDE_DIR", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "not-a-duration")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, "./data", cfg.DataDir)
	assert.Equal(t, "../data", cfg.DataOverrideDir, "data/ at the repo root, seen from backend/")
	assert.Equal(t, 15*time.Second, cfg.ShutdownTimeout, "an invalid duration falls back to the default")
	assert.True(t, cfg.IsProduction())
}
