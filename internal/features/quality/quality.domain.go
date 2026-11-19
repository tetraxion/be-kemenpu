// Package quality serves the data-quality report: how many findings each
// category has, and the findings themselves.
package quality

import "context"

// RequiredCategories must always appear in the summary, even with a count of
// 0. The names and their order come from the API contract.
var RequiredCategories = []string{
	"koordinat_utm_dikonversi",
	"koordinat_tertukar_dibetulkan",
	"koordinat_tidak_valid",
	"jauh_dari_saluran",
	"aset_ganda",
	"ruas_ganda",
	"petak_ganda",
	"geometri_diperbaiki",
	"luas_tidak_wajar",
	"laporan_ganda",
	"laporan_tanpa_aset",
}

// CategoryCount is the number of findings in one category.
type CategoryCount struct {
	Category string
	Count    int
}

// Finding is one cleaning action taken on one source row or feature.
type Finding struct {
	SourceFile string
	RowRef     string
	Category   string
	Problem    string
	Action     string
}

type Repository interface {
	Counts(ctx context.Context) (map[string]int, error)
	// Findings lists findings, limited to one category when it is not empty.
	Findings(ctx context.Context, category string) ([]Finding, error)
}
