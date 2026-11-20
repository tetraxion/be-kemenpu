// Package canal serves the canal length recap and map layer.
package canal

import (
	"context"

	"irrigation-server/internal/pkg/geojson"
)

type Recap struct {
	Level     string
	Condition string
	LengthKm  float64
}

type Repository interface {
	Recap(ctx context.Context) ([]Recap, error)
	Features(ctx context.Context) ([]geojson.Feature, error)
}
