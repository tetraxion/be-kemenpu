package canal

import (
	"context"

	"irrigation-server/internal/pkg/geo"
	"irrigation-server/internal/pkg/geojson"
)

// RecapResponse uses the property names fixed by the API contract.
type RecapResponse struct {
	Level     string  `json:"tingkat" enums:"primer,sekunder" example:"sekunder"`
	Condition string  `json:"kondisi" enums:"baik,rusak_ringan,rusak_sedang,rusak_berat" example:"rusak_berat"`
	LengthKm  float64 `json:"panjang_km" example:"3.42"`
}

type RecapListResponse struct {
	Data []RecapResponse `json:"data"`
}

func ToRecapListResponse(recaps []Recap) RecapListResponse {
	data := make([]RecapResponse, len(recaps))
	for i, r := range recaps {
		data[i] = RecapResponse{Level: r.Level, Condition: r.Condition, LengthKm: geo.Round(r.LengthKm, 2)}
	}
	return RecapListResponse{Data: data}
}

type Service interface {
	Recap(ctx context.Context) ([]Recap, error)
	Features(ctx context.Context) ([]geojson.Feature, error)
}
