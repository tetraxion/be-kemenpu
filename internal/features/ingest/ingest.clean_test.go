package ingest

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"irrigation-server/internal/pkg/geo"
)

// origin is a point inside the UTM 49S zone; test geometry is laid out in
// metres around it and converted to whatever the source file would contain.
var origin = geo.Point{X: 430000, Y: 9155000}

func at(dx, dy float64) geo.Point { return geo.Point{X: origin.X + dx, Y: origin.Y + dy} }

func wgs(dx, dy float64) geo.LonLat { return geo.ToWGS84(at(dx, dy)) }

func coordinates(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

// lineFeature builds a LineString feature in WGS84 from metre offsets.
func lineFeature(t *testing.T, code, condition string, offsets ...[2]float64) Feature {
	t.Helper()
	var line [][2]float64
	for _, o := range offsets {
		p := wgs(o[0], o[1])
		line = append(line, [2]float64{p.Lon, p.Lat})
	}
	return Feature{
		Properties:   map[string]any{"id_ruas": code, "nama_saluran": "Primer Uji", "tingkat": " PRIMER", "kondisi": condition},
		GeometryType: "LineString", Coordinates: coordinates(t, line),
	}
}

func categories(rec *Recorder) map[string]int {
	out := map[string]int{}
	for _, f := range rec.Findings {
		out[f.Category]++
	}
	return out
}

func TestCleanCanalsDropsDuplicatesAndConvertsCoordinates(t *testing.T) {
	utmLine := [][2]float64{{origin.X + 1000, origin.Y}, {origin.X + 2000, origin.Y}}
	swappedStart, swappedEnd := wgs(2000, 0), wgs(3000, 0)
	features := []Feature{
		lineFeature(t, "R1", "RR", [2]float64{0, 0}, [2]float64{1000, 0}),
		lineFeature(t, "R1-COPY", "rr", [2]float64{1000, 0}, [2]float64{0, 0}), // same geometry, drawn backwards
		{Properties: map[string]any{"id_ruas": "R2", "tingkat": "primer", "kondisi": "b"},
			GeometryType: "LineString", Coordinates: coordinates(t, utmLine)},
		{Properties: map[string]any{"id_ruas": "R3", "tingkat": "sekunder", "kondisi": "Rusak Berat"},
			GeometryType: "LineString", Coordinates: coordinates(t, [][2]float64{
				{swappedStart.Lat, swappedStart.Lon}, {swappedEnd.Lat, swappedEnd.Lon}})},
		{Properties: map[string]any{"id_ruas": "R4"}}, // no geometry
	}
	rec := &Recorder{}

	canals := CleanCanals(features, rec)

	require.Len(t, canals, 3)
	assert.Equal(t, "R1", canals[0].SegmentCode)
	assert.Equal(t, "primer", canals[0].Level)
	assert.Equal(t, "rusak_ringan", canals[0].Condition)
	assert.Equal(t, "rusak_berat", canals[2].Condition)
	for _, c := range canals {
		assert.InDelta(t, 1000, c.LengthM, 0.01, "length of %s is measured on EPSG:32749", c.SegmentCode)
	}
	got := categories(rec)
	assert.Equal(t, 1, got[CategoryDuplicateSegment])
	assert.Equal(t, 1, got[CategoryUTMConverted])
	assert.Equal(t, 1, got[CategorySwappedFixed])
	assert.Equal(t, 1, got[categoryUnreadableGeom])
}

func assetRow(code, nomenclature, x, y, condition, date string) Row {
	return Row{"kode_aset": code, "nomenklatur": nomenclature, "jenis": "Bangunan Sadap", "nama_saluran": "Primer Uji",
		"x": x, "y": y, "kondisi": condition, "tahun_bangun": "1990", "tgl_update": date}
}

func lonLatText(p geo.LonLat) (string, string) {
	return fmt.Sprintf("%.7f", p.Lon), fmt.Sprintf("%.7f", p.Lat)
}

func TestCleanAssetsAppliesTheCoordinateRules(t *testing.T) {
	canals := CleanCanals([]Feature{lineFeature(t, "R1", "baik", [2]float64{0, 0}, [2]float64{5000, 0})}, &Recorder{})

	nearLon, nearLat := lonLatText(wgs(100, 20))   // 20 m from the canal
	limitLon, limitLat := lonLatText(wgs(200, 49)) // just inside 50 m
	farLon, farLat := lonLatText(wgs(300, 400))    // 400 m: flagged, geometry kept
	awayLon, awayLat := lonLatText(wgs(400, 2500)) // 2.5 km: geometry dropped
	rows := []Row{
		assetRow("KW-0001", "bkw1", nearLon, nearLat, "RR", "2025-01-10"),
		assetRow("KW-0002", "B KW 2", limitLat, limitLon, "b", "10/01/2025"),                                                   // swapped
		assetRow("KW-0003", "b-kw-3", fmt.Sprintf("%.2f", origin.X+500), fmt.Sprintf("%.2f", origin.Y+10), "rs", "2025/01/10"), // UTM
		assetRow("KW-0004", "B.KW.4", farLon, farLat, "baik", "2025-01-10"),
		assetRow("KW-0005", "B.KW.5", awayLon, awayLat, "baik", "2025-01-10"),
		assetRow("KW-0006", "B.KW.6", "0", "0", "baik", "2025-01-10"),
		assetRow("KW-0007", "B.KW.7", "", "", "baik", "2025-01-10"),
	}
	rec := &Recorder{}

	assets := CleanAssets(rows, canals, rec)

	require.Len(t, assets, 7)
	byCode := map[string]*Asset{}
	for _, a := range assets {
		byCode[a.Code] = a
	}
	assert.Equal(t, "B.KW.1", byCode["KW-0001"].Nomenclature)
	assert.Empty(t, byCode["KW-0001"].Flags)
	assert.Empty(t, byCode["KW-0002"].Flags, "49 m is not far from the canal")
	assert.Equal(t, "2025-01-10", byCode["KW-0002"].InventoryDate)
	assert.NotNil(t, byCode["KW-0003"].Location)
	assert.Equal(t, []string{CategoryFarFromCanal}, byCode["KW-0004"].Flags)
	assert.NotNil(t, byCode["KW-0004"].Location, "a far structure keeps its geometry")
	for _, code := range []string{"KW-0005", "KW-0006", "KW-0007"} {
		assert.Equal(t, []string{CategoryInvalidCoord}, byCode[code].Flags, code)
		assert.Nil(t, byCode[code].Location, code)
	}

	got := categories(rec)
	assert.Equal(t, 1, got[CategorySwappedFixed])
	assert.Equal(t, 1, got[CategoryUTMConverted])
	assert.Equal(t, 1, got[CategoryFarFromCanal])
	assert.Equal(t, 3, got[CategoryInvalidCoord])
}

func TestCleanAssetsKeepsTheLatestOfADuplicatedCode(t *testing.T) {
	lon, lat := lonLatText(wgs(100, 0))
	rows := []Row{
		assetRow("KW-0011", "B.SR.2", lon, lat, "b", "2024-12-25"),
		assetRow("KW-0012", "B.SR.3", lon, lat, "baik", "1 Juli 2025"),
		assetRow("kw-0011", "b_sr_2", lon, lat, "rusak berat", "17-06-2025"), // newer, different date format
		assetRow("KW-0012", "B.SR.3", lon, lat, "rr", "2025/03/11"),          // older
	}
	rec := &Recorder{}

	assets := CleanAssets(rows, nil, rec)

	require.Len(t, assets, 2)
	assert.Equal(t, "rusak_berat", assets[0].InventoryCondition)
	assert.Equal(t, "2025-06-17", assets[0].InventoryDate)
	assert.Equal(t, "baik", assets[1].InventoryCondition)
	assert.Equal(t, "2025-07-01", assets[1].InventoryDate)
	assert.Equal(t, 2, categories(rec)[CategoryDuplicateAsset])
}

// squareFeature builds a square plot of the given side (metres) in WGS84.
func squareFeature(t *testing.T, code string, area any, offtake string, x, y, side float64, close bool) Feature {
	t.Helper()
	corners := [][2]float64{{x, y}, {x + side, y}, {x + side, y + side}, {x, y + side}}
	if close {
		corners = append(corners, corners[0])
	}
	var ring [][2]float64
	for _, c := range corners {
		p := wgs(c[0], c[1])
		ring = append(ring, [2]float64{p.Lon, p.Lat})
	}
	return Feature{
		Properties:   map[string]any{"kode_petak": code, "nama_petak": "Petak " + code, "luas_baku": area, "nomenklatur_sadap": offtake},
		GeometryType: "Polygon", Coordinates: coordinates(t, [][][2]float64{ring}),
	}
}

// fakeRepair stands in for PostGIS: it reports the polygon as valid with a fixed area.
func fakeRepair(areaM2 float64, wasValid bool) PolygonRepairer {
	return func(string) (PolygonRepair, error) {
		return PolygonRepair{WasValid: wasValid, GeomHex: "00", AreaM2: areaM2}, nil
	}
}

func TestCleanPlotsAppliesTheAreaRules(t *testing.T) {
	features := []Feature{
		squareFeature(t, "P.1", 41.0, "bkw1", 0, 0, 640, true),       // plausible
		squareFeature(t, "P.2", "40,5", "B KW 1", 0, 0, 640, true),   // decimal comma
		squareFeature(t, "P.3", 412000.0, "B.KW.1", 0, 0, 640, true), // m², becomes 41.2 ha
		squareFeature(t, "P.4", 22.5, "B.KW.1", 0, 0, 640, true),     // 45% off: geometry wins
		squareFeature(t, "P.5", 48.9, "B.KW.1", 0, 0, 640, true),     // 19.3% off: attribute kept
		squareFeature(t, "P.6", nil, "B.KW.1", 0, 0, 640, true),      // missing: geometry wins
		squareFeature(t, "P.1", 41.0, "B.KW.1", 0, 0, 640, true),     // repeated code
	}
	rec := &Recorder{}

	plots := CleanPlots(features, fakeRepair(409600, true), rec) // geometry = 40.96 ha

	require.Len(t, plots, 6)
	byCode := map[string]*Plot{}
	for _, p := range plots {
		byCode[p.Code] = p
	}
	assert.Equal(t, "B.KW.1", byCode["P.1"].OfftakeNomenclature)
	assert.InDelta(t, 41.0, byCode["P.1"].AreaHa, 1e-9)
	assert.InDelta(t, 40.5, byCode["P.2"].AreaHa, 1e-9)
	assert.InDelta(t, 41.2, byCode["P.3"].AreaHa, 1e-9)
	assert.Empty(t, byCode["P.3"].Flags, "a unit fix alone is not an implausible area")
	assert.InDelta(t, 40.96, byCode["P.4"].AreaHa, 1e-9)
	assert.Equal(t, []string{CategoryImplausibleArea}, byCode["P.4"].Flags)
	assert.InDelta(t, 48.9, byCode["P.5"].AreaHa, 1e-9)
	assert.Empty(t, byCode["P.5"].Flags)
	assert.InDelta(t, 40.96, byCode["P.6"].AreaHa, 1e-9)
	assert.Equal(t, []string{CategoryImplausibleArea}, byCode["P.6"].Flags)

	got := categories(rec)
	assert.Equal(t, 1, got[CategoryDuplicatePlot])
	assert.Equal(t, 2, got[CategoryImplausibleArea])
	assert.Equal(t, 1, got[categoryAreaFromM2])
	assert.Equal(t, 1, got[categoryDecimalComma])
	assert.Equal(t, 0, got[CategoryGeometryRepaired])
}

func TestCleanPlotsCountsRepairedGeometry(t *testing.T) {
	rec := &Recorder{}
	plots := CleanPlots([]Feature{
		squareFeature(t, "OPEN", 41.0, "", 0, 0, 640, false), // ring not closed in the source
	}, fakeRepair(409600, true), rec)
	require.Len(t, plots, 1)
	assert.Equal(t, 1, categories(rec)[CategoryGeometryRepaired])

	rec = &Recorder{}
	CleanPlots([]Feature{squareFeature(t, "BOWTIE", 41.0, "", 0, 0, 640, true)}, fakeRepair(409600, false), rec)
	assert.Equal(t, 1, categories(rec)[CategoryGeometryRepaired], "a polygon PostGIS had to make valid is counted")
}

func referenceAssets() []*Asset {
	first := &Asset{Code: "KW-0001", Nomenclature: "B.KW.1"}
	first.SetLocation(wgs(0, 0).Lon, wgs(0, 0).Lat)
	second := &Asset{Code: "KW-0002", Nomenclature: "B.KW.2"}
	second.SetLocation(wgs(1000, 0).Lon, wgs(1000, 0).Lat)
	return []*Asset{first, second}
}

func TestCleanReportsMatchesByNomenclatureThenDistance(t *testing.T) {
	ref := NewReference(referenceAssets())
	near := wgs(1000, 25) // 25 m from KW-0002
	away := wgs(1000, 35) // 35 m: beyond the 30 m limit
	rows := []Row{
		{"id_laporan": "LP-1", "tanggal": "22 Agustus 2026", "nomenklatur": "bkw 1", "kondisi": "RB", "status": "AKTIF"},
		{"id_laporan": "LP-2", "tanggal": "46246", "nomenklatur": "", "kondisi": "rr", "status": "aktif",
			"lat": fmt.Sprintf("%.7f", near.Lat), "lon": fmt.Sprintf("%.7f", near.Lon)},
		{"id_laporan": "LP-3", "tanggal": "2026-08-01", "nomenklatur": "B.XZ.7", "kondisi": "rs", "status": "Batal",
			"lat": fmt.Sprintf("%.7f", away.Lat), "lon": fmt.Sprintf("%.7f", away.Lon)},
		{"id_laporan": "LP-4", "tanggal": "2026-08-01", "nomenklatur": "B.XQ.5", "kondisi": "rs"},
		// lat/lon columns swapped: still resolves to KW-0002.
		{"id_laporan": "LP-5", "tanggal": "1/9/2026", "nomenklatur": "B.ZZ.1", "kondisi": "baik",
			"lat": fmt.Sprintf("%.7f", near.Lon), "lon": fmt.Sprintf("%.7f", near.Lat)},
	}
	rec := &Recorder{}

	reports := CleanReports(rows, FileReports, ref, rec)

	require.Len(t, reports, 5)
	assert.Equal(t, "KW-0001", reports[0].AssetCode)
	assert.Equal(t, "2026-08-22", reports[0].ReportedOn)
	assert.Equal(t, "rusak_berat", reports[0].Condition)
	assert.Equal(t, "aktif", reports[0].Status)
	assert.Equal(t, "KW-0002", reports[1].AssetCode, "empty nomenclature falls back to the nearest structure")
	assert.Equal(t, "2026-08-12", reports[1].ReportedOn, "Excel serial date")
	assert.Empty(t, reports[2].AssetCode, "35 m is beyond the matching distance")
	assert.Equal(t, "batal", reports[2].Status)
	assert.Empty(t, reports[3].AssetCode, "unknown nomenclature and no coordinates")
	assert.Equal(t, "KW-0002", reports[4].AssetCode)
	assert.Equal(t, "2026-09-01", reports[4].ReportedOn, "ambiguous date is read day first")
	assert.Equal(t, 1, categories(rec)[CategorySwappedFixed])
}

func TestCleanReportsKeepsTheLastRowOfARepeatedID(t *testing.T) {
	rows := []Row{
		{"id_laporan": "LP-1", "tanggal": "2026-08-01", "nomenklatur": "B.KW.1", "kondisi": "rr"},
		{"id_laporan": "lp-1", "tanggal": "2026-08-02", "nomenklatur": "B.KW.1", "kondisi": "rb"},
	}
	reports := CleanReports(rows, FileReports, NewReference(referenceAssets()), &Recorder{})
	require.Len(t, reports, 1)
	assert.Equal(t, "rusak_berat", reports[0].Condition)
}

func TestClassifyFollowUp(t *testing.T) {
	existing := map[string]Report{
		"LP-1": {ID: "LP-1", ReportedOn: "2026-08-01", Nomenclature: "B.KW.1", Condition: "rusak_ringan", Status: "aktif", AssetCode: "KW-0001"},
		"LP-2": {ID: "LP-2", ReportedOn: "2026-08-02", Nomenclature: "B.KW.1", Condition: "rusak_berat", Status: "aktif", AssetCode: "KW-0001"},
		"LP-3": {ID: "LP-3", ReportedOn: "2026-08-03", Nomenclature: "B.KW.2", Condition: "baik", Status: "aktif", AssetCode: "KW-0002"},
	}
	incoming := []Report{
		// Same content, only the raw spelling differs.
		{ID: "LP-1", ReportedOn: "2026-08-01", Nomenclature: "B.KW.1", RawNomenclature: "bkw 1", Condition: "rusak_ringan", Status: "aktif", AssetCode: "KW-0001"},
		{ID: "LP-2", ReportedOn: "2026-08-02", Nomenclature: "B.KW.1", Condition: "rusak_berat", Status: "batal", AssetCode: "KW-0001"},
		{ID: "LP-3", ReportedOn: "2026-08-03", Nomenclature: "B.KW.2", Condition: "rusak_sedang", Status: "aktif", AssetCode: "KW-0002"},
		{ID: "LP-4", ReportedOn: "2026-10-01", Nomenclature: "B.KW.2", Condition: "baik", Status: "aktif", AssetCode: "KW-0002"},
	}

	summary, upserts := ClassifyFollowUp(incoming, existing)

	assert.Equal(t, Summary{Added: 1, Updated: 1, Cancelled: 1, Unchanged: 1}, summary)
	assert.Len(t, upserts, 3, "an unchanged report is not written again")

	// Applying the same file to the resulting state changes nothing.
	for _, r := range upserts {
		existing[r.ID] = r
	}
	again, upsertsAgain := ClassifyFollowUp(incoming, existing)
	assert.Equal(t, Summary{Unchanged: 4}, again)
	assert.Empty(t, upsertsAgain)
}

// TestAnalyzeMatchesTheAssignmentExample reproduces the worked example:
// B.KW.2 affects four plots totalling 150 ha, and KW.1.Ki upstream is spared.
func TestAnalyzeMatchesTheAssignmentExample(t *testing.T) {
	var canals []Canal
	for _, line := range exampleLines() {
		canals = append(canals, Canal{UTM: [][]geo.Point{line}})
	}
	points := examplePoints()
	structure := func(code, nomenclature, kind, point string) *Asset {
		a := &Asset{Code: code, Nomenclature: nomenclature, Kind: kind, Location: &geo.LonLat{}, UTM: points[point]}
		return a
	}
	far := structure("KW-0008", "B.KW.9", "bangunan_sadap", "B.KW.3")
	far.Flags = []string{CategoryFarFromCanal}
	assets := []*Asset{
		structure("KW-0001", "B.KW.0", "bendung", "weir"),
		structure("KW-0002", "B.KW.1", "bangunan_sadap", "B.KW.1"),
		structure("KW-0003", "B.KW.2", "bangunan_bagi_sadap", "B.KW.2"),
		structure("KW-0004", "B.KW.3", "bangunan_sadap", "B.KW.3"),
		structure("KW-0005", "B.SR.1", "bangunan_sadap", "B.SR.1"),
		structure("KW-0006", "B.SR.2", "bangunan_sadap", "B.SR.2"),
		structure("KW-0007", "B.KW.1A", "bangunan_pelengkap", "aqueduct"),
		far,
	}
	plots := []*Plot{
		{Code: "KW.1.Ki", AreaHa: 30, OfftakeNomenclature: "B.KW.1"},
		{Code: "KW.2.Ka", AreaHa: 40, OfftakeNomenclature: "B.KW.2"},
		{Code: "KW.3.Ki", AreaHa: 50, OfftakeNomenclature: "B.KW.3"},
		{Code: "SR.1.Ki", AreaHa: 25, OfftakeNomenclature: "B.SR.1"},
		{Code: "SR.2.Ka", AreaHa: 35, OfftakeNomenclature: "B.SR.2"},
		{Code: "XX.1.Ki", AreaHa: 99, OfftakeNomenclature: "B.XX.1"}, // unknown offtake, no geometry
		{Code: "KW.9.Ki", AreaHa: 77, OfftakeNomenclature: "B.KW.9"}, // tapped at a structure off the network
	}
	rec := &Recorder{}

	Analyze(canals, assets, plots, rec)

	area := map[string]float64{}
	impacted := map[string][]string{}
	for _, a := range assets {
		area[a.Nomenclature] = a.ImpactedAreaHa
		impacted[a.Nomenclature] = a.ImpactedPlots
	}
	assert.Equal(t, 180.0, area["B.KW.0"], "the weir affects every served plot")
	assert.Equal(t, 180.0, area["B.KW.1"])
	assert.Equal(t, 150.0, area["B.KW.1A"], "a structure between B.KW.1 and B.KW.2 spares KW.1.Ki")
	assert.Equal(t, 150.0, area["B.KW.2"])
	assert.ElementsMatch(t, []string{"KW.2.Ka", "KW.3.Ki", "SR.1.Ki", "SR.2.Ka"}, impacted["B.KW.2"])
	assert.Equal(t, 50.0, area["B.KW.3"])
	assert.Equal(t, 60.0, area["B.SR.1"])
	assert.Equal(t, 35.0, area["B.SR.2"])
	assert.Equal(t, 0.0, area["B.KW.9"], "a structure far from the canal has no impact")

	assert.Equal(t, []string{CategoryNoOfftake}, plots[5].Flags)
	assert.Equal(t, "KW-0008", plots[6].OfftakeAssetCode)
	assert.Equal(t, 1, categories(rec)[CategoryNoOfftake])
}

func TestAnalyzeFallsBackToTheNearestOfftake(t *testing.T) {
	canals := []Canal{{UTM: [][]geo.Point{{{X: 0, Y: 0}, {X: 2000, Y: 0}}}}}
	asset := func(code, nomenclature, kind string, x, y float64) *Asset {
		return &Asset{Code: code, Nomenclature: nomenclature, Kind: kind, Location: &geo.LonLat{}, UTM: geo.Point{X: x, Y: y}}
	}
	assets := []*Asset{
		asset("A1", "B.KW.0", "bendung", 0, 0),
		asset("A2", "B.KW.1", "bangunan_sadap", 1000, 0),
		asset("A3", "B.KW.1A", "bangunan_pelengkap", 1500, 0), // closer, but not an offtake
	}
	square := func(x, y float64) [][]geo.Point {
		return [][]geo.Point{{{X: x, Y: y}, {X: x + 100, Y: y}, {X: x + 100, Y: y + 100}, {X: x, Y: y + 100}, {X: x, Y: y}}}
	}
	plots := []*Plot{
		{Code: "NEAR", AreaHa: 10, rings: square(1150, 50)}, // 158 m from A2
		{Code: "FAR", AreaHa: 20, rings: square(1150, 250)}, // 292 m from A2
	}
	rec := &Recorder{}

	Analyze(canals, assets, plots, rec)

	assert.Equal(t, "A2", plots[0].OfftakeAssetCode)
	assert.Empty(t, plots[1].OfftakeAssetCode)
	assert.Equal(t, []string{CategoryNoOfftake}, plots[1].Flags)
	assert.Equal(t, 10.0, assets[1].ImpactedAreaHa)
	assert.Equal(t, 10.0, assets[0].ImpactedAreaHa)
}
