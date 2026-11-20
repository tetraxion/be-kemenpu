// Package networktest holds the worked example from the assignment for tests.
package networktest

import "irrigation-server/internal/pkg/geo"

// The assignment's example scheme on a local metre grid:
//
//	Weir(0,0) - B.KW.1(1000,0) - B.KW.2(2000,0) - B.KW.3(3000,0)        primary
//	                                 |
//	                              B.SR.1(2000,-1000) - B.SR.2(2000,-2000)   secondary
func ExampleLines() [][]geo.Point {
	return [][]geo.Point{
		{{X: 1000, Y: 0}, {X: 0, Y: 0}},            // drawn against the flow
		{{X: 1003, Y: 2}, {X: 2000, Y: 0}},         // gap of about 3.6 m
		{{X: 3000, Y: 0}, {X: 2004, Y: -3}},        // against the flow, gap of 5 m
		{{X: 1998, Y: -4}, {X: 2000, Y: -1000}},    // secondary branch
		{{X: 2000, Y: -2000}, {X: 2001, Y: -1006}}, // against the flow
	}
}

func ExamplePoints() map[string]geo.Point {
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
