package quality

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	counts       map[string]int
	lastCategory string
}

func (f *fakeRepo) Counts(context.Context) (map[string]int, error) { return f.counts, nil }

func (f *fakeRepo) Findings(_ context.Context, category string) ([]Finding, error) {
	f.lastCategory = category
	return nil, nil
}

func TestSummaryAlwaysListsRequiredCategories(t *testing.T) {
	svc := NewService(&fakeRepo{counts: map[string]int{
		"koordinat_utm_dikonversi": 37,
		"tanpa_sadap":              2,
		"angka_koma_desimal":       9,
	}})

	summary, err := svc.Summary(context.Background())
	require.NoError(t, err)
	require.Len(t, summary, len(RequiredCategories)+2)

	for i, category := range RequiredCategories {
		assert.Equal(t, category, summary[i].Category, "required categories keep the contract order")
	}
	assert.Equal(t, 37, summary[0].Count)
	assert.Equal(t, 0, summary[1].Count, "a required category without findings is listed with 0")

	extra := summary[len(RequiredCategories):]
	assert.Equal(t, CategoryCount{Category: "angka_koma_desimal", Count: 9}, extra[0])
	assert.Equal(t, CategoryCount{Category: "tanpa_sadap", Count: 2}, extra[1])
}

func TestFindingsTrimsTheCategoryFilter(t *testing.T) {
	repo := &fakeRepo{}
	_, err := NewService(repo).Findings(context.Background(), FindingQuery{Category: " aset_ganda "})
	require.NoError(t, err)
	assert.Equal(t, "aset_ganda", repo.lastCategory)
}
