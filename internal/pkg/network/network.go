package network

import (
	"math"
	"sort"

	"irrigation-server/internal/pkg/geo"
)

// The canal network as a directed graph flowing downstream from the weir.

type edge struct {
	line     []geo.Point
	length   float64
	a, b     int // nodes at the drawn start and end
	from, to int // flow direction; -1 when unreachable from the weir
}

type Network struct {
	edges             []edge
	parent            []int // edge flowing into the node; -1 for the root
	outgoing          []int // number of edges flowing out of the node
	root              int
	connectTolerance  float64
	junctionTolerance float64
}

// Position is on a node when Node >= 0, otherwise Offset metres from the upstream end of an edge.
type Position struct {
	Node   int
	Edge   int
	Offset float64
}

// New snaps a structure within junctionTolerance onto the junction, so it keeps the plots of every branch.
func New(lines [][]geo.Point, connectTolerance, junctionTolerance float64) *Network {
	n := &Network{root: -1, connectTolerance: connectTolerance, junctionTolerance: junctionTolerance}
	var usable [][]geo.Point
	for _, line := range lines {
		if len(line) >= 2 && geo.Length(line) > 0 {
			usable = append(usable, line)
		}
	}
	// A segment end touching the middle of another splits it, so every junction is a node.
	for i, line := range usable {
		var cuts []float64
		length := geo.Length(line)
		for k, other := range usable {
			if k == i {
				continue
			}
			for _, end := range []geo.Point{other[0], other[len(other)-1]} {
				d, offset := geo.Project(end, line)
				if d <= connectTolerance && offset > connectTolerance && offset < length-connectTolerance {
					cuts = append(cuts, offset)
				}
			}
		}
		for _, piece := range geo.Split(line, mergeClose(cuts, connectTolerance)) {
			n.addEdge(piece)
		}
	}
	n.connect()
	return n
}

func mergeClose(offsets []float64, tolerance float64) []float64 {
	sort.Float64s(offsets)
	var out []float64
	for _, o := range offsets {
		if len(out) == 0 || o-out[len(out)-1] > tolerance {
			out = append(out, o)
		}
	}
	return out
}

func (n *Network) addEdge(line []geo.Point) {
	if length := geo.Length(line); length > 0 {
		n.edges = append(n.edges, edge{line: line, length: length, from: -1, to: -1})
	}
}

func (n *Network) connect() {
	count := 2 * len(n.edges)
	cluster := make([]int, count)
	for i := range cluster {
		cluster[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if cluster[i] != i {
			cluster[i] = find(cluster[i])
		}
		return cluster[i]
	}
	end := func(i int) geo.Point {
		line := n.edges[i/2].line
		if i%2 == 0 {
			return line[0]
		}
		return line[len(line)-1]
	}
	for p := 0; p < count; p++ {
		for q := p + 1; q < count; q++ {
			if geo.Distance(end(p), end(q)) <= n.connectTolerance {
				cluster[find(p)] = find(q)
			}
		}
	}
	ids := map[int]int{}
	nodeOf := func(i int) int {
		root := find(i)
		if _, ok := ids[root]; !ok {
			ids[root] = len(ids)
		}
		return ids[root]
	}
	for i := range n.edges {
		n.edges[i].a, n.edges[i].b = nodeOf(2*i), nodeOf(2*i+1)
	}
	n.parent = make([]int, len(ids))
	n.outgoing = make([]int, len(ids))
}

func (n *Network) nearest(p geo.Point) (index int, distance, offset float64) {
	index, distance = -1, math.Inf(1)
	for i, e := range n.edges {
		if d, o := geo.Project(p, e.line); d < distance {
			index, distance, offset = i, d, o
		}
	}
	return
}

// SetRoot directs every edge away from the weir. The drawn direction is never used.
func (n *Network) SetRoot(p geo.Point) bool {
	i, _, offset := n.nearest(p)
	if i < 0 {
		return false
	}
	e := n.edges[i]
	switch {
	case offset <= n.junctionTolerance && offset <= e.length-offset:
		n.root = e.a
	case e.length-offset <= n.junctionTolerance:
		n.root = e.b
	default: // the weir sits mid-segment: split it so the root is a node
		pieces := geo.Split(e.line, []float64{offset})
		n.edges[i] = edge{line: pieces[0], length: geo.Length(pieces[0]), from: -1, to: -1}
		n.addEdge(pieces[1])
		n.connect()
		n.root = n.edges[i].b
	}
	n.orient()
	return true
}

func (n *Network) orient() {
	distance := make([]float64, len(n.parent))
	for i := range distance {
		distance[i] = math.Inf(1)
		n.parent[i] = -1
		n.outgoing[i] = 0
	}
	distance[n.root] = 0
	for changed := true; changed; {
		changed = false
		for i, e := range n.edges {
			if distance[e.a]+e.length < distance[e.b] {
				distance[e.b], n.parent[e.b], changed = distance[e.a]+e.length, i, true
			}
			if distance[e.b]+e.length < distance[e.a] {
				distance[e.a], n.parent[e.a], changed = distance[e.b]+e.length, i, true
			}
		}
	}
	for i := range n.edges {
		e := &n.edges[i]
		e.from, e.to = -1, -1
		if math.IsInf(distance[e.a], 1) && math.IsInf(distance[e.b], 1) {
			continue
		}
		if n.parent[e.a] == i || (n.parent[e.b] != i && distance[e.b] < distance[e.a]) {
			e.from, e.to = e.b, e.a
		} else {
			e.from, e.to = e.a, e.b
		}
		n.outgoing[e.from]++
	}
}

// Locate fails when p is beyond maxDistance or its canal is not connected to the root.
func (n *Network) Locate(p geo.Point, maxDistance float64) (Position, bool) {
	i, d, drawnOffset := n.nearest(p)
	if i < 0 || d > maxDistance || n.root < 0 || n.edges[i].from < 0 {
		return Position{}, false
	}
	e := n.edges[i]
	offset := drawnOffset
	if e.from == e.b {
		offset = e.length - drawnOffset
	}
	nearUpstream := offset <= n.junctionTolerance && (e.from == n.root || n.outgoing[e.from] >= 2)
	nearDownstream := e.length-offset <= n.junctionTolerance && n.outgoing[e.to] >= 2
	switch {
	case nearUpstream && (!nearDownstream || offset <= e.length-offset):
		return Position{Node: e.from, Edge: -1}, true
	case nearDownstream:
		return Position{Node: e.to, Edge: -1}, true
	}
	return Position{Node: -1, Edge: i, Offset: offset}, true
}

func (n *Network) inSubtree(node, ancestor int) bool {
	for steps := 0; node >= 0 && steps <= len(n.edges); steps++ {
		if node == ancestor {
			return true
		}
		e := n.parent[node]
		if e < 0 {
			return false
		}
		node = n.edges[e].from
	}
	return false
}

// Downstream is also true when s and x are at the same place.
func (n *Network) Downstream(x, s Position) bool {
	// The most upstream node that must be passed to reach s.
	entry := s.Node
	if entry < 0 {
		entry = n.edges[s.Edge].from
	}
	if x.Node >= 0 {
		return n.inSubtree(entry, x.Node)
	}
	if s.Node < 0 && s.Edge == x.Edge {
		return s.Offset >= x.Offset-1e-6
	}
	return n.inSubtree(entry, n.edges[x.Edge].to)
}
