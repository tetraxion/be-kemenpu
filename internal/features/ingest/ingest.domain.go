// Package ingest reads the source files, cleans them by the rules of the
// assignment, derives the network analysis, and owns every write to the
// irrigation schema. Other features only read that schema.
package ingest

import (
	"context"
	"fmt"
	"math"
	"strings"

	"irrigation-server/internal/pkg/apperror"
	"irrigation-server/internal/pkg/geo"
)

// Source file names inside the data folder.
const (
	FileAssets   = "aset_bangunan.csv"
	FileReports  = "laporan_penelusuran.xlsx"
	FileFollowUp = "laporan_susulan.xlsx"
	FileCanals   = "saluran.geojson"
	FilePlots    = "petak_tersier.geojson"
)

// Distances and tolerances taken from the assignment (metres unless noted).
const (
	ConnectToleranceM   = 10.0   // part D: two segment ends count as connected
	JunctionToleranceM  = 30.0   // assumption: a structure this close to a junction sits on the junction
	MaxCanalDistanceM   = 50.0   // beyond this: jauh_dari_saluran
	MaxValidDistanceM   = 2000.0 // beyond this: koordinat_tidak_valid
	MaxReportMatchM     = 30.0   // report matched to the nearest structure
	MaxOfftakeMatchM    = 200.0  // plot matched to the nearest offtake structure
	AreaTolerance       = 0.20   // attribute vs geometry area
	duplicateToleranceM = 0.5    // two segments are the same when every vertex coincides
)

// Flags and finding categories. The first eleven are required by the API
// contract and keep the exact names given there.
const (
	CategoryUTMConverted     = "koordinat_utm_dikonversi"
	CategorySwappedFixed     = "koordinat_tertukar_dibetulkan"
	CategoryInvalidCoord     = "koordinat_tidak_valid"
	CategoryFarFromCanal     = "jauh_dari_saluran"
	CategoryDuplicateAsset   = "aset_ganda"
	CategoryDuplicateSegment = "ruas_ganda"
	CategoryDuplicatePlot    = "petak_ganda"
	CategoryGeometryRepaired = "geometri_diperbaiki"
	CategoryImplausibleArea  = "luas_tidak_wajar"
	CategoryDuplicateReport  = "laporan_ganda"
	CategoryReportNoAsset    = "laporan_tanpa_aset"

	CategoryNoOfftake          = "tanpa_sadap"
	categoryAreaFromM2         = "luas_m2_dikonversi"
	categoryNomenclature       = "nomenklatur_diseragamkan"
	categoryCondition          = "kondisi_diseragamkan"
	categoryUnknownCondition   = "kondisi_tidak_dikenal"
	categoryUnknownKind        = "jenis_tidak_dikenal"
	categoryDate               = "tanggal_diseragamkan"
	categoryInvalidDate        = "tanggal_tidak_valid"
	categoryDecimalComma       = "angka_koma_desimal"
	categoryUnreadableGeom     = "geometri_tidak_terbaca"
	categoryMissingID          = "pengenal_kosong"
	categoryOfftakeByDistance  = "petak_sadap_dari_jarak"
	categoryReportByDistance   = "laporan_dicocokkan_dari_jarak"
	categoryReportCoordIgnored = "koordinat_laporan_diabaikan"
	categoryUnreadableFile     = "berkas_tidak_terbaca"
	categoryNoWeir             = "bendung_tidak_ditemukan"
)

// Import statuses reported by GET /api/health.
const (
	StatusLoading = "memuat"
	StatusReady   = "siap"
	StatusFailed  = "gagal"
)

