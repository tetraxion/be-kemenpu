package quality

import (
	"context"
	"sort"
	"strings"
)

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// Summary lists the required categories first in contract order, then extras by name.
func (s *service) Summary(ctx context.Context) ([]CategoryCount, error) {
	counts, err := s.repo.Counts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]CategoryCount, 0, len(RequiredCategories)+len(counts))
	required := map[string]bool{}
	for _, category := range RequiredCategories {
		required[category] = true
		out = append(out, CategoryCount{Category: category, Count: counts[category]})
	}
	var extra []string
	for category := range counts {
		if !required[category] {
			extra = append(extra, category)
		}
	}
	sort.Strings(extra)
	for _, category := range extra {
		out = append(out, CategoryCount{Category: category, Count: counts[category]})
	}
	return out, nil
}

func (s *service) Findings(ctx context.Context, q FindingQuery) ([]Finding, error) {
	return s.repo.Findings(ctx, strings.TrimSpace(q.Category))
}
