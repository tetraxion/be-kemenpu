package response

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"irrigation-server/internal/pkg/apperror"
	"irrigation-server/internal/pkg/middleware"
)

func respond(t *testing.T, err error) (int, ErrorBody) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestID())
	r.GET("/", func(c *gin.Context) { Error(c, err) })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	var envelope ErrorEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope), "body: %s", rec.Body.String())
	assert.Equal(t, rec.Header().Get(middleware.HeaderRequestID), envelope.Error.RequestID,
		"the envelope carries the id the client can quote")
	return rec.Code, envelope.Error
}

func TestErrorMapsApplicationErrors(t *testing.T) {
	status, body := respond(t, fmt.Errorf("asset: get: %w", apperror.NotFound("ASSET_NOT_FOUND", "bangunan tidak ditemukan")))
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "ASSET_NOT_FOUND", body.Code)
	assert.Equal(t, "bangunan tidak ditemukan", body.Message)
}

func TestErrorHidesInternalDetail(t *testing.T) {
	status, body := respond(t, errors.New(`pq: relation "irrigation.asset" does not exist`))
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "INTERNAL_ERROR", body.Code)
	assert.NotContains(t, body.Message, "irrigation.asset")
}

func TestErrorReportsADatabaseOutageAsUnavailable(t *testing.T) {
	outage := fmt.Errorf("asset: list: %w", &net.OpError{Op: "dial", Err: errors.New("connection refused")})
	status, body := respond(t, outage)
	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Equal(t, "DATABASE_UNAVAILABLE", body.Code)
	assert.NotContains(t, body.Message, "connection refused")
}

func TestErrorReportsATimeout(t *testing.T) {
	status, body := respond(t, fmt.Errorf("asset: list: %w", context.DeadlineExceeded))
	assert.Equal(t, http.StatusGatewayTimeout, status)
	assert.Equal(t, "REQUEST_TIMEOUT", body.Code)
}
