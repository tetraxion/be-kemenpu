package ingest

import (
	"encoding/json"
	"fmt"
	"strings"

	"irrigation-server/internal/pkg/geo"
	"irrigation-server/internal/pkg/normalize"
)

// Helpers shared by the canal, asset, plot and report cleaners.

// condition normalises a condition and records the outcome; "" when unknown.
func (r *Recorder) condition(sourceFile, rowRef, original string) string {
	if strings.TrimSpace(original) == "" {
		return ""
	}
	c, ok := normalize.Condition(original)
	if !ok {
		r.Add(sourceFile, rowRef, categoryUnknownCondition, fmt.Sprintf("kondisi tertulis %q", original), "Kondisi dikosongkan")
		return ""
	}
	r.text(sourceFile, rowRef, categoryCondition, "kondisi", original, c)
	return c
}

// date normalises a date and records the outcome; "" when unreadable.
func (r *Recorder) date(sourceFile, rowRef, column, original string) string {
	if strings.TrimSpace(original) == "" {
		return ""
	}
	d, ok := normalize.Date(original)
	if !ok {
		r.Add(sourceFile, rowRef, categoryInvalidDate, fmt.Sprintf("%s tertulis %q", column, original), "Tanggal dikosongkan")
		return ""
	}
	r.text(sourceFile, rowRef, categoryDate, column, original, d)
	return d
}

// pair reads two coordinate columns (x = longitude/easting, y =
// latitude/northing). When the pair is unusable it returns the reason.
func (r *Recorder) pair(sourceFile, rowRef, x, y string) (geo.Coordinate, string) {
	if strings.TrimSpace(x) == "" || strings.TrimSpace(y) == "" {
		return geo.Coordinate{}, "Koordinat kosong"
	}
	fx, okX := normalize.Number(x)
	fy, okY := normalize.Number(y)
	if !okX || !okY {
		return geo.Coordinate{}, fmt.Sprintf("Koordinat tidak terbaca (%q, %q)", x, y)
	}
	if strings.Contains(x, ",") || strings.Contains(y, ",") {
		r.Add(sourceFile, rowRef, categoryDecimalComma, "Koordinat memakai koma desimal", "Dibaca sebagai angka desimal")
	}
	c := geo.Fix(fx, fy)
	if !c.Valid {
		if fx == 0 && fy == 0 {
			return c, "Koordinat bernilai 0,0"
		}
		return c, fmt.Sprintf("Koordinat di luar rentang (%v, %v)", fx, fy)
	}
	r.coordinate(sourceFile, rowRef, c.FromUTM, c.Swapped)
	return c, ""
}

// fixSequence repairs a raw [x, y] vertex list.
func fixSequence(raw [][]float64, fromUTM, swapped *bool) ([]geo.LonLat, bool) {
	out := make([]geo.LonLat, 0, len(raw))
	for _, v := range raw {
		if len(v) < 2 {
			return nil, false
		}
		c := geo.Fix(v[0], v[1])
		if !c.Valid {
			return nil, false
		}
		*fromUTM = *fromUTM || c.FromUTM
		*swapped = *swapped || c.Swapped
		out = append(out, c.LonLat)
	}
	return out, true
}

func toUTM(points []geo.LonLat) []geo.Point {
	out := make([]geo.Point, len(points))
	for i, p := range points {
		out[i] = geo.ToUTM(p)
	}
	return out
}

func positions(points []geo.LonLat) [][2]float64 {
	out := make([][2]float64, len(points))
	for i, p := range points {
		out[i] = [2]float64{p.Lon, p.Lat}
	}
	return out
}

func geometryJSON(geometryType string, coordinates any) string {
	b, _ := json.Marshal(map[string]any{"type": geometryType, "coordinates": coordinates})
	return string(b)
}
