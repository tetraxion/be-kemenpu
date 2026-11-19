package canal

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"irrigation-server/internal/pkg/geojson"
	"irrigation-server/internal/pkg/response"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// Recap godoc
//
//	@Summary		Canal length per level and condition
//	@Description	Segment lengths are measured on EPSG:32749 and summed after duplicate segments are removed, in kilometres with two decimals.
//	@Tags			canals
//	@Produce		json
//	@Success		200	{object}	RecapListResponse
//	@Failure		503	{object}	response.ErrorEnvelope	"The initial import has not finished"
//	@Router			/api/rekap/saluran [get]
func (h *Handler) Recap(c *gin.Context) {
	recaps, err := h.svc.Recap(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, ToRecapListResponse(recaps))
}

// Map godoc
//
//	@Summary		Canal segments as GeoJSON
//	@Description	Not part of the assignment contract; added for the map. Geometry is WGS84. Properties: `id_ruas`, `nama_saluran`, `tingkat`, `kondisi`, `panjang_m`.
//	@Tags			map
//	@Produce		json
//	@Success		200	{object}	geojson.FeatureCollection
//	@Failure		503	{object}	response.ErrorEnvelope	"The initial import has not finished"
//	@Router			/api/peta/saluran [get]
func (h *Handler) Map(c *gin.Context) {
	features, err := h.svc.Features(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, geojson.NewCollection(features))
}
