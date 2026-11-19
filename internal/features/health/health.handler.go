// Package health reports the import status required by the API contract.
package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// StatusProvider is the source of the import status ("memuat" or "siap").
type StatusProvider interface {
	Status() string
}

type Handler struct {
	provider StatusProvider
}

func NewHandler(provider StatusProvider) *Handler {
	return &Handler{provider: provider}
}

type StatusResponse struct {
	Status string `json:"status" enums:"memuat,siap,gagal" example:"siap"`
}

// Status godoc
//
//	@Summary		Import status
//	@Description	`memuat` while the data folder is being imported, `siap` once it has finished. Always answers 200, because the checker polls it during the import. Every other endpoint answers 503 until the status is `siap`.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	StatusResponse
//	@Router			/api/health [get]
func (h *Handler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, StatusResponse{Status: h.provider.Status()})
}
