package asset

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	assets     map[string]Asset
	plots      map[string][]ImpactedPlot
	candidates []PriorityCandidate
	lastFilter ListFilter
}

func (f *fakeRepo) List(_ context.Context, filter ListFilter) ([]Asset, error) {
	f.lastFilter = filter
	return nil, nil
}

func (f *fakeRepo) Get(_ context.Context, code string) (Asset, error) {
	a, ok := f.assets[code]
	if !ok {
		return Asset{}, ErrNotFound
	}
	return a, nil
}

func (f *fakeRepo) Reports(context.Context, string) ([]Report, error) { return nil, nil }

func (f *fakeRepo) ImpactedPlots(_ context.Context, code string) ([]ImpactedPlot, error) {
	return f.plots[code], nil
}

func (f *fakeRepo) PriorityCandidates(context.Context) ([]PriorityCandidate, error) {
	return f.candidates, nil
}

func TestListNormalizesFilters(t *testing.T) {
	repo := &fakeRepo{}
	_, err := NewService(repo).List(context.Background(), ListQuery{Kind: "Bangunan Bagi-Sadap", Condition: " RR ", CanalName: "  Primer  Kali Wening "})
	require.NoError(t, err)
	assert.Equal(t, ListFilter{Kind: "bangunan_bagi_sadap", Condition: "rusak_ringan", CanalName: "Primer Kali Wening"}, repo.lastFilter)
}

func TestImpactSumsUnroundedAreas(t *testing.T) {
	repo := &fakeRepo{
		assets: map[string]Asset{"KW-0003": {Code: "KW-0003"}},
		plots:  map[string][]ImpactedPlot{"KW-0003": {{"KW.2.Ka", 40.004}, {"KW.3.Ki", 50.004}, {"SR.1.Ki", 25.004}}},
	}
	svc := NewService(repo)

	impact, err := svc.Impact(context.Background(), " kw-0003 ")
	require.NoError(t, err)
	assert.Equal(t, "KW-0003", impact.AssetCode)
	assert.Equal(t, 115.01, impact.TotalAreaHa, "rounding each plot first would give 115.00")
	assert.Len(t, impact.Plots, 3)

	_, err = svc.Impact(context.Background(), "KW-9999")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPrioritiesFollowTheScoringRule(t *testing.T) {
	repo := &fakeRepo{candidates: []PriorityCandidate{
		{Code: "KW-0005", Condition: "rusak_ringan", ImpactedAreaHa: 600},    // 600
		{Code: "KW-0002", Condition: "rusak_berat", ImpactedAreaHa: 150},     // 600: tie, ordered by code
		{Code: "KW-0009", Condition: "baik", ImpactedAreaHa: 900},            // 0: excluded
		{Code: "KW-0001", Condition: "rusak_sedang", ImpactedAreaHa: 312.45}, // 624.9
		{Code: "KW-0007", Condition: "rusak_berat", ImpactedAreaHa: 10},      // 40
	}}
	svc := NewService(repo)

	all, err := svc.Priorities(context.Background(), 0)
	require.NoError(t, err)
	require.Len(t, all, 4, "a good structure scores 0 and is not listed")

	codes := make([]string, len(all))
	for i, p := range all {
		codes[i] = p.Code
		assert.Equal(t, i+1, p.Rank)
	}
	assert.Equal(t, []string{"KW-0001", "KW-0002", "KW-0005", "KW-0007"}, codes)
	assert.Equal(t, 624.9, all[0].Score)
	assert.Equal(t, 600.0, all[1].Score)

	limited, err := svc.Priorities(context.Background(), 2)
	require.NoError(t, err)
	assert.Len(t, limited, 2)
}

func TestListRejectsUnknownFilterValues(t *testing.T) {
	svc := NewService(&fakeRepo{})

	_, err := svc.List(context.Background(), ListQuery{Kind: "menara"})
	assert.ErrorIs(t, err, ErrInvalidKind)

	_, err = svc.List(context.Background(), ListQuery{Condition: "hancur"})
	assert.ErrorIs(t, err, ErrInvalidCondition)

	_, err = svc.List(context.Background(), ListQuery{CanalName: "Saluran Yang Tidak Ada"})
	assert.NoError(t, err, "an unknown canal name is a valid filter that matches nothing")
}

func TestPrioritiesValidatesTheLimit(t *testing.T) {
	svc := NewService(&fakeRepo{})

	_, err := svc.Priorities(context.Background(), -1)
	assert.ErrorIs(t, err, ErrInvalidLimit)

	// A very large limit is how a client asks for every row.
	for _, limit := range []int{0, 1, 1000000} {
		_, err := svc.Priorities(context.Background(), limit)
		assert.NoError(t, err, "limit %d", limit)
	}
}
