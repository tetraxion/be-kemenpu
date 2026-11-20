package ingest

import (
	"context"
	"io"
)

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
	Run(ctx context.Context)
	// ImportFollowUp is idempotent.
	ImportFollowUp(ctx context.Context) (Summary, error)
	ImportFollowUpFrom(ctx context.Context, workbook io.Reader) (Summary, error)
	Status() string
	// MarkFailed is for a startup that never reached the import.
	MarkFailed()
}
