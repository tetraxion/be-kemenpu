package ingest

import (
	"errors"
	"net/http"
	"strings"

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
//	@Description
//	@Description	Optional: send the request as `multipart/form-data` with an `.xlsx` in the field `berkas` to import that file instead. The same cleaning rules apply and the response has the same shape. A request without a body always reads the data folder.
//	@Tags			import
//	@Accept			mpfd
//	@Produce		json
//	@Param			berkas	formData	file	false	"Follow-up workbook (.xlsx, at most 10 MB). Leave out to read the data folder."
//	@Success		200		{object}	FollowUpResponse
//	@Failure		400		{object}	response.ErrorEnvelope	"The upload has no file in `berkas`, or the file is larger than 10 MB"
//	@Failure		404		{object}	response.ErrorEnvelope	"The follow-up file is not in the data folder"
//	@Failure		422		{object}	response.ErrorEnvelope	"The file is not a readable workbook, or has no `id_laporan` column"
//	@Failure		503		{object}	response.ErrorEnvelope	"The initial import has not finished"
//	@Router			/api/impor/susulan [post]
func (h *Handler) ImportFollowUp(c *gin.Context) {
	var (
		summary Summary
		err     error
	)
	// Only a multipart request is an upload. Anything else reads the data folder.
	if strings.HasPrefix(c.ContentType(), "multipart/") {
		summary, err = h.importUpload(c)
	} else {
		summary, err = h.svc.ImportFollowUp(c.Request.Context())
	}
	if err != nil {
		response.Error(c, err)
		return
	}
	response.JSON(c, http.StatusOK, ToFollowUpResponse(summary))
}

func (h *Handler) importUpload(c *gin.Context) (Summary, error) {
	// The limit covers the whole body, form overhead included.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxUploadBytes+1<<10)
	header, err := c.FormFile(UploadField)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return Summary{}, ErrUploadTooLarge
		}
		return Summary{}, ErrUploadMissing.Wrap(err)
	}
	if header.Size > MaxUploadBytes {
		return Summary{}, ErrUploadTooLarge
	}
	file, err := header.Open()
	if err != nil {
		return Summary{}, ErrUploadUnreadable.Wrap(err)
	}
	defer file.Close()
	return h.svc.ImportFollowUpFrom(c.Request.Context(), file)
}
