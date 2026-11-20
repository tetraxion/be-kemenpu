// Package response standardises the JSON responses of the API.
package response

import (
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"

	"irrigation-server/internal/pkg/apperror"
	"irrigation-server/internal/pkg/database"
	"irrigation-server/internal/pkg/middleware"
)

type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code      string `json:"code" example:"ASSET_NOT_FOUND"`
	Message   string `json:"message" example:"bangunan tidak ditemukan"`
	RequestID string `json:"request_id,omitempty" example:"0192b3a4-5c6d-7e8f-9a0b-1c2d3e4f5a6b"`
}

var errDatabaseUnavailable = apperror.Unavailable("DATABASE_UNAVAILABLE", "database sedang tidak dapat dihubungi, coba lagi sebentar")

// JSON writes no envelope: the checker reads the contract properties at the top level.
func JSON(c *gin.Context, status int, body any) {
	c.JSON(status, body)
}

// Error sends internal detail to the log only, with the request id.
func Error(c *gin.Context, err error) {
	var known *apperror.Error
	if !errors.As(err, &known) && database.IsUnavailable(err) {
		err = errDatabaseUnavailable.Wrap(err)
	}
	appErr := apperror.From(err)
	status := apperror.HTTPStatus(appErr.Kind)
	requestID := middleware.GetRequestID(c)

	switch {
	case appErr.Kind == apperror.KindInternal:
		slog.ErrorContext(c.Request.Context(), "request failed",
			"request_id", requestID, "code", appErr.Code, "error", err)
	case appErr.Err != nil:
		// Expected conditions whose cause is still logged.
		slog.WarnContext(c.Request.Context(), "request rejected",
			"request_id", requestID, "code", appErr.Code, "error", appErr.Err)
	}

	c.AbortWithStatusJSON(status, ErrorEnvelope{Error: ErrorBody{
		Code:      appErr.Code,
		Message:   appErr.Message,
		RequestID: requestID,
	}})
}
