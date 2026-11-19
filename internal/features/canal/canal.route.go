package canal

import "github.com/gin-gonic/gin"

// RegisterRoutes registers the canal endpoints under /api.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/rekap/saluran", h.Recap)
	rg.GET("/peta/saluran", h.Map)
}
