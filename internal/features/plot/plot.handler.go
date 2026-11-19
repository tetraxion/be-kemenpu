package plot

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

// Map godoc
//
//	@Summary		Tertiary plots as GeoJSON
//	@Description	Not part of the assignment contract; added for the map. Geometry is WGS84. Properties: `kode_petak`, `nama_petak`, `luas_ha`, `luas_baku_ha`, `luas_geometri_ha`, `nomenklatur_sadap`, `kode_aset_sadap`, `tanda`.
//	@Tags			map
//	@Produce		json
//	@Success		200	{object}	geojson.FeatureCollection
//	@Failure		503	{object}	response.ErrorEnvelope	"The initial import has not finished"
//	@Router			/api/peta/petak [get]
func (h *Handler) Map(c *gin.Context) {
	features, err := h.svc.Features(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, geojson.NewCollection(features))
}