var (
	ErrFollowUpMissing = apperror.NotFound("FOLLOW_UP_FILE_NOT_FOUND", FileFollowUp+" tidak ada di folder data")
	ErrNotReady        = apperror.Unavailable("IMPORT_IN_PROGRESS", "impor data belum selesai")
	// ErrImportFailed: waiting will not help; the cause is in the server log.
	ErrImportFailed = apperror.Unavailable("IMPORT_FAILED", "impor data gagal, periksa log server")
	// ErrFollowUpUnreadable: the file exists but is not a readable workbook.
	ErrFollowUpUnreadable = apperror.Unprocessable("FOLLOW_UP_FILE_UNREADABLE", FileFollowUp+" tidak dapat dibaca sebagai berkas Excel")

	// Errors of the optional upload (multipart field UploadField).
	ErrUploadMissing    = apperror.Invalid("UPLOAD_FILE_MISSING", "berkas tidak ditemukan pada kolom '"+UploadField+"'")
	ErrUploadTooLarge   = apperror.Invalid("UPLOAD_FILE_TOO_LARGE", "ukuran berkas melebihi 10 MB")
	ErrUploadUnreadable = apperror.Unprocessable("UPLOAD_FILE_UNREADABLE", "berkas yang diunggah tidak dapat dibaca sebagai berkas Excel (.xlsx)")
	// ErrUploadNotReports: a readable workbook that is not a report file.
	ErrUploadNotReports = apperror.Unprocessable("UPLOAD_FILE_NOT_REPORTS", "berkas yang diunggah tidak memuat kolom id_laporan")
)

const (
	// UploadField is the multipart field that carries an uploaded follow-up file.
	UploadField = "berkas"
	// MaxUploadBytes bounds an uploaded follow-up file.
	MaxUploadBytes = 10 << 20
)

// Finding records one cleaning action on one source row or feature.
// Problem and Action are shown to maintenance staff, so they are in Indonesian.
type Finding struct {
	SourceFile, RowRef, Category, Problem, Action string
}

// Recorder collects findings while cleaning.
type Recorder struct{ Findings []Finding }

func (r *Recorder) Add(sourceFile, rowRef, category, problem, action string) {
	r.Findings = append(r.Findings, Finding{sourceFile, rowRef, category, problem, action})
}

// coordinate records a UTM conversion and/or an order correction.
func (r *Recorder) coordinate(sourceFile, rowRef string, fromUTM, swapped bool) {
	if fromUTM {
		r.Add(sourceFile, rowRef, CategoryUTMConverted, "Koordinat dalam UTM 49S", "Dikonversi ke WGS84")
	}
	if swapped {
		r.Add(sourceFile, rowRef, CategorySwappedFixed, "Lintang dan bujur tertukar", "Urutan dibetulkan")
	}
}

// text records a normalisation when the result differs from the original.
func (r *Recorder) text(sourceFile, rowRef, category, column, original, result string) {
	if result != "" && strings.TrimSpace(original) != result {
		r.Add(sourceFile, rowRef, category, fmt.Sprintf("%s tertulis %q", column, original), "Diseragamkan menjadi "+result)
	}
}

// Canal is one cleaned canal segment.
type Canal struct {
	SegmentCode, CanalName, Level, Condition string
	Parts                                    [][]geo.LonLat // WGS84, one entry per line part
	UTM                                      [][]geo.Point
	LengthM                                  float64
}

// Asset is one cleaned irrigation structure.
type Asset struct {
	Code, Nomenclature, Kind, CanalName string
	InventoryCondition, InventoryDate   string // "" when absent
	BuiltYear                           *int
	Location                            *geo.LonLat
	UTM                                 geo.Point
	CanalDistanceM                      *float64
	Flags                               []string
	ImpactedAreaHa                      float64
	ImpactedPlots                       []string

	row           int
	invalidReason string
}

// SetLocation stores the WGS84 location together with its UTM equivalent.
func (a *Asset) SetLocation(lon, lat float64) {
	a.Location = &geo.LonLat{Lon: lon, Lat: lat}
	a.UTM = geo.ToUTM(*a.Location)
}

func (a *Asset) hasFlag(flag string) bool {
	for _, f := range a.Flags {
		if f == flag {
			return true
		}
	}
	return false
}

