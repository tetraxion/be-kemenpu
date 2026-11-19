package quality

import "context"

// ---- request ----

type FindingQuery struct {
	Category string `form:"kategori"`
}

// ---- response ----

// CategoryCountResponse uses the property names fixed by the API contract.
type CategoryCountResponse struct {
	Category string `json:"kategori" example:"koordinat_utm_dikonversi"`
	Count    int    `json:"jumlah" example:"37"`
}

type SummaryResponse struct {
	Data []CategoryCountResponse `json:"data"`
}

type FindingResponse struct {
	SourceFile string `json:"sumber" example:"aset_bangunan.csv"`
	RowRef     string `json:"pengenal" example:"baris 3 (KW-0003)"`
	Category   string `json:"kategori" example:"koordinat_tertukar_dibetulkan"`
	Problem    string `json:"masalah" example:"Lintang dan bujur tertukar"`
	Action     string `json:"tindakan" example:"Urutan dibetulkan"`
}

type FindingListResponse struct {
	Count int               `json:"jumlah" example:"15"`
	Data  []FindingResponse `json:"data"`
}

func ToSummaryResponse(counts []CategoryCount) SummaryResponse {
	data := make([]CategoryCountResponse, len(counts))
	for i, c := range counts {
		data[i] = CategoryCountResponse{Category: c.Category, Count: c.Count}
	}
	return SummaryResponse{Data: data}
}

func ToFindingListResponse(findings []Finding) FindingListResponse {
	data := make([]FindingResponse, len(findings))
	for i, f := range findings {
		data[i] = FindingResponse{SourceFile: f.SourceFile, RowRef: f.RowRef, Category: f.Category, Problem: f.Problem, Action: f.Action}
	}
	return FindingListResponse{Count: len(data), Data: data}
}

type Service interface {
	Summary(ctx context.Context) ([]CategoryCount, error)
	Findings(ctx context.Context, q FindingQuery) ([]Finding, error)
}
