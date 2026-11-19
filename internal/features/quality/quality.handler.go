package quality

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

// Summary godoc
//
//	@Summary		Number of data-quality findings per category
//	@Description	Counted per source row or feature that was repaired, flagged or dropped, across every imported file. The eleven categories required by the assignment are always present, with 0 when nothing was found; extra categories follow.
//	@Tags			quality
//	@Produce		json
//	@Success		200	{object}	SummaryResponse
//	@Failure		503	{object}	response.ErrorEnvelope	"The initial import has not finished"
//	@Router			/api/kualitas-data [get]
func (h *Handler) Summary(c *gin.Context) {
	counts, err := h.svc.Summary(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, ToSummaryResponse(counts))
}

// Findings godoc
//
//	@Summary		List data-quality findings
//	@Description	Not part of the assignment contract; added for the data-quality page. One row per cleaning action: source file, row reference, problem and the action taken.
//	@Tags			quality
//	@Produce		json
//	@Param			kategori	query		string	false	"Finding category, e.g. aset_ganda"
//	@Success		200			{object}	FindingListResponse
//	@Failure		503			{object}	response.ErrorEnvelope	"The initial import has not finished"
//	@Router			/api/kualitas-data/temuan [get]
func (h *Handler) Findings(c *gin.Context) {
	var q FindingQuery
	_ = c.ShouldBindQuery(&q) // plain string: binding cannot fail
	findings, err := h.svc.Findings(c.Request.Context(), q)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, ToFindingListResponse(findings))
}
