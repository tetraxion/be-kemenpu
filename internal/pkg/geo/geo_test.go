package geo

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUTMRoundTrip(t *testing.T) {
	for _, p := range []LonLat{{110.431368, -7.638081}, {110.3751664, -7.6385514}, {111, -7}, {108.2, -1.5}} {
		back := ToWGS84(ToUTM(p))
		assert.InDelta(t, p.Lon, back.Lon, 1e-9)
		assert.InDelta(t, p.Lat, back.Lat, 1e-9)
	}
}

func TestUTMOnCentralMeridian(t *testing.T) {
	// On the central meridian the easting is exactly the false easting.
	p := ToUTM(LonLat{111, -7})
	assert.InDelta(t, 500000, p.X, 1e-6)
	// On the equator the northing is exactly the false northing.
	p = ToUTM(LonLat{111, 0})
	assert.InDelta(t, 10000000, p.Y, 1e-6)
}

func TestUTMMatchesPROJ(t *testing.T) {
	// Reference values from PostGIS/PROJ: ST_Transform between EPSG:4326 and EPSG:32749.
	p := ToUTM(LonLat{110.4, -7.65})
	assert.InDelta(t, 433824.147, p.X, 0.001)
	assert.InDelta(t, 9154348.741, p.Y, 0.001)

	back := ToWGS84(Point{430002.82, 9155998.98})
	assert.InDelta(t, 110.3653764, back.Lon, 5e-8)
	assert.InDelta(t, -7.6350238, back.Lat, 5e-8)
}

func TestFix(t *testing.T) {
	plain := Fix(110.431368, -7.638081)
	assert.True(t, plain.Valid)
	assert.False(t, plain.FromUTM)
	assert.False(t, plain.Swapped)

	swapped := Fix(-7.640567, 110.385810)
	assert.True(t, swapped.Valid)
	assert.True(t, swapped.Swapped)
	assert.False(t, swapped.FromUTM)
	assert.InDelta(t, 110.385810, swapped.Lon, 1e-9)
	assert.InDelta(t, -7.640567, swapped.Lat, 1e-9)

	utm := Fix(437888.80, 9158071.62)
	assert.True(t, utm.Valid)
	assert.True(t, utm.FromUTM)
	assert.False(t, utm.Swapped)
	assert.InDelta(t, 110.437, utm.Lon, 0.01)
	assert.InDelta(t, -7.617, utm.Lat, 0.01)

	utmSwapped := Fix(9158071.62, 437888.80)
	assert.True(t, utmSwapped.Valid)
	assert.True(t, utmSwapped.FromUTM)
	assert.True(t, utmSwapped.Swapped)
	assert.InDelta(t, utm.Lon, utmSwapped.Lon, 1e-12)

	assert.False(t, Fix(0, 0).Valid, "0,0 is not a location")
	assert.False(t, Fix(math.NaN(), 1).Valid)
	assert.False(t, Fix(5000, 7000).Valid, "metres outside the UTM range")
}

func TestProjectAndSplit(t *testing.T) {
	line := []Point{{0, 0}, {100, 0}, {100, 100}}

	distance, offset := Project(Point{50, 10}, line)
	assert.InDelta(t, 10, distance, 1e-9)
	assert.InDelta(t, 50, offset, 1e-9)

	distance, offset = Project(Point{120, 60}, line)
	assert.InDelta(t, 20, distance, 1e-9)
	assert.InDelta(t, 160, offset, 1e-9)

	assert.InDelta(t, 200, Length(line), 1e-9)

	pieces := Split(line, []float64{30, 150})
	if assert.Len(t, pieces, 3) {
		assert.InDelta(t, 30, Length(pieces[0]), 1e-9)
		assert.InDelta(t, 120, Length(pieces[1]), 1e-9)
		assert.InDelta(t, 50, Length(pieces[2]), 1e-9)
	}
}

func TestRound(t *testing.T) {
	assert.Equal(t, 312.45, Round(312.4549, 2))
	assert.Equal(t, 3.42, Round(3.4151, 2))
}
