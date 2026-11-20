package ingest

import (
	"fmt"
	"math"

	"irrigation-server/internal/pkg/geo"
	"irrigation-server/internal/pkg/normalize"
)

func distanceToBoundary(p geo.Point, rings [][]geo.Point) float64 {
	nearest := math.Inf(1)
	for _, ring := range rings {
		nearest = math.Min(nearest, geo.DistanceToLine(p, ring))
	}
	return nearest
}

// Analyze: a plot is affected by X when the water path from the weir to its offtake passes through X.
func Analyze(canals []Canal, assets []*Asset, plots []*Plot, rec *Recorder) {
	byNomenclature := map[string]*Asset{}
	for _, a := range assets {
		if _, exists := byNomenclature[a.Nomenclature]; a.Nomenclature != "" && !exists {
			byNomenclature[a.Nomenclature] = a
		}
	}

	var lines [][]geo.Point
	for _, c := range canals {
		lines = append(lines, c.UTM...)
	}
	network := NewNetwork(lines, ConnectToleranceM, JunctionToleranceM)
	hasRoot := false
	for _, a := range assets {
		if a.Kind == normalize.KindWeir && a.onNetwork() {
			hasRoot = network.SetRoot(a.UTM)
			break
		}
	}
	if !hasRoot {
		rec.Add(FileAssets, "-", categoryNoWeir, "Tidak ada bendung dengan posisi pada jaringan",
			"Arah aliran tidak dapat ditentukan, luas terdampak semua bangunan 0")
	}
	positions := map[*Asset]Position{}
	for _, a := range assets {
		if !a.onNetwork() {
			continue
		}
		if pos, ok := network.Locate(a.UTM, MaxCanalDistanceM); ok {
			positions[a] = pos
		}
	}

	totals := map[*Asset]float64{}
	for _, p := range plots {
		rowRef := fmt.Sprintf("fitur %d (%s)", p.index+1, p.Code)
		offtake := byNomenclature[p.OfftakeNomenclature]
		if offtake == nil && len(p.rings) > 0 {
			nearest := MaxOfftakeMatchM
			for _, a := range assets {
				if (a.Kind != normalize.KindOfftake && a.Kind != normalize.KindDivisionOfftake) || !a.onNetwork() {
					continue
				}
				if d := distanceToBoundary(a.UTM, p.rings); d <= nearest {
					offtake, nearest = a, d
				}
			}
			if offtake != nil {
				rec.Add(FilePlots, rowRef, categoryOfftakeByDistance,
					fmt.Sprintf("nomenklatur_sadap %q kosong atau tidak dikenal", p.OfftakeNomenclature),
					fmt.Sprintf("Dilayani bangunan terdekat %s (%.0f m dari batas petak)", offtake.Nomenclature, nearest))
			}
		}
		if offtake == nil {
			p.Flags = append(p.Flags, CategoryNoOfftake)
			rec.Add(FilePlots, rowRef, CategoryNoOfftake,
				fmt.Sprintf("nomenklatur_sadap %q kosong atau tidak dikenal, dan tidak ada bangunan sadap dalam 200 m", p.OfftakeNomenclature),
				"Diberi tanda, tidak ikut dihitung")
			continue
		}
		p.OfftakeAssetCode = offtake.Code
		offtakePosition, ok := positions[offtake]
		if !ok {
			continue // the offtake structure has no position on the network
		}
		for _, x := range assets {
			position, located := positions[x]
			if located && (x == offtake || network.Downstream(position, offtakePosition)) {
				x.ImpactedPlots = append(x.ImpactedPlots, p.Code)
				totals[x] += p.AreaHa
			}
		}
	}
	for a, total := range totals {
		a.ImpactedAreaHa = geo.Round(total, 2)
	}
}
