package ingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"irrigation-server/internal/pkg/geo"
)

// The example scheme of the assignment, on a local metre grid:
//
//	Weir(0,0) - B.KW.1(1000,0) - B.KW.2(2000,0) - B.KW.3(3000,0)        primary
//	                                 |
//	                              B.SR.1(2000,-1000) - B.SR.2(2000,-2000)   secondary
//
// Segments are deliberately drawn in mixed directions with gaps of a few metres.
func exampleLines() [][]geo.Point {
	return [][]geo.Point{
		{{X: 1000, Y: 0}, {X: 0, Y: 0}},            // drawn against the flow
		{{X: 1003, Y: 2}, {X: 2000, Y: 0}},         // gap of about 3.6 m
		{{X: 3000, Y: 0}, {X: 2004, Y: -3}},        // against the flow, gap of 5 m
		{{X: 1998, Y: -4}, {X: 2000, Y: -1000}},    // secondary branch
		{{X: 2000, Y: -2000}, {X: 2001, Y: -1006}}, // against the flow
	}
}

func examplePoints() map[string]geo.Point {
	return map[string]geo.Point{
		"weir":     {X: 2, Y: 3},
		"B.KW.1":   {X: 1001, Y: 6},
		"B.KW.2":   {X: 2007, Y: 1}, // surveyed 7 m off, towards the downstream primary branch
		"B.KW.3":   {X: 2995, Y: -4},
		"B.SR.1":   {X: 2003, Y: -1002},
		"B.SR.2":   {X: 1999, Y: -1996},
		"aqueduct": {X: 1500, Y: 5}, // mid-segment between B.KW.1 and B.KW.2
	}
}

func TestDownstreamMatchesTheAssignmentExample(t *testing.T) {
	network := NewNetwork(exampleLines(), 10, 30)
	points := examplePoints()
	require.True(t, network.SetRoot(points["weir"]))

	positions := map[string]Position{}
	for name, p := range points {
		pos, ok := network.Locate(p, 50)
		require.True(t, ok, "%s must get a position", name)
		positions[name] = pos
	}

	// want[x] lists the offtakes downstream of x (or x itself).
	offtakes := []string{"B.KW.1", "B.KW.2", "B.KW.3", "B.SR.1", "B.SR.2"}
	want := map[string][]string{
		"weir":     offtakes,
		"B.KW.1":   offtakes,
		"aqueduct": {"B.KW.2", "B.KW.3", "B.SR.1", "B.SR.2"},
		"B.KW.2":   {"B.KW.2", "B.KW.3", "B.SR.1", "B.SR.2"},
		"B.KW.3":   {"B.KW.3"},
		"B.SR.1":   {"B.SR.1", "B.SR.2"},
		"B.SR.2":   {"B.SR.2"},
	}
	for x, expected := range want {
		var got []string
		for _, s := range offtakes {
			if s == x || network.Downstream(positions[x], positions[s]) {
				got = append(got, s)
			}
		}
		assert.Equal(t, expected, got, "structures downstream of %s", x)
	}
}

func TestLocateRejectsStructuresOffTheNetwork(t *testing.T) {
	network := NewNetwork(exampleLines(), 10, 30)
	points := examplePoints()

	_, ok := network.Locate(points["B.KW.1"], 50)
	assert.False(t, ok, "without a root there is no flow direction")

	network.SetRoot(points["weir"])
	_, ok = network.Locate(geo.Point{X: 1500, Y: 80}, 50)
	assert.False(t, ok, "80 m from the canal is beyond the 50 m limit")
	_, ok = network.Locate(geo.Point{X: 1500, Y: 50}, 50)
	assert.True(t, ok, "exactly 50 m is still on the network")
}

func TestDisconnectedSegmentIsUnreachable(t *testing.T) {
	lines := [][]geo.Point{
		{{X: 0, Y: 0}, {X: 1000, Y: 0}},
		{{X: 1020, Y: 0}, {X: 2000, Y: 0}}, // gap of 20 m: not connected
	}
	network := NewNetwork(lines, 10, 30)
	network.SetRoot(geo.Point{X: 0, Y: 0})

	_, ok := network.Locate(geo.Point{X: 1500, Y: 0}, 50)
	assert.False(t, ok, "a segment behind a 20 m gap is not fed by the weir")
	_, ok = network.Locate(geo.Point{X: 500, Y: 0}, 50)
	assert.True(t, ok)
}

func TestGapAtTheConnectToleranceStillConnects(t *testing.T) {
	lines := [][]geo.Point{
		{{X: 0, Y: 0}, {X: 1000, Y: 0}},
		{{X: 1010, Y: 0}, {X: 2000, Y: 0}}, // exactly 10 m
	}
	network := NewNetwork(lines, 10, 30)
	network.SetRoot(geo.Point{X: 0, Y: 0})

	_, ok := network.Locate(geo.Point{X: 1500, Y: 0}, 50)
	assert.True(t, ok)
}

func TestBranchFromTheMiddleOfASegment(t *testing.T) {
	// The secondary touches the middle of the primary, not one of its ends.
	lines := [][]geo.Point{
		{{X: 0, Y: 0}, {X: 2000, Y: 0}},
		{{X: 1000, Y: -2}, {X: 1000, Y: -1000}},
	}
	network := NewNetwork(lines, 10, 30)
	network.SetRoot(geo.Point{X: 0, Y: 0})

	upstream, _ := network.Locate(geo.Point{X: 500, Y: 0}, 50)
	primaryTail, _ := network.Locate(geo.Point{X: 1500, Y: 0}, 50)
	secondary, _ := network.Locate(geo.Point{X: 1000, Y: -500}, 50)

	assert.True(t, network.Downstream(upstream, secondary))
	assert.True(t, network.Downstream(upstream, primaryTail))
	assert.False(t, network.Downstream(primaryTail, secondary), "sibling branches do not affect each other")
	assert.False(t, network.Downstream(secondary, primaryTail))
}

func TestWeirInTheMiddleOfASegmentFeedsBothSides(t *testing.T) {
	network := NewNetwork([][]geo.Point{{{X: 0, Y: 0}, {X: 2000, Y: 0}}}, 10, 30)
	require.True(t, network.SetRoot(geo.Point{X: 1000, Y: 0}))

	weir, _ := network.Locate(geo.Point{X: 1000, Y: 0}, 50)
	left, okLeft := network.Locate(geo.Point{X: 200, Y: 0}, 50)
	right, okRight := network.Locate(geo.Point{X: 1800, Y: 0}, 50)
	require.True(t, okLeft)
	require.True(t, okRight)

	assert.True(t, network.Downstream(weir, left))
	assert.True(t, network.Downstream(weir, right))
	assert.False(t, network.Downstream(left, right))
}
