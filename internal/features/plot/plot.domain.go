// Package plot serves tertiary plots as a map layer.
package plot

import (
	"context"

	"irrigation-server/internal/pkg/geojson"
)

type Repository interface {
	Features(ctx context.Context) ([]geojson.Feature, error)
}

type Service interface {
	Features(ctx context.Context) ([]geojson.Feature, error)
}
