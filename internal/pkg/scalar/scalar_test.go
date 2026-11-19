package scalar

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterServesThePageWithTheSpecURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r, "/scalar", "/swagger/doc.json", "Irrigation API Reference")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/scalar", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	body := rec.Body.String()
	assert.Contains(t, body, "@scalar/api-reference@"+Version)
	assert.Contains(t, body, `url: "/swagger/doc.json"`, "the spec URL is rendered as a JavaScript string")
	assert.Contains(t, body, "<title>Irrigation API Reference</title>")
}
