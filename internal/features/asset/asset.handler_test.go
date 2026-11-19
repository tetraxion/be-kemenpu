package asset

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubService struct {
	asset      Asset
	reports    []Report
	impact     Impact
	priorities []Priority
	lastQuery  ListQuery
	lastLimit  int
}

func (s *stubService) List(_ context.Context, q ListQuery) ([]Asset, error) {
	s.lastQuery = q
	return []Asset{s.asset}, nil
}

func (s *stubService) Get(_ context.Context, code string) (Asset, []Report, error) {
	if code != s.asset.Code {
		return Asset{}, nil, ErrNotFound
	}
	return s.asset, s.reports, nil
}

func (s *stubService) Impact(_ context.Context, code string) (Impact, error) {
	if code != s.asset.Code {
		return Impact{}, ErrNotFound
	}
	return s.impact, nil
}

func (s *stubService) Priorities(_ context.Context, limit int) ([]Priority, error) {
	s.lastLimit = limit
	return s.priorities, nil
}

func ptr[T any](v T) *T { return &v }

func newTestRouter(svc Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r.Group("/api"), NewHandler(svc))
	return r
}

// get performs a request and decodes the JSON body into generic values, so
// the assertions see exactly the property names a client would.
func get(t *testing.T, r http.Handler, target string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "body: %s", rec.Body.String())
	return rec.Code, body
}

func contractStub() *stubService {
	return &stubService{
		asset: Asset{
			Code: "KW-0012", Nomenclature: ptr("B.KW.3"), Kind: ptr("bangunan_bagi_sadap"),
			CanalName: ptr("Primer Kali Wening"), Condition: ptr("rusak_sedang"), ConditionDate: ptr("2026-08-14"),
			Lon: ptr(110.412351234), Lat: ptr(-7.654121234), Flags: []string{"jauh_dari_saluran"},
		},
		reports: []Report{{ID: "LP-2026-0001", ReportedOn: ptr("2026-08-14"), Condition: ptr("rusak_sedang"), Status: "batal"}},
		impact:  Impact{AssetCode: "KW-0012", TotalAreaHa: 312.45, Plots: []ImpactedPlot{{Code: "KW.3.Ki", AreaHa: 41.204}}},
		priorities: []Priority{{Rank: 1, Score: 1249.8,
			PriorityCandidate: PriorityCandidate{Code: "KW-0012", Condition: "rusak_berat", ImpactedAreaHa: 312.45}}},
	}
}

func TestListFollowsTheContract(t *testing.T) {
	svc := contractStub()
	status, body := get(t, newTestRouter(svc), "/api/aset?jenis=bendung&kondisi=baik&saluran=Primer")

	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, ListQuery{Kind: "bendung", Condition: "baik", CanalName: "Primer"}, svc.lastQuery)
	assert.Equal(t, 1.0, body["jumlah"])
	item := body["data"].([]any)[0].(map[string]any)
	assert.Equal(t, map[string]any{
		"kode_aset": "KW-0012", "nomenklatur": "B.KW.3", "jenis": "bangunan_bagi_sadap",
		"nama_saluran": "Primer Kali Wening", "kondisi": "rusak_sedang", "tgl_kondisi": "2026-08-14",
		"lon": 110.4123512, "lat": -7.6541212, "tanda": []any{"jauh_dari_saluran"},
	}, item)
}

func TestAssetWithoutGeometryUsesNull(t *testing.T) {
	svc := &stubService{asset: Asset{Code: "KW-0039"}}
	_, body := get(t, newTestRouter(svc), "/api/aset/KW-0039")

	for _, key := range []string{"lon", "lat"} {
		value, present := body[key]
		assert.True(t, present, "%s must be present", key)
		assert.Nil(t, value, "%s must be null", key)
	}
	assert.Equal(t, []any{}, body["tanda"], "no flags is [], not null")
	assert.Equal(t, []any{}, body["riwayat"], "no reports is [], not null")
}

func TestGetAddsTheReportHistory(t *testing.T) {
	status, body := get(t, newTestRouter(contractStub()), "/api/aset/KW-0012")

	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "KW-0012", body["kode_aset"])
	history := body["riwayat"].([]any)
	require.Len(t, history, 1)
	report := history[0].(map[string]any)
	assert.Equal(t, "LP-2026-0001", report["id_laporan"])
	assert.Equal(t, "batal", report["status"], "cancelled reports stay in the history")
}

func TestImpactFollowsTheContract(t *testing.T) {
	status, body := get(t, newTestRouter(contractStub()), "/api/aset/KW-0012/dampak")

	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, map[string]any{
		"kode_aset": "KW-0012", "jumlah_petak": 1.0, "luas_terdampak_ha": 312.45,
		"petak": []any{map[string]any{"kode_petak": "KW.3.Ki", "luas_ha": 41.2}},
	}, body)
}

func TestPrioritiesFollowTheContract(t *testing.T) {
	svc := contractStub()
	r := newTestRouter(svc)

	status, body := get(t, r, "/api/prioritas?limit=5")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, 5, svc.lastLimit)
	assert.Equal(t, map[string]any{"data": []any{map[string]any{
		"peringkat": 1.0, "kode_aset": "KW-0012", "kondisi": "rusak_berat", "luas_terdampak_ha": 312.45, "skor": 1249.8,
	}}}, body)

	status, body = get(t, r, "/api/prioritas?limit=abc")
	assert.Equal(t, http.StatusBadRequest, status, "an unreadable limit is rejected, not silently defaulted")
	assert.Equal(t, "ASSET_INVALID_LIMIT", body["error"].(map[string]any)["code"])

	_, _ = get(t, r, "/api/prioritas")
	assert.Equal(t, 0, svc.lastLimit, "no limit is passed as 0 so the service applies the default")
}

func TestUnknownAssetIsNotFound(t *testing.T) {
	r := newTestRouter(contractStub())
	for _, target := range []string{"/api/aset/KW-9999", "/api/aset/KW-9999/dampak"} {
		status, body := get(t, r, target)
		assert.Equal(t, http.StatusNotFound, status, target)
		assert.Equal(t, "ASSET_NOT_FOUND", body["error"].(map[string]any)["code"], target)
	}
}
