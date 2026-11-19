package health

import "github.com/gin-gonic/gin"

// RegisterRoutes registers the status probe under /api.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/health", h.Status)
}
