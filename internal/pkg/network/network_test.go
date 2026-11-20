package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"irrigation-server/internal/pkg/geo"
	"irrigation-server/internal/pkg/network/networktest"
)

func TestDownstreamMatchesTheAssignmentExample(t *testing.T) {
	network := New(networktest.ExampleLines(), 10, 30)
	points := networktest.ExamplePoints()
	require.True(t, network.SetRoot(points["weir"]))

	positions := map[string]Position{}
	for name, p := range points {
		pos, ok := network.Locate(p, 50)
		require.True(t, ok, "%s must get a position", name)
		positions[name] = pos
	}

	// want[x]: offtakes at or downstream of x
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
	network := New(networktest.ExampleLines(), 10, 30)
	points := networktest.ExamplePoints()

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
	network := New(lines, 10, 30)
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
	network := New(lines, 10, 30)
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
	network := New(lines, 10, 30)
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
	network := New([][]geo.Point{{{X: 0, Y: 0}, {X: 2000, Y: 0}}}, 10, 30)
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
