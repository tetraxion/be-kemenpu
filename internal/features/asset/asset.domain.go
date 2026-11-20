// Package asset serves irrigation structures.
package asset

import (
	"context"

	"irrigation-server/internal/pkg/apperror"
)

const DefaultPriorityLimit = 20

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

type ImpactedPlot struct {
	Code   string
	AreaHa float64
}

type Impact struct {
	AssetCode   string
	TotalAreaHa float64
	Plots       []ImpactedPlot
}

// PriorityCandidate affects at least one plot.
type PriorityCandidate struct {
	Code           string
	Condition      string
	ImpactedAreaHa float64
}

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
	// Reports includes cancelled reports but not duplicates, newest first.
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
