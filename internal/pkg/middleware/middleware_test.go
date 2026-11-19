package middleware

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func serve(r http.Handler, header http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for k, v := range header {
		req.Header.Set(k, v[0]) // Set canonicalises the key, as a real client's header would be
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestRecoveryTurnsAPanicIntoAnErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID(), Recovery(quietLog()))
	r.GET("/", func(*gin.Context) { panic("boom") })

	rec := serve(r, nil)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "INTERNAL_ERROR", body.Error.Code)
	assert.NotContains(t, body.Error.Message, "boom", "the panic value stays in the log")
	assert.Equal(t, rec.Header().Get(HeaderRequestID), body.Error.RequestID)
}

func TestRequestIDIsReusedOrGenerated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/", func(c *gin.Context) { c.String(http.StatusOK, GetRequestID(c)) })

	given := serve(r, http.Header{HeaderRequestID: {"abc-123"}})
	assert.Equal(t, "abc-123", given.Body.String())
	assert.Equal(t, "abc-123", given.Header().Get(HeaderRequestID))

	generated := serve(r, nil)
	assert.NotEmpty(t, generated.Body.String())
	assert.Equal(t, generated.Body.String(), generated.Header().Get(HeaderRequestID))
}

func TestTimeoutGivesTheRequestADeadline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	deadlineSet := func(limit time.Duration) bool {
		r := gin.New()
		r.Use(Timeout(limit))
		var has bool
		r.GET("/", func(c *gin.Context) { _, has = c.Request.Context().Deadline() })
		serve(r, nil)
		return has
	}

	assert.True(t, deadlineSet(time.Minute))
	assert.False(t, deadlineSet(0), "0 disables the bound instead of expiring every request at once")
}

func TestTimeoutCancelsWorkThatRunsTooLong(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Timeout(20 * time.Millisecond))
	var cause error
	r.GET("/", func(c *gin.Context) {
		select {
		case <-c.Request.Context().Done():
			cause = c.Request.Context().Err()
		case <-time.After(2 * time.Second):
		}
	})

	serve(r, nil)

	assert.ErrorContains(t, cause, "deadline exceeded")
}
