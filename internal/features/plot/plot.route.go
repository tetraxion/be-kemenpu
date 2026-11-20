package plot

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/peta/petak", h.Map)
}
