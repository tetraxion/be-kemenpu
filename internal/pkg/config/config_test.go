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
	// make test exports the developer's .env, so clear the variables first.
	for _, key := range []string{"APP_ENV", "HTTP_ADDR", "DOCS_ENABLED", "REQUEST_TIMEOUT", "DATABASE_WAIT", "DATA_DIR", "DATA_OVERRIDE_DIR"} {
		t.Setenv(key, "")
	}
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db")
	t.Setenv("SHUTDOWN_TIMEOUT", "not-a-duration")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, "./data", cfg.DataDir)
	assert.Equal(t, "../data", cfg.DataOverrideDir, "data/ at the repo root, seen from backend/")
	assert.Equal(t, 15*time.Second, cfg.ShutdownTimeout, "an invalid duration falls back to the default")
	assert.True(t, cfg.IsProduction())
}
