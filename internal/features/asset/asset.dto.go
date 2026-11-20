package asset

import (
	"context"

	"irrigation-server/internal/pkg/geo"
)

// The JSON property names below are fixed by the API contract of the assignment.

type ListQuery struct {
	Kind      string `form:"jenis"`
	Condition string `form:"kondisi"`
	CanalName string `form:"saluran"`
}

type PriorityQuery struct {
	Limit int `form:"limit"`
}

type AssetResponse struct {
	Code          string   `json:"kode_aset" example:"KW-0012"`
	Nomenclature  *string  `json:"nomenklatur" example:"B.KW.3"`
	Kind          *string  `json:"jenis" enums:"bendung,bangunan_bagi,bangunan_sadap,bangunan_bagi_sadap,bangunan_pelengkap" example:"bangunan_bagi_sadap"`
	CanalName     *string  `json:"nama_saluran" example:"Primer Kali Wening"`
	Condition     *string  `json:"kondisi" enums:"baik,rusak_ringan,rusak_sedang,rusak_berat" example:"rusak_sedang"`
	ConditionDate *string  `json:"tgl_kondisi" example:"2026-08-14"`
	Lon           *float64 `json:"lon" example:"110.41235"`
	Lat           *float64 `json:"lat" example:"-7.65412"`
	Flags         []string `json:"tanda" example:"jauh_dari_saluran"`
}

type ListResponse struct {
	Count int             `json:"jumlah" example:"128"`
	Data  []AssetResponse `json:"data"`
}

type ReportResponse struct {
	ID           string   `json:"id_laporan" example:"LP-2026-0021"`
	ReportedOn   *string  `json:"tanggal" example:"2026-08-14"`
	Officer      *string  `json:"petugas" example:"Mulyono"`
	Nomenclature *string  `json:"nomenklatur" example:"B.KW.3"`
	DamageType   *string  `json:"jenis_kerusakan" example:"Retak pada sayap"`
	Condition    *string  `json:"kondisi" enums:"baik,rusak_ringan,rusak_sedang,rusak_berat" example:"rusak_sedang"`
	Lat          *float64 `json:"lat"`
	Lon          *float64 `json:"lon"`
	Status       string   `json:"status" enums:"aktif,batal" example:"aktif"`
	Note         *string  `json:"keterangan" example:"Foto terlampir"`
}

type DetailResponse struct {
	AssetResponse
	History []ReportResponse `json:"riwayat"`
}

type ImpactedPlotResponse struct {
	Code   string  `json:"kode_petak" example:"KW.3.Ki"`
	AreaHa float64 `json:"luas_ha" example:"41.2"`
}

type ImpactResponse struct {
	AssetCode   string                 `json:"kode_aset" example:"KW-0012"`
	PlotCount   int                    `json:"jumlah_petak" example:"7"`
	TotalAreaHa float64                `json:"luas_terdampak_ha" example:"312.45"`
	Plots       []ImpactedPlotResponse `json:"petak"`
}

type PriorityResponse struct {
	Rank           int     `json:"peringkat" example:"1"`
	Code           string  `json:"kode_aset" example:"KW-0012"`
	Condition      string  `json:"kondisi" enums:"rusak_ringan,rusak_sedang,rusak_berat" example:"rusak_berat"`
	ImpactedAreaHa float64 `json:"luas_terdampak_ha" example:"312.45"`
	Score          float64 `json:"skor" example:"1249.8"`
}

type PriorityListResponse struct {
	Data []PriorityResponse `json:"data"`
}

// coordinatePrecision keeps about 1 cm, well inside the checker's 1% tolerance.
const coordinatePrecision = 7

func roundCoordinate(v *float64) *float64 {
	if v == nil {
		return nil
	}
	rounded := geo.Round(*v, coordinatePrecision)
	return &rounded
}

func ToResponse(a Asset) AssetResponse {
	flags := a.Flags
	if flags == nil {
		flags = []string{} // an empty list is returned as [], never null
	}
	return AssetResponse{
		Code: a.Code, Nomenclature: a.Nomenclature, Kind: a.Kind, CanalName: a.CanalName,
		Condition: a.Condition, ConditionDate: a.ConditionDate,
		Lon: roundCoordinate(a.Lon), Lat: roundCoordinate(a.Lat), Flags: flags,
	}
}

func ToListResponse(assets []Asset) ListResponse {
	data := make([]AssetResponse, len(assets))
	for i, a := range assets {
		data[i] = ToResponse(a)
	}
	return ListResponse{Count: len(data), Data: data}
}

func ToDetailResponse(a Asset, reports []Report) DetailResponse {
	history := make([]ReportResponse, len(reports))
	for i, r := range reports {
		history[i] = ReportResponse{
			ID: r.ID, ReportedOn: r.ReportedOn, Officer: r.Officer, Nomenclature: r.Nomenclature,
			DamageType: r.DamageType, Condition: r.Condition, Lat: r.Lat, Lon: r.Lon, Status: r.Status, Note: r.Note,
		}
	}
	return DetailResponse{AssetResponse: ToResponse(a), History: history}
}

func ToImpactResponse(impact Impact) ImpactResponse {
	plots := make([]ImpactedPlotResponse, len(impact.Plots))
	for i, p := range impact.Plots {
		plots[i] = ImpactedPlotResponse{Code: p.Code, AreaHa: geo.Round(p.AreaHa, 2)}
	}
	return ImpactResponse{AssetCode: impact.AssetCode, PlotCount: len(plots), TotalAreaHa: impact.TotalAreaHa, Plots: plots}
}

func ToPriorityListResponse(priorities []Priority) PriorityListResponse {
	data := make([]PriorityResponse, len(priorities))
	for i, p := range priorities {
		data[i] = PriorityResponse{Rank: p.Rank, Code: p.Code, Condition: p.Condition, ImpactedAreaHa: p.ImpactedAreaHa, Score: p.Score}
	}
	return PriorityListResponse{Data: data}
}

type Service interface {
	List(ctx context.Context, q ListQuery) ([]Asset, error)
	Get(ctx context.Context, code string) (Asset, []Report, error)
	Impact(ctx context.Context, code string) (Impact, error)
	Priorities(ctx context.Context, limit int) ([]Priority, error)
}
