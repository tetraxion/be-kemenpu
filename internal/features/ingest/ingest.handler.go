package ingest

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

// ImportFollowUp godoc
//
//	@Summary		Import the follow-up reports
//	@Description	Imports `laporan_susulan.xlsx` from the data folder: new reports are added, reports with a known `id_laporan` replace the stored one, and reports with status `batal` are cancelled. The latest condition and the priorities are recomputed. Safe to repeat: a second call reports every row as `tidak_berubah` and leaves the same state.
//	@Tags			import
//	@Produce		json
//	@Success		200	{object}	FollowUpResponse
//	@Failure		404	{object}	response.ErrorEnvelope	"The follow-up file is not in the data folder"
//	@Failure		422	{object}	response.ErrorEnvelope	"The follow-up file is not a readable workbook"
//	@Failure		503	{object}	response.ErrorEnvelope	"The initial import has not finished"
//	@Router			/api/impor/susulan [post]
func (h *Handler) ImportFollowUp(c *gin.Context) {
	summary, err := h.svc.ImportFollowUp(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, ToFollowUpResponse(summary))
}
