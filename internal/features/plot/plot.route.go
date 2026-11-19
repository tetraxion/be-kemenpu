package plot

import "github.com/gin-gonic/gin"

// RegisterRoutes registers the plot endpoints under /api.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/peta/petak", h.Map)
}
