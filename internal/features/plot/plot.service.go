package plot

import (
	"context"

	"irrigation-server/internal/pkg/geojson"
)

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Features(ctx context.Context) ([]geojson.Feature, error) {
	return s.repo.Features(ctx)
}
