package ingest

import (
	"context"
	"io"
)

// ---- response ----

// FollowUpResponse is the contract shape of POST /api/impor/susulan.
type FollowUpResponse struct {
	Added     int `json:"ditambah" example:"14"`
	Updated   int `json:"diubah" example:"6"`
	Cancelled int `json:"dibatalkan" example:"3"`
	Unchanged int `json:"tidak_berubah" example:"0"`
}

func ToFollowUpResponse(s Summary) FollowUpResponse {
	return FollowUpResponse{Added: s.Added, Updated: s.Updated, Cancelled: s.Cancelled, Unchanged: s.Unchanged}
}

type Service interface {
	// Run performs the initial import; the outcome is visible through Status.
	Run(ctx context.Context)
	// ImportFollowUp imports laporan_susulan.xlsx. Calling it again leaves
	// the same final state and reports every row as unchanged.
	ImportFollowUp(ctx context.Context) (Summary, error)
	// ImportFollowUpFrom applies the same import to an uploaded workbook
	// instead of the file in the data folder.
	ImportFollowUpFrom(ctx context.Context, workbook io.Reader) (Summary, error)
	Status() string
	// MarkFailed records that startup could not reach the point of importing
	// (e.g. the database never came up).
	MarkFailed()
}
