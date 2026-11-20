package ingest

import (
	"encoding/json"
	"fmt"

	"irrigation-server/internal/pkg/geo"
	"irrigation-server/internal/pkg/normalize"
)

func (c Canal) GeoJSON() string {
	if len(c.Parts) == 1 {
		return geometryJSON("LineString", positions(c.Parts[0]))
	}
	parts := make([]any, len(c.Parts))
	for i, p := range c.Parts {
		parts[i] = positions(p)
	}
	return geometryJSON("MultiLineString", parts)
}

func parseLines(f Feature) (parts [][]geo.LonLat, fromUTM, swapped, ok bool) {
	var raw [][][]float64
	switch f.GeometryType {
	case "LineString":
		var single [][]float64
		if json.Unmarshal(f.Coordinates, &single) != nil {
			return
		}
		raw = [][][]float64{single}
	case "MultiLineString":
		if json.Unmarshal(f.Coordinates, &raw) != nil {
			return
		}
	default:
		return
	}
	for _, line := range raw {
		fixed, good := fixSequence(line, &fromUTM, &swapped)
		if !good || len(fixed) < 2 {
			return nil, fromUTM, swapped, false
		}
		parts = append(parts, fixed)
	}
	return parts, fromUTM, swapped, len(parts) > 0
}

// sameLine ignores drawing direction.
func sameLine(a, b []geo.Point) bool {
	if len(a) != len(b) {
		return false
	}
	forward, backward := true, true
	for i := range a {
		forward = forward && geo.Distance(a[i], b[i]) <= duplicateToleranceM
		backward = backward && geo.Distance(a[i], b[len(b)-1-i]) <= duplicateToleranceM
	}
	return forward || backward
}

func sameGeometry(a, b [][]geo.Point) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !sameLine(a[i], b[i]) {
			return false
		}
	}
	return true
}

// CleanCanals drops repeated geometry. The source does not draw segments in flow direction.
func CleanCanals(features []Feature, rec *Recorder) []Canal {
	var canals []Canal
	for i, f := range features {
		code := normalize.Text(f.Properties["id_ruas"])
		rowRef := fmt.Sprintf("fitur %d (%s)", i+1, code)
		parts, fromUTM, swapped, ok := parseLines(f)
		if !ok {
			rec.Add(FileCanals, rowRef, categoryUnreadableGeom, "Geometri kosong atau tidak terbaca", "Ruas dibuang")
			continue
		}
		rec.coordinate(FileCanals, rowRef, fromUTM, swapped)

		canal := Canal{SegmentCode: code, CanalName: normalize.Text(f.Properties["nama_saluran"]), Parts: parts}
		for _, p := range parts {
			utm := toUTM(p)
			canal.UTM = append(canal.UTM, utm)
			canal.LengthM += geo.Length(utm)
		}
		canal.Level, _ = normalize.CanalLevel(normalize.Text(f.Properties["tingkat"]))
		canal.Condition = rec.condition(FileCanals, rowRef, normalize.Text(f.Properties["kondisi"]))

		duplicate := false
		for _, kept := range canals {
			if sameGeometry(kept.UTM, canal.UTM) {
				rec.Add(FileCanals, rowRef, CategoryDuplicateSegment,
					"Geometri sama dengan ruas "+kept.SegmentCode, "Ruas dibuang, satu disimpan")
				duplicate = true
				break
			}
		}
		if !duplicate {
			canals = append(canals, canal)
		}
	}
	return canals
}
