package ingest

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"

	"irrigation-server/internal/pkg/geo"
	"irrigation-server/internal/pkg/normalize"
)

// PolygonRepairer validates one WGS84 polygon and repairs it when needed.
type PolygonRepairer func(geojson string) (PolygonRepair, error)

// parsePolygons repairs coordinates and closes rings that the source left open.
func parsePolygons(f Feature) (polygons [][][]geo.LonLat, fromUTM, swapped, closed, ok bool) {
	var raw [][][][]float64
	switch f.GeometryType {
	case "Polygon":
		var single [][][]float64
		if json.Unmarshal(f.Coordinates, &single) != nil {
			return
		}
		raw = [][][][]float64{single}
	case "MultiPolygon":
		if json.Unmarshal(f.Coordinates, &raw) != nil {
			return
		}
	default:
		return
	}
	for _, polygon := range raw {
		var rings [][]geo.LonLat
		for _, ring := range polygon {
			fixed, good := fixSequence(ring, &fromUTM, &swapped)
			if !good || len(fixed) < 3 {
				return nil, fromUTM, swapped, closed, false
			}
			if fixed[0] != fixed[len(fixed)-1] {
				fixed = append(fixed, fixed[0])
				closed = true
			}
			if len(fixed) < 4 {
				return nil, fromUTM, swapped, closed, false
			}
			rings = append(rings, fixed)
		}
		if len(rings) > 0 {
			polygons = append(polygons, rings)
		}
	}
	return polygons, fromUTM, swapped, closed, len(polygons) > 0
}

// repairWorkers bounds how many polygons are validated in PostGIS at once.
// It stays below the default pgx pool size so the import never queues on its
// own connections.
const repairWorkers = 4

// plotGeometry is the parsed and validated geometry of one plot feature.
type plotGeometry struct {
	fromUTM, swapped, closed, ok bool
	rings                        [][]geo.Point
	repair                       PolygonRepair
}

// preparePlotGeometry parses one feature and has PostGIS validate it. It
// touches nothing shared, so many of them can run at the same time.
func preparePlotGeometry(f Feature, repair PolygonRepairer) plotGeometry {
	var g plotGeometry
	var polygons [][][]geo.LonLat
	polygons, g.fromUTM, g.swapped, g.closed, g.ok = parsePolygons(f)
	if !g.ok {
		return g
	}
	all := make([]any, len(polygons))
	for pi, polygon := range polygons {
		ringList := make([]any, len(polygon))
		for ri, ring := range polygon {
			ringList[ri] = positions(ring)
			g.rings = append(g.rings, toUTM(ring))
		}
		all[pi] = ringList
	}
	result, err := repair(geometryJSON("MultiPolygon", all))
	if err != nil || result.AreaM2 <= 0 {
		g.ok = false
		return g
	}
	g.repair = result
	return g
}

// preparePlotGeometries validates every polygon concurrently. Each plot is
// one independent round trip to PostGIS, which is the slow part of the plot
// import. Results are written by index, so the output order is the file order
// no matter which goroutine finishes first.
func preparePlotGeometries(features []Feature, repair PolygonRepairer) []plotGeometry {
	geometries := make([]plotGeometry, len(features))
	slots := make(chan struct{}, repairWorkers)
	var wg sync.WaitGroup
	for i := range features {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			// A panic in a worker would crash the server. The zero value
			// left behind marks the plot as having no usable geometry.
			defer func() {
				if rec := recover(); rec != nil {
					geometries[i] = plotGeometry{}
				}
			}()
			geometries[i] = preparePlotGeometry(features[i], repair)
		})
	}
	wg.Wait()
	return geometries
}

