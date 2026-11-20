package ingest

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/impor/susulan", h.ImportFollowUp)
}
