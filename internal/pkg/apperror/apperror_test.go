package apperror

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromKeepsApplicationErrors(t *testing.T) {
	notFound := NotFound("ASSET_NOT_FOUND", "bangunan tidak ditemukan")
	wrapped := fmt.Errorf("asset: get: %w", notFound)

	got := From(wrapped)
	assert.Same(t, notFound, got)
	assert.Equal(t, http.StatusNotFound, HTTPStatus(got.Kind))
}

func TestFromHidesUnexpectedErrors(t *testing.T) {
	got := From(errors.New("connection refused"))
	assert.Equal(t, "INTERNAL_ERROR", got.Code)
	assert.NotContains(t, got.Message, "connection refused", "internal detail must not reach the client")
	assert.Equal(t, http.StatusInternalServerError, HTTPStatus(got.Kind))
}

func TestFromReportsTimeoutAndCancellation(t *testing.T) {
	timeout := From(fmt.Errorf("asset: list: %w", context.DeadlineExceeded))
	assert.Equal(t, "REQUEST_TIMEOUT", timeout.Code)
	assert.Equal(t, http.StatusGatewayTimeout, HTTPStatus(timeout.Kind))

	cancelled := From(fmt.Errorf("asset: list: %w", context.Canceled))
	assert.Equal(t, "REQUEST_CANCELLED", cancelled.Code)
	assert.Equal(t, http.StatusServiceUnavailable, HTTPStatus(cancelled.Kind))
}

func TestWrapKeepsTheCauseOutOfTheMessage(t *testing.T) {
	base := Unprocessable("FOLLOW_UP_FILE_UNREADABLE", "berkas tidak dapat dibaca")
	cause := errors.New("zip: not a valid zip file")

	wrapped := base.Wrap(cause)

	assert.NotSame(t, base, wrapped)
	assert.Nil(t, base.Err, "the package-level error is not modified")
	assert.ErrorIs(t, wrapped, base, "a wrapped copy still matches the original")
	assert.ErrorIs(t, wrapped, cause)
	assert.Equal(t, "berkas tidak dapat dibaca", From(wrapped).Message)
	assert.NotErrorIs(t, wrapped, NotFound("OTHER", "x"))
}

func TestHTTPStatus(t *testing.T) {
	cases := map[Kind]int{
		KindInternal:         http.StatusInternalServerError,
		KindInvalid:          http.StatusBadRequest,
		KindNotFound:         http.StatusNotFound,
		KindMethodNotAllowed: http.StatusMethodNotAllowed,
		KindUnprocessable:    http.StatusUnprocessableEntity,
		KindUnavailable:      http.StatusServiceUnavailable,
		KindTimeout:          http.StatusGatewayTimeout,
	}
	for kind, want := range cases {
		assert.Equal(t, want, HTTPStatus(kind))
	}
}
