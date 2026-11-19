package ingest

import "github.com/gin-gonic/gin"

// RegisterRoutes registers the follow-up import endpoint under /api.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/impor/susulan", h.ImportFollowUp)
}
