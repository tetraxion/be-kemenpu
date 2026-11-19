package asset

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"irrigation-server/internal/pkg/response"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// List godoc
//
//	@Summary		List irrigation structures
//	@Description	Every structure with its latest condition. Filters accept the same spelling variants as the source files, so `RR` selects `rusak_ringan`. A structure without geometry has `lon` and `lat` set to null.
//	@Tags			assets
//	@Produce		json
//	@Param			jenis	query		string	false	"Structure type"	Enums(bendung,bangunan_bagi,bangunan_sadap,bangunan_bagi_sadap,bangunan_pelengkap)
//	@Param			kondisi	query		string	false	"Latest condition"	Enums(baik,rusak_ringan,rusak_sedang,rusak_berat)
//	@Param			saluran	query		string	false	"Canal name, case-insensitive"
//	@Success		200		{object}	ListResponse
//	@Failure		400		{object}	response.ErrorEnvelope	"Unknown jenis or kondisi"
//	@Failure		503		{object}	response.ErrorEnvelope	"The initial import has not finished"
//	@Router			/api/aset [get]
func (h *Handler) List(c *gin.Context) {
	var q ListQuery
	_ = c.ShouldBindQuery(&q) // plain strings: binding cannot fail
	assets, err := h.svc.List(c.Request.Context(), q)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, ToListResponse(assets))
}

// Get godoc
//
//	@Summary		Get one structure with its report history
//	@Description	`riwayat` holds every report matched to the structure, newest first, cancelled ones included. Reports dropped as duplicates are left out.
//	@Tags			assets
//	@Produce		json
//	@Param			kode_aset	path		string	true	"Asset code, e.g. KW-0012"
//	@Success		200			{object}	DetailResponse
//	@Failure		404			{object}	response.ErrorEnvelope
//	@Failure		503			{object}	response.ErrorEnvelope	"The initial import has not finished"
//	@Router			/api/aset/{kode_aset} [get]
func (h *Handler) Get(c *gin.Context) {
	a, reports, err := h.svc.Get(c.Request.Context(), c.Param("code"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, ToDetailResponse(a, reports))
}

// Impact godoc
//
//	@Summary		Downstream impact of one structure
//	@Description	The tertiary plots that lose water when the structure fails: every plot whose water path from the weir passes through it, including plots tapped at the structure itself. A structure without geometry or flagged `jauh_dari_saluran` has an impacted area of 0.
//	@Tags			assets
//	@Produce		json
//	@Param			kode_aset	path		string	true	"Asset code, e.g. KW-0012"
//	@Success		200			{object}	ImpactResponse
//	@Failure		404			{object}	response.ErrorEnvelope
//	@Failure		503			{object}	response.ErrorEnvelope	"The initial import has not finished"
//	@Router			/api/aset/{kode_aset}/dampak [get]
func (h *Handler) Impact(c *gin.Context) {
	impact, err := h.svc.Impact(c.Request.Context(), c.Param("code"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, ToImpactResponse(impact))
}

// Priorities godoc
//
//	@Summary		Maintenance priority list
//	@Description	Score = condition weight (baik 0, rusak_ringan 1, rusak_sedang 2, rusak_berat 4) x impacted area. Only scores above 0 are listed, highest first; ties are ordered by `kode_aset`.
//	@Tags			assets
//	@Produce		json
//	@Param			limit	query		int	false	"Maximum number of rows"	default(20)	minimum(1)
//	@Success		200		{object}	PriorityListResponse
//	@Failure		400		{object}	response.ErrorEnvelope	"limit is not a positive integer"
//	@Failure		503		{object}	response.ErrorEnvelope	"The initial import has not finished"
//	@Router			/api/prioritas [get]
func (h *Handler) Priorities(c *gin.Context) {
	var q PriorityQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, ErrInvalidLimit.Wrap(err))
		return
	}
	priorities, err := h.svc.Priorities(c.Request.Context(), q.Limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, ToPriorityListResponse(priorities))
}
