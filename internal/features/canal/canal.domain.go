// Package canal serves canal segments: the length recap and the map layer.
package canal

import (
	"context"

	"irrigation-server/internal/pkg/geojson"
)

// Recap is the total canal length of one level and condition.
type Recap struct {
	Level     string
	Condition string
	LengthKm  float64
}

type Repository interface {
	// Recap sums segment lengths per level and condition. Duplicate segments
	// were already removed at import time.
	Recap(ctx context.Context) ([]Recap, error)
	Features(ctx context.Context) ([]geojson.Feature, error)
}
