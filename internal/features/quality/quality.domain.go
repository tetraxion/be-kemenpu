// Package quality serves the data-quality report.
package quality

import "context"

// RequiredCategories always appear in the summary, in contract order, even with a count of 0.
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

type CategoryCount struct {
	Category string
	Count    int
}

type Finding struct {
	SourceFile string
	RowRef     string
	Category   string
	Problem    string
	Action     string
}

type Repository interface {
	Counts(ctx context.Context) (map[string]int, error)
	// An empty category means all findings.
	Findings(ctx context.Context, category string) ([]Finding, error)
}
