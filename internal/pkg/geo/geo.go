// Package geo converts between WGS84 and UTM zone 49S and measures planar geometry.
package geo

import "math"

// Point is a planar EPSG:32749 coordinate in metres.
type Point struct{ X, Y float64 }

// LonLat is a WGS84 coordinate in degrees.
type LonLat struct{ Lon, Lat float64 }

// WGS84 ellipsoid and UTM zone 49S parameters.
const (
	semiMajorAxis   = 6378137.0
	flattening      = 1 / 298.257223563
	scaleFactor     = 0.9996
	centralMeridian = 111.0
	falseEasting    = 500000.0
	falseNorthing   = 10000000.0
)

// Krüger series coefficients, computed once from the ellipsoid constants.
var (
	n       = flattening / (2 - flattening)
	radiusA = semiMajorAxis / (1 + n) * (1 + n*n/4 + n*n*n*n/64)
	alpha   = [3]float64{n/2 - 2*n*n/3 + 5*n*n*n/16, 13*n*n/48 - 3*n*n*n/5, 61 * n * n * n / 240}
	beta    = [3]float64{n/2 - 2*n*n/3 + 37*n*n*n/96, n*n/48 + n*n*n/15, 17 * n * n * n / 480}
	delta   = [3]float64{2*n - 2*n*n/3 - 2*n*n*n, 7*n*n/3 - 8*n*n*n/5, 56 * n * n * n / 15}
	rootN   = 2 * math.Sqrt(n) / (1 + n)
)

// ToUTM projects WGS84 to EPSG:32749 with the Krüger series.
func ToUTM(p LonLat) Point {
	phi := p.Lat * math.Pi / 180
	lam := (p.Lon - centralMeridian) * math.Pi / 180
	t := math.Sinh(math.Atanh(math.Sin(phi)) - rootN*math.Atanh(rootN*math.Sin(phi)))
	xi := math.Atan2(t, math.Cos(lam))
	eta := math.Atanh(math.Sin(lam) / math.Sqrt(1+t*t))
	x, y := eta, xi
	for j := 1; j <= 3; j++ {
		f := 2 * float64(j)
		x += alpha[j-1] * math.Cos(f*xi) * math.Sinh(f*eta)
		y += alpha[j-1] * math.Sin(f*xi) * math.Cosh(f*eta)
	}
	return Point{falseEasting + scaleFactor*radiusA*x, falseNorthing + scaleFactor*radiusA*y}
}

func ToWGS84(p Point) LonLat {
	xi := (p.Y - falseNorthing) / (scaleFactor * radiusA)
	eta := (p.X - falseEasting) / (scaleFactor * radiusA)
	xi1, eta1 := xi, eta
	for j := 1; j <= 3; j++ {
		f := 2 * float64(j)
		xi1 -= beta[j-1] * math.Sin(f*xi) * math.Cosh(f*eta)
		eta1 -= beta[j-1] * math.Cos(f*xi) * math.Sinh(f*eta)
	}
	chi := math.Asin(math.Sin(xi1) / math.Cosh(eta1))
	phi := chi
	for j := 1; j <= 3; j++ {
		phi += delta[j-1] * math.Sin(2*float64(j)*chi)
	}
	lam := math.Atan2(math.Sinh(eta1), math.Cos(xi1))
	return LonLat{centralMeridian + lam*180/math.Pi, phi * 180 / math.Pi}
}

type Coordinate struct {
	LonLat
	Valid   bool
	FromUTM bool // was UTM 49S, now converted
	Swapped bool // was in the wrong order, now corrected
}

// Fix infers system and order from the ranges: above 180 can only be metres, latitude never exceeds 90.
func Fix(x, y float64) Coordinate {
	var c Coordinate
	if math.IsNaN(x) || math.IsNaN(y) || (x == 0 && y == 0) {
		return c
	}
	ax, ay := math.Abs(x), math.Abs(y)
	if ax > 180 || ay > 180 {
		easting, northing := x, y
		if ax > ay { // south of the equator the northing is always the larger value
			easting, northing = y, x
			c.Swapped = true
		}
		if easting < 100000 || easting > 900000 || northing < 1000000 || northing > falseNorthing {
			return Coordinate{}
		}
		c.LonLat = ToWGS84(Point{easting, northing})
		c.FromUTM = true
	} else {
		c.Lon, c.Lat = x, y
		if ay > 90 && ax <= 90 {
			c.Lon, c.Lat = y, x
			c.Swapped = true
		}
	}
	if math.Abs(c.Lat) > 90 || math.Abs(c.Lon) > 180 {
		return Coordinate{}
	}
	c.Valid = true
	return c
}

func Distance(a, b Point) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

func Length(line []Point) float64 {
	var total float64
	for i := 1; i < len(line); i++ {
		total += Distance(line[i-1], line[i])
	}
	return total
}

// Project returns the distance from p to the line and the offset in metres from the line start.
func Project(p Point, line []Point) (distance, offset float64) {
	distance = math.Inf(1)
	if len(line) == 1 {
		return Distance(p, line[0]), 0
	}
	var travelled float64
	for i := 1; i < len(line); i++ {
		a, b := line[i-1], line[i]
		dx, dy := b.X-a.X, b.Y-a.Y
		segment := math.Hypot(dx, dy)
		u := 0.0
		if segment > 0 {
			u = ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / (segment * segment)
			u = math.Max(0, math.Min(1, u))
		}
		if d := Distance(p, Point{a.X + u*dx, a.Y + u*dy}); d < distance {
			distance, offset = d, travelled+u*segment
		}
		travelled += segment
	}
	return distance, offset
}

func DistanceToLine(p Point, line []Point) float64 {
	d, _ := Project(p, line)
	return d
}

// Split takes ascending offsets in metres from the start.
func Split(line []Point, offsets []float64) [][]Point {
	var pieces [][]Point
	current := []Point{line[0]}
	var travelled float64
	k := 0
	for i := 1; i < len(line); i++ {
		a, b := line[i-1], line[i]
		segment := Distance(a, b)
		for k < len(offsets) && offsets[k] <= travelled+segment {
			u := 0.0
			if segment > 0 {
				u = (offsets[k] - travelled) / segment
			}
			cut := Point{a.X + u*(b.X-a.X), a.Y + u*(b.Y-a.Y)}
			current = append(current, cut)
			pieces = append(pieces, current)
			current = []Point{cut}
			k++
		}
		current = append(current, b)
		travelled += segment
	}
	return append(pieces, current)
}

func Round(v float64, decimals int) float64 {
	p := math.Pow(10, float64(decimals))
	return math.Round(v*p) / p
}