// onNetwork reports whether the structure may take part in the impact analysis.
func (a *Asset) onNetwork() bool { return a.Location != nil && !a.hasFlag(CategoryFarFromCanal) }

// Plot is one cleaned tertiary plot.
type Plot struct {
	Code, Name          string
	AttributeAreaHa     *float64 // luas_baku after the unit is corrected
	GeometryAreaHa      float64
	AreaHa              float64 // the area used in the analysis
	OfftakeNomenclature string
	OfftakeAssetCode    string
	Flags               []string
	GeomHex             string // hex EWKB of the valid geometry; "" when absent

	rings [][]geo.Point // polygon rings in UTM, for boundary distances
	index int
}

// Report is one cleaned field report.
type Report struct {
	ID, ReportedOn, Officer       string
	Nomenclature, RawNomenclature string
	DamageType, Condition, Note   string
	Status                        string
	Lat, Lon                      *float64
	AssetCode                     string // "" when no structure matches
	SourceFile                    string
}

// SameContent compares two reports after normalisation. The raw nomenclature
// is left out on purpose: "B KW 7A" and "B.KW.7a" are the same report.
func (r Report) SameContent(o Report) bool {
	sameNumber := func(a, b *float64) bool {
		if a == nil || b == nil {
			return a == b
		}
		return math.Abs(*a-*b) < 1e-7
	}
	return r.ReportedOn == o.ReportedOn && r.Officer == o.Officer && r.Nomenclature == o.Nomenclature &&
		r.DamageType == o.DamageType && r.Condition == o.Condition && r.Note == o.Note &&
		r.Status == o.Status && r.AssetCode == o.AssetCode && sameNumber(r.Lat, o.Lat) && sameNumber(r.Lon, o.Lon)
}

// AssetPoint is a structure location used to match reports by distance.
type AssetPoint struct {
	Code string
	UTM  geo.Point
}

// Reference is the set of structures reports are matched against.
type Reference struct {
	ByNomenclature map[string]string // canonical nomenclature -> asset code
	Points         []AssetPoint
}

// NewReference builds a reference from cleaned assets.
func NewReference(assets []*Asset) Reference {
	ref := Reference{ByNomenclature: map[string]string{}}
	for _, a := range assets {
		if _, exists := ref.ByNomenclature[a.Nomenclature]; a.Nomenclature != "" && !exists {
			ref.ByNomenclature[a.Nomenclature] = a.Code
		}
		if a.Location != nil {
			ref.Points = append(ref.Points, AssetPoint{a.Code, a.UTM})
		}
	}
	return ref
}

// Snapshot is the full result of the initial import.
type Snapshot struct {
	Canals   []Canal
	Assets   []*Asset
	Plots    []*Plot
	Reports  []Report
	Findings []Finding
}

// PolygonRepair is the outcome of validating one plot polygon.
type PolygonRepair struct {
	WasValid bool
	GeomHex  string  // hex EWKB of the repaired geometry
	AreaM2   float64 // measured on EPSG:32749
}

// FollowUpChange is what a follow-up import wants written.
type FollowUpChange struct {
	SourceFile string
	Upserts    []Report
	Findings   []Finding // replaces the earlier findings of SourceFile
}

// Summary is the result of a follow-up import.
type Summary struct {
	Added, Updated, Cancelled, Unchanged int
}

type Repository interface {
	// RepairPolygon validates a WGS84 polygon (GeoJSON) and repairs it when needed.
	RepairPolygon(ctx context.Context, geojson string) (PolygonRepair, error)
	// ReplaceAll replaces the whole schema content with the snapshot and
	// recomputes everything derived from reports, in one transaction.
	ReplaceAll(ctx context.Context, snap Snapshot) error
	// FollowUp hands decide the stored structures and reports, applies the
	// change it returns, and recomputes derived state, in one transaction.
	FollowUp(ctx context.Context, decide func(ref Reference, existing map[string]Report) FollowUpChange) error
}
