package ingest

import (
	"fmt"
	"strings"

	"irrigation-server/internal/pkg/datafile"
	"irrigation-server/internal/pkg/geo"
	"irrigation-server/internal/pkg/normalize"
)

// CleanReports matches by nomenclature, then the nearest structure within 30 m. A repeated id_laporan keeps the last row.
func CleanReports(rows []datafile.Row, sourceFile string, ref Reference, rec *Recorder) []Report {
	position := map[string]int{}
	var reports []Report
	for i, row := range rows {
		id := strings.ToUpper(normalize.Clean(row["id_laporan"]))
		if id == "" {
			rec.Add(sourceFile, fmt.Sprintf("baris %d", i+2), categoryMissingID, "id_laporan kosong", "Baris dibuang")
			continue
		}
		report := Report{
			ID: id, SourceFile: sourceFile,
			Officer:         normalize.Clean(row["petugas"]),
			RawNomenclature: normalize.Clean(row["nomenklatur"]),
			DamageType:      normalize.Clean(row["jenis_kerusakan"]),
			Note:            normalize.Clean(row["keterangan"]),
			Status:          normalize.ReportStatus(row["status"]),
		}
		report.Nomenclature = normalize.Nomenclature(row["nomenklatur"])
		rec.text(sourceFile, id, categoryNomenclature, "nomenklatur", row["nomenklatur"], report.Nomenclature)
		report.Condition = rec.condition(sourceFile, id, row["kondisi"])
		report.ReportedOn = rec.date(sourceFile, id, "tanggal", row["tanggal"])

		var location *geo.Point
		if strings.TrimSpace(row["lat"]) != "" || strings.TrimSpace(row["lon"]) != "" {
			c, reason := rec.pair(sourceFile, id, row["lon"], row["lat"])
			if c.Valid {
				lat, lon := c.Lat, c.Lon
				report.Lat, report.Lon = &lat, &lon
				utm := geo.ToUTM(c.LonLat)
				location = &utm
			} else {
				rec.Add(sourceFile, id, categoryReportCoordIgnored, reason, "Koordinat laporan dikosongkan")
			}
		}

		if code, known := ref.ByNomenclature[report.Nomenclature]; known && report.Nomenclature != "" {
			report.AssetCode = code
		} else if location != nil {
			nearest := MaxReportMatchM
			for _, point := range ref.Points {
				if d := geo.Distance(*location, point.UTM); d <= nearest {
					report.AssetCode, nearest = point.Code, d
				}
			}
			if report.AssetCode != "" {
				rec.Add(sourceFile, id, categoryReportByDistance,
					fmt.Sprintf("nomenklatur %q kosong atau tidak dikenal", report.RawNomenclature),
					fmt.Sprintf("Dicocokkan ke bangunan terdekat %s (%.1f m)", report.AssetCode, nearest))
			}
		}

		if at, repeated := position[id]; repeated {
			rec.Add(sourceFile, id, categoryRepeatedReportID, "id_laporan muncul lebih dari sekali dalam satu berkas", "Baris terakhir dipakai")
			reports[at] = report
		} else {
			position[id] = len(reports)
			reports = append(reports, report)
		}
	}
	return reports
}

// ClassifyFollowUp skips unchanged reports, so a repeated import is a no-op.
func ClassifyFollowUp(incoming []Report, existing map[string]Report) (Summary, []Report) {
	var summary Summary
	var upserts []Report
	for _, report := range incoming {
		before, exists := existing[report.ID]
		switch {
		case !exists:
			summary.Added++
		case before.SameContent(report):
			summary.Unchanged++
			continue
		case report.Status == normalize.StatusCancelled && before.Status != normalize.StatusCancelled:
			summary.Cancelled++
		default:
			summary.Updated++
		}
		upserts = append(upserts, report)
	}
	return summary, upserts
}