// CleanPlots repairs coordinates and geometry, corrects the unit of
// luas_baku, flags implausible areas, and drops repeated kode_petak.
//
// Geometry is validated concurrently first; the rules are then applied in
// file order on one goroutine, because "keep the first of a repeated code"
// and the order of the findings both depend on that order.
func CleanPlots(features []Feature, repair PolygonRepairer, rec *Recorder) []*Plot {
	geometries := preparePlotGeometries(features, repair)

	seen := map[string]bool{}
	var plots []*Plot
	for i, f := range features {
		code := normalize.Text(f.Properties["kode_petak"])
		rowRef := fmt.Sprintf("fitur %d (%s)", i+1, code)
		if code == "" {
			rec.Add(FilePlots, rowRef, categoryMissingID, "kode_petak kosong", "Fitur dibuang")
			continue
		}
		p := &Plot{Code: code, Name: normalize.Text(f.Properties["nama_petak"]), index: i}
		rawOfftake := normalize.Text(f.Properties["nomenklatur_sadap"])
		p.OfftakeNomenclature = normalize.Nomenclature(rawOfftake)
		rec.text(FilePlots, rowRef, categoryNomenclature, "nomenklatur_sadap", rawOfftake, p.OfftakeNomenclature)

		applyPlotGeometry(p, geometries[i], rowRef, rec)
		cleanPlotArea(p, f.Properties["luas_baku"], rowRef, rec)

		if seen[strings.ToUpper(code)] {
			rec.Add(FilePlots, rowRef, CategoryDuplicatePlot, "kode_petak sudah ada", "Fitur dibuang, satu disimpan")
			continue
		}
		seen[strings.ToUpper(code)] = true
		plots = append(plots, p)
	}
	return plots
}

func applyPlotGeometry(p *Plot, g plotGeometry, rowRef string, rec *Recorder) {
	if !g.ok {
		rec.Add(FilePlots, rowRef, categoryUnreadableGeom, "Geometri kosong atau tidak terbaca", "Petak disimpan tanpa geometri")
		return
	}
	rec.coordinate(FilePlots, rowRef, g.fromUTM, g.swapped)
	p.GeomHex, p.GeometryAreaHa, p.rings = g.repair.GeomHex, g.repair.AreaM2/10000, g.rings
	if !g.repair.WasValid || g.closed {
		problem := "Poligon tidak valid (berpotongan sendiri)"
		if g.closed {
			problem = "Cincin poligon tidak tertutup"
		}
		rec.Add(FilePlots, rowRef, CategoryGeometryRepaired, problem, "Geometri diperbaiki")
	}
}

func cleanPlotArea(p *Plot, rawArea any, rowRef string, rec *Recorder) {
	area, hasArea := normalize.NumberFromAny(rawArea)
	if s, isText := rawArea.(string); isText && hasArea && strings.Contains(s, ",") {
		rec.Add(FilePlots, rowRef, categoryDecimalComma, fmt.Sprintf("luas_baku tertulis %q", s), "Dibaca sebagai angka desimal")
	}
	hasArea = hasArea && area > 0
	if hasArea {
		// The unit is not stated per row. The geometry decides: the value is
		// in m² when value/10000 is closer to the geometry area than the value.
		inSquareMetres := area > 10000
		if p.GeometryAreaHa > 0 {
			inSquareMetres = math.Abs(math.Log(area/10000/p.GeometryAreaHa)) < math.Abs(math.Log(area/p.GeometryAreaHa))
		}
		if inSquareMetres {
			rec.Add(FilePlots, rowRef, categoryAreaFromM2, fmt.Sprintf("luas_baku %v tercatat dalam m²", area),
				fmt.Sprintf("Diubah menjadi %v ha", geo.Round(area/10000, 2)))
			area /= 10000
		}
		p.AttributeAreaHa = &area
		p.AreaHa = area
	}
	switch {
	case p.GeometryAreaHa > 0 && !hasArea:
		p.AreaHa = p.GeometryAreaHa
		p.Flags = append(p.Flags, CategoryImplausibleArea)
		rec.Add(FilePlots, rowRef, CategoryImplausibleArea, "luas_baku kosong atau tidak terbaca",
			fmt.Sprintf("Dipakai luas geometri %.2f ha", p.GeometryAreaHa))
	case p.GeometryAreaHa > 0 && math.Abs(area-p.GeometryAreaHa)/p.GeometryAreaHa > AreaTolerance:
		p.AreaHa = p.GeometryAreaHa
		p.Flags = append(p.Flags, CategoryImplausibleArea)
		rec.Add(FilePlots, rowRef, CategoryImplausibleArea,
			fmt.Sprintf("luas_baku %.2f ha berselisih lebih dari 20%% dari luas geometri %.2f ha", area, p.GeometryAreaHa),
			"Dipakai luas geometri")
	}
}
