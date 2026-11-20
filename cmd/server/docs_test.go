package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"irrigation-server/internal/features/ingest"
	"irrigation-server/internal/pkg/config"
	"irrigation-server/internal/pkg/swagger"
)

func docsRouter(enabled bool) http.Handler {
	return newRouter(config.Config{AppEnv: "production", DocsEnabled: enabled}, slog.Default(), nil,
		&stubIngest{status: ingest.StatusLoading})
}

func fetch(h http.Handler, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestDocumentationIsServedWhileTheImportRuns(t *testing.T) {
	router := docsRouter(true)

	spec := fetch(router, swagger.SpecPath)
	require.Equal(t, http.StatusOK, spec.Code, "the documentation does not wait for the import")
	assert.Contains(t, spec.Body.String(), "Irrigation Network Condition API")

	assert.Equal(t, http.StatusOK, fetch(router, "/swagger/index.html").Code)
	page := fetch(router, "/scalar")
	assert.Equal(t, http.StatusOK, page.Code)
	assert.Contains(t, page.Body.String(), swagger.SpecPath)
}

func TestDocumentationCanBeDisabled(t *testing.T) {
	router := docsRouter(false)

	for _, target := range []string{swagger.SpecPath, "/swagger/index.html", "/scalar"} {
		assert.Equal(t, http.StatusNotFound, fetch(router, target).Code, target)
	}
}

func TestEveryRouteIsDocumented(t *testing.T) {
	engine := newRouter(config.Config{AppEnv: "production", DocsEnabled: true}, slog.Default(), nil,
		&stubIngest{status: ingest.StatusReady})

	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	require.NoError(t, json.Unmarshal(fetch(engine, swagger.SpecPath).Body.Bytes(), &spec))
	documented := map[string]bool{}
	for path, methods := range spec.Paths {
		for method := range methods {
			documented[strings.ToUpper(method)+" "+path] = true
		}
	}

	registered := map[string]bool{}
	for _, route := range engine.Routes() {
		if !strings.HasPrefix(route.Path, "/api/") {
			continue // the documentation pages themselves
		}
		// Gin writes :code, the specification uses the contract names.
		registered[route.Method+" "+strings.ReplaceAll(route.Path, ":code", "{kode_aset}")] = true
	}

	require.NotEmpty(t, registered)
	assert.Equal(t, registered, documented)
}
