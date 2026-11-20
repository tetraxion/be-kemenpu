package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"irrigation-server/internal/features/ingest"
	"irrigation-server/internal/pkg/config"
)

// stubIngest is an ingest service with a settable status and no database.
type stubIngest struct {
	status   string
	summary  ingest.Summary
	uploaded string
}

func (s *stubIngest) Run(context.Context) {}
func (s *stubIngest) Status() string      { return s.status }
func (s *stubIngest) MarkFailed()         { s.status = ingest.StatusFailed }
func (s *stubIngest) ImportFollowUp(context.Context) (ingest.Summary, error) {
	return s.summary, nil
}
func (s *stubIngest) ImportFollowUpFrom(_ context.Context, workbook io.Reader) (ingest.Summary, error) {
	content, err := io.ReadAll(workbook)
	s.uploaded = string(content)
	return s.summary, err
}

func request(t *testing.T, h http.Handler, method, target string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "body: %s", rec.Body.String())
	return rec.Code, body
}

func TestHealthReportsTheImportStatus(t *testing.T) {
	svc := &stubIngest{status: ingest.StatusLoading}
	// The pool is nil: no route below reaches the database.
	router := newRouter(config.Config{AppEnv: "production"}, slog.Default(), nil, svc)

	status, body := request(t, router, http.MethodGet, "/api/health")
	assert.Equal(t, http.StatusOK, status, "health answers 200 while the import runs")
	assert.Equal(t, map[string]any{"status": "memuat"}, body)

	svc.status = ingest.StatusReady
	_, body = request(t, router, http.MethodGet, "/api/health")
	assert.Equal(t, map[string]any{"status": "siap"}, body)
}

func TestDataEndpointsWaitForTheImport(t *testing.T) {
	svc := &stubIngest{status: ingest.StatusLoading}
	router := newRouter(config.Config{AppEnv: "production"}, slog.Default(), nil, svc)

	for _, target := range []string{"/api/aset", "/api/aset/KW-0001", "/api/aset/KW-0001/dampak", "/api/rekap/saluran",
		"/api/prioritas", "/api/kualitas-data", "/api/peta/saluran", "/api/peta/petak"} {
		status, body := request(t, router, http.MethodGet, target)
		assert.Equal(t, http.StatusServiceUnavailable, status, target)
		assert.Equal(t, "IMPORT_IN_PROGRESS", body["error"].(map[string]any)["code"], target)
	}
	status, _ := request(t, router, http.MethodPost, "/api/impor/susulan")
	assert.Equal(t, http.StatusServiceUnavailable, status)
}

func TestFollowUpImportFollowsTheContract(t *testing.T) {
	svc := &stubIngest{status: ingest.StatusReady, summary: ingest.Summary{Added: 14, Updated: 6, Cancelled: 3, Unchanged: 0}}
	router := newRouter(config.Config{AppEnv: "production"}, slog.Default(), nil, svc)

	status, body := request(t, router, http.MethodPost, "/api/impor/susulan")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, map[string]any{"ditambah": 14.0, "diubah": 6.0, "dibatalkan": 3.0, "tidak_berubah": 0.0}, body)
}

func uploadRequest(t *testing.T, h http.Handler, field string, content []byte) (int, map[string]any) {
	t.Helper()
	var form bytes.Buffer
	w := multipart.NewWriter(&form)
	part, err := w.CreateFormFile(field, "laporan.xlsx")
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/impor/susulan", &form)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "body: %s", rec.Body.String())
	return rec.Code, body
}

func TestFollowUpUploadIsOptional(t *testing.T) {
	svc := &stubIngest{status: ingest.StatusReady, summary: ingest.Summary{Added: 2}}
	router := newRouter(config.Config{AppEnv: "production"}, slog.Default(), nil, svc)

	status, body := uploadRequest(t, router, ingest.UploadField, []byte("workbook bytes"))
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, 2.0, body["ditambah"], "an upload answers in the contract shape")
	assert.Equal(t, "workbook bytes", svc.uploaded, "the uploaded file reaches the service")

	status, body = uploadRequest(t, router, "lampiran", []byte("workbook bytes"))
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "UPLOAD_FILE_MISSING", body["error"].(map[string]any)["code"])

	status, body = uploadRequest(t, router, ingest.UploadField, make([]byte, ingest.MaxUploadBytes+2<<10))
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "UPLOAD_FILE_TOO_LARGE", body["error"].(map[string]any)["code"])
}

func TestUnknownRouteAnswersJSON(t *testing.T) {
	router := newRouter(config.Config{AppEnv: "production"}, slog.Default(), nil, &stubIngest{status: ingest.StatusReady})

	status, body := request(t, router, http.MethodGet, "/api/tidak-ada")
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "ROUTE_NOT_FOUND", body["error"].(map[string]any)["code"])
}

func TestWrongMethodAnswers405(t *testing.T) {
	router := newRouter(config.Config{AppEnv: "production"}, slog.Default(), nil, &stubIngest{status: ingest.StatusReady})

	status, body := request(t, router, http.MethodGet, "/api/impor/susulan")
	assert.Equal(t, http.StatusMethodNotAllowed, status, "a known address with the wrong method is not a 404")
	assert.Equal(t, "METHOD_NOT_ALLOWED", body["error"].(map[string]any)["code"])

	status, _ = request(t, router, http.MethodDelete, "/api/aset")
	assert.Equal(t, http.StatusMethodNotAllowed, status)
}

func TestFailedImportIsReportedAsFailed(t *testing.T) {
	router := newRouter(config.Config{AppEnv: "production"}, slog.Default(), nil, &stubIngest{status: ingest.StatusFailed})

	_, health := request(t, router, http.MethodGet, "/api/health")
	assert.Equal(t, map[string]any{"status": "gagal"}, health)

	status, body := request(t, router, http.MethodGet, "/api/aset")
	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Equal(t, "IMPORT_FAILED", body["error"].(map[string]any)["code"],
		"a client must be able to tell a failed import from one still running")
}

func TestRootAddressPointsToTheAPI(t *testing.T) {
	loading := &stubIngest{status: ingest.StatusLoading}

	status, body := request(t, newRouter(config.Config{AppEnv: "production", DocsEnabled: true}, slog.Default(), nil, loading),
		http.MethodGet, "/")
	assert.Equal(t, http.StatusOK, status, "the index does not wait for the import")
	assert.Equal(t, "/api/health", body["health"])
	assert.Equal(t, "/swagger/index.html", body["docs"].(map[string]any)["swagger"])

	_, body = request(t, newRouter(config.Config{AppEnv: "production"}, slog.Default(), nil, loading), http.MethodGet, "/")
	assert.NotContains(t, body, "docs", "disabled documentation is not advertised")
}
