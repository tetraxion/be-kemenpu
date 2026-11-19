package ingest

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"irrigation-server/internal/pkg/geo"
	"irrigation-server/internal/pkg/normalize"
)

func distanceToCanals(p geo.Point, canals []Canal) float64 {
	nearest := math.Inf(1)
	for _, c := range canals {
		for _, line := range c.UTM {
			nearest = math.Min(nearest, geo.DistanceToLine(p, line))
		}
	}
	return nearest
}

// CleanAssets normalises the inventory, keeps the most recently updated row
// of each repeated kode_aset, and flags locations by their distance to the
// nearest canal. Assets come back ordered by code.
func CleanAssets(rows []Row, canals []Canal, rec *Recorder) []*Asset {
	byCode := map[string]*Asset{}
	var codes []string
	for i, row := range rows {
		rowNumber := i + 2 // row 1 is the header
		code := strings.ToUpper(normalize.Clean(row["kode_aset"]))
		rowRef := fmt.Sprintf("baris %d (%s)", rowNumber, code)
		if code == "" {
			rec.Add(FileAssets, rowRef, categoryMissingID, "kode_aset kosong", "Baris dibuang")
			continue
		}
		a := &Asset{Code: code, CanalName: normalize.Clean(row["nama_saluran"]), row: rowNumber}
		a.Nomenclature = normalize.Nomenclature(row["nomenklatur"])
		rec.text(FileAssets, rowRef, categoryNomenclature, "nomenklatur", row["nomenklatur"], a.Nomenclature)

		kind, ok := normalize.AssetKind(row["jenis"])
		if !ok {
			kind = normalize.KindAuxiliary
			rec.Add(FileAssets, rowRef, categoryUnknownKind, fmt.Sprintf("jenis tertulis %q", row["jenis"]), "Dianggap bangunan_pelengkap")
		}
		a.Kind = kind
		a.InventoryCondition = rec.condition(FileAssets, rowRef, row["kondisi"])
		a.InventoryDate = rec.date(FileAssets, rowRef, "tgl_update", row["tgl_update"])
		if year, err := strconv.Atoi(strings.TrimSpace(row["tahun_bangun"])); err == nil {
			a.BuiltYear = &year
		}
		c, reason := rec.pair(FileAssets, rowRef, row["x"], row["y"])
		if c.Valid {
			a.SetLocation(c.Lon, c.Lat)
		}
		a.invalidReason = reason

		earlier, exists := byCode[code]
		if !exists {
			byCode[code] = a
			codes = append(codes, code)
			continue
		}
		// ISO dates compare correctly as text. On equal dates the later row wins.
		dropped, kept := earlier, a
		if a.InventoryDate < earlier.InventoryDate {
			dropped, kept = a, earlier
		}
		byCode[code] = kept
		rec.Add(FileAssets, fmt.Sprintf("baris %d (%s)", dropped.row, code), CategoryDuplicateAsset,
			fmt.Sprintf("kode_aset sama dengan baris %d", kept.row),
			fmt.Sprintf("Baris dibuang, dipakai tgl_update terbaru (%s)", kept.InventoryDate))
	}

	sort.Strings(codes)
	assets := make([]*Asset, 0, len(codes))
	for _, code := range codes {
		a := byCode[code]
		rowRef := fmt.Sprintf("baris %d (%s)", a.row, code)
		if a.Location != nil && len(canals) > 0 {
			d := distanceToCanals(a.UTM, canals)
			switch {
			case d > MaxValidDistanceM:
				a.Location = nil
				a.invalidReason = fmt.Sprintf("Koordinat %.0f m dari saluran terdekat (lebih dari 2 km)", d)
			case d > MaxCanalDistanceM:
				a.CanalDistanceM = &d
				a.Flags = append(a.Flags, CategoryFarFromCanal)
				rec.Add(FileAssets, rowRef, CategoryFarFromCanal, fmt.Sprintf("Bangunan %.0f m dari saluran terdekat", d),
					"Disimpan dengan tanda, tidak ikut analisis dampak")
			default:
				a.CanalDistanceM = &d
			}
		}
		if a.Location == nil {
			a.Flags = append(a.Flags, CategoryInvalidCoord)
			rec.Add(FileAssets, rowRef, CategoryInvalidCoord, a.invalidReason, "Disimpan tanpa geometri")
		}
		assets = append(assets, a)
	}
	return assets
}
