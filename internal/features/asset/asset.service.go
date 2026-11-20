package asset

import (
	"context"
	"sort"
	"strings"

	"irrigation-server/internal/pkg/geo"
	"irrigation-server/internal/pkg/normalize"
)

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func normalizeCode(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }

func (s *service) List(ctx context.Context, q ListQuery) ([]Asset, error) {
	// Filters accept the source spelling variants: "RR" selects rusak_ringan.
	f := ListFilter{Kind: strings.TrimSpace(q.Kind), Condition: strings.TrimSpace(q.Condition), CanalName: normalize.Clean(q.CanalName)}
	// An unknown value is an error, not an empty list.
	if f.Kind != "" {
		kind, ok := normalize.AssetKind(f.Kind)
		if !ok {
			return nil, ErrInvalidKind
		}
		f.Kind = kind
	}
	if f.Condition != "" {
		condition, ok := normalize.Condition(f.Condition)
		if !ok {
			return nil, ErrInvalidCondition
		}
		f.Condition = condition
	}
	return s.repo.List(ctx, f)
}

func (s *service) Get(ctx context.Context, code string) (Asset, []Report, error) {
	a, err := s.repo.Get(ctx, normalizeCode(code))
	if err != nil {
		return Asset{}, nil, err
	}
	reports, err := s.repo.Reports(ctx, a.Code)
	if err != nil {
		return Asset{}, nil, err
	}
	return a, reports, nil
}

func (s *service) Impact(ctx context.Context, code string) (Impact, error) {
	a, err := s.repo.Get(ctx, normalizeCode(code))
	if err != nil {
		return Impact{}, err
	}
	plots, err := s.repo.ImpactedPlots(ctx, a.Code)
	if err != nil {
		return Impact{}, err
	}
	var total float64
	for _, p := range plots {
		total += p.AreaHa
	}
	// The total is rounded once, from unrounded plot areas.
	return Impact{AssetCode: a.Code, TotalAreaHa: geo.Round(total, 2), Plots: plots}, nil
}

// Priorities: score = condition weight × impacted area, above 0 only, ties by asset code.
func (s *service) Priorities(ctx context.Context, limit int) ([]Priority, error) {
	// 0 means the caller gave no limit.
	if limit == 0 {
		limit = DefaultPriorityLimit
	}
	if limit < 0 {
		return nil, ErrInvalidLimit
	}
	candidates, err := s.repo.PriorityCandidates(ctx)
	if err != nil {
		return nil, err
	}
	priorities := make([]Priority, 0, len(candidates))
	for _, c := range candidates {
		score := geo.Round(normalize.ConditionWeight(c.Condition)*c.ImpactedAreaHa, 2)
		if score > 0 {
			priorities = append(priorities, Priority{PriorityCandidate: c, Score: score})
		}
	}
	sort.Slice(priorities, func(i, j int) bool {
		if priorities[i].Score != priorities[j].Score {
			return priorities[i].Score > priorities[j].Score
		}
		return priorities[i].Code < priorities[j].Code
	})
	if len(priorities) > limit {
		priorities = priorities[:limit]
	}
	for i := range priorities {
		priorities[i].Rank = i + 1
	}
	return priorities, nil
}
