// Package asset serves irrigation structures: the list, one structure with
// its report history, its downstream impact, and the maintenance priority.
package asset

import (
	"context"

	"irrigation-server/internal/pkg/apperror"
)

// DefaultPriorityLimit is the number of priority rows returned when the
// caller gives no limit.
const DefaultPriorityLimit = 20

// Asset is one irrigation structure with its latest condition.
type Asset struct {
	Code          string
	Nomenclature  *string
	Kind          *string
	CanalName     *string
	Condition     *string
	ConditionDate *string // YYYY-MM-DD
	Lon, Lat      *float64
	Flags         []string
}

// Report is one field report matched to a structure.
type Report struct {
	ID           string
	ReportedOn   *string
	Officer      *string
	Nomenclature *string
	DamageType   *string
	Condition    *string
	Lat, Lon     *float64
	Status       string
	Note         *string
}

// ImpactedPlot is a tertiary plot that loses water when the structure fails.
type ImpactedPlot struct {
	Code   string
	AreaHa float64
}

// Impact is the downstream impact of one structure.
type Impact struct {
	AssetCode   string
	TotalAreaHa float64
	Plots       []ImpactedPlot
}

// PriorityCandidate is a structure that affects at least one plot.
type PriorityCandidate struct {
	Code           string
	Condition      string
	ImpactedAreaHa float64
}

// Priority is one row of the maintenance priority list.
type Priority struct {
	Rank int
	PriorityCandidate
	Score float64
}

type ListFilter struct {
	Kind      string
	Condition string
	CanalName string
}

type Repository interface {
	List(ctx context.Context, f ListFilter) ([]Asset, error)
	Get(ctx context.Context, code string) (Asset, error)
	// Reports returns every report matched to the structure, cancelled ones
	// included, newest first. Reports dropped as duplicates are left out.
	Reports(ctx context.Context, code string) ([]Report, error)
	ImpactedPlots(ctx context.Context, code string) ([]ImpactedPlot, error)
	PriorityCandidates(ctx context.Context) ([]PriorityCandidate, error)
}

var (
	ErrNotFound         = apperror.NotFound("ASSET_NOT_FOUND", "bangunan tidak ditemukan")
	ErrInvalidKind      = apperror.Invalid("ASSET_INVALID_KIND", "jenis harus salah satu dari: bendung, bangunan_bagi, bangunan_sadap, bangunan_bagi_sadap, bangunan_pelengkap")
	ErrInvalidCondition = apperror.Invalid("ASSET_INVALID_CONDITION", "kondisi harus salah satu dari: baik, rusak_ringan, rusak_sedang, rusak_berat")
	ErrInvalidLimit     = apperror.Invalid("ASSET_INVALID_LIMIT", "limit harus bilangan bulat positif")
)
