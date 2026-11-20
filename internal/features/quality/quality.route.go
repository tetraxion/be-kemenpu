package quality

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	g := rg.Group("/kualitas-data")
	g.GET("", h.Summary)
	g.GET("/temuan", h.Findings)
}
