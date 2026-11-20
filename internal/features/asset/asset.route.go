package asset

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	g := rg.Group("/aset")
	g.GET("", h.List)
	g.GET("/:code", h.Get)
	g.GET("/:code/dampak", h.Impact)
	rg.GET("/prioritas", h.Priorities)
}
