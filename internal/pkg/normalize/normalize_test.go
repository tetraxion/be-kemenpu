package normalize

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNomenclature(t *testing.T) {
	cases := map[string]string{
		"B.KW.1":   "B.KW.1",
		"BKW 1":    "B.KW.1",
		"bkw1":     "B.KW.1",
		" B-KW-2":  "B.KW.2",
		"b_wg_4 ":  "B.WG.4",
		"B.KW.6a":  "B.KW.6A",
		"b ka 3b":  "B.KA.3B",
		"BKA1A":    "B.KA.1A",
		"bbu 2a":   "B.BU.2A",
		"B KW 0":   "B.KW.0",
		"B.SR.17c": "B.SR.17C",
		"B.KW.01":  "B.KW.1",
		"":         "",
		"   ":      "",
	}
	for in, want := range cases {
		assert.Equal(t, want, Nomenclature(in), "input %q", in)
	}
}

func TestCondition(t *testing.T) {
	cases := map[string]string{
		"RR": ConditionLight, "Rusak Ringan": ConditionLight, "rusak ringan ": ConditionLight,
		"Rusak-Berat": ConditionHeavy, "rusak  berat ": ConditionHeavy, "RB": ConditionHeavy,
		"RUSAK_SEDANG": ConditionModerate, "rs": ConditionModerate, " RS ": ConditionModerate,
		"b": ConditionGood, "BAIK ": ConditionGood, "Baik": ConditionGood,
	}
	for in, want := range cases {
		got, ok := Condition(in)
		assert.True(t, ok, "input %q", in)
		assert.Equal(t, want, got, "input %q", in)
	}
	_, ok := Condition("hancur")
	assert.False(t, ok, "an unknown condition must be rejected")
}

func TestConditionWeight(t *testing.T) {
	assert.Equal(t, 0.0, ConditionWeight(ConditionGood))
	assert.Equal(t, 1.0, ConditionWeight(ConditionLight))
	assert.Equal(t, 2.0, ConditionWeight(ConditionModerate))
	assert.Equal(t, 4.0, ConditionWeight(ConditionHeavy))
	assert.Equal(t, 0.0, ConditionWeight(""))
}

func TestAssetKind(t *testing.T) {
	cases := map[string]string{
		"Bangunan Bagi-Sadap": KindDivisionOfftake,
		"BANGUNAN SADAP":      KindOfftake,
		"Bangunan Bagi":       KindDivision,
		"Bendung":             KindWeir,
		"GORONG-GORONG":       KindAuxiliary,
		"siphon":              KindAuxiliary,
		"Talang":              KindAuxiliary,
		"Terjunan":            KindAuxiliary,
	}
	for in, want := range cases {
		got, ok := AssetKind(in)
		assert.True(t, ok, "input %q", in)
		assert.Equal(t, want, got, "input %q", in)
	}
	_, ok := AssetKind("menara")
	assert.False(t, ok)
}

func TestDate(t *testing.T) {
	cases := map[string]string{
		"2024-05-28":             "2024-05-28",
		"31-12-2025":             "2025-12-31",
		"14/06/2025":             "2025-06-14",
		"2025/03/12":             "2025-03-12",
		"6/1/2026":               "2026-01-06", // ambiguous: day first
		"08-03-2026":             "2026-03-08",
		"1 Juli 2026":            "2026-07-01",
		"10 November 2025":       "2025-11-10",
		"22 Agustus 2026":        "2026-08-22",
		"4 Februari 2025":        "2025-02-04",
		"03/25/2026":             "2026-03-25", // not ambiguous: month first
		"46246":                  "2026-08-12", // Excel serial date
		"2026-08-12 00:00:00":    "2026-08-12",
		"2026-08-12T00:00:00":    "2026-08-12",
		"14/08/2026 10:30":       "2026-08-14",
		"14-08-2026T10:00:00":    "2026-08-14",
		"Jumat, 14 Agustus 2026": "2026-08-14",
		"20260814":               "2026-08-14",
		"14082026":               "2026-08-14",
	}
	for in, want := range cases {
		got, ok := Date(in)
		assert.True(t, ok, "input %q", in)
		assert.Equal(t, want, got, "input %q", in)
	}
	for _, bad := range []string{"", "31/02/2026", "besok", "12/2026", "2026", "7"} {
		_, ok := Date(bad)
		assert.False(t, ok, "input %q must fail", bad)
	}
}

func TestNumber(t *testing.T) {
	cases := map[string]float64{
		"110,402958": 110.402958,
		"-7,687024":  -7.687024,
		"437888.80":  437888.80,
		"38,6":       38.6,
		"412000":     412000,
		"1.234,5":    1234.5,
		"1,234.5":    1234.5,
		" 33 ":       33,
	}
	for in, want := range cases {
		got, ok := Number(in)
		assert.True(t, ok, "input %q", in)
		assert.InDelta(t, want, got, 1e-9, "input %q", in)
	}
	_, ok := Number("")
	assert.False(t, ok)

	fromText, ok := NumberFromAny("38,6")
	assert.True(t, ok)
	assert.InDelta(t, 38.6, fromText, 1e-9)
	_, ok = NumberFromAny(nil)
	assert.False(t, ok)
}

func TestReportStatusAndCanalLevel(t *testing.T) {
	assert.Equal(t, StatusCancelled, ReportStatus("BATAL"))
	assert.Equal(t, StatusActive, ReportStatus("Aktif"))
	assert.Equal(t, StatusActive, ReportStatus(""))

	level, ok := CanalLevel(" Sekunder")
	assert.True(t, ok)
	assert.Equal(t, "sekunder", level)
}

func TestOtherSpellingsFallBackOnKeywords(t *testing.T) {
	conditions := map[string]string{
		"Rusak Berat (RB)": ConditionHeavy,
		"R. Ringan":        ConditionLight,
		"sedang":           ConditionModerate,
		"Kondisi Baik":     ConditionGood,
	}
	for in, want := range conditions {
		got, ok := Condition(in)
		assert.True(t, ok, "input %q", in)
		assert.Equal(t, want, got, "input %q", in)
	}
	for _, bad := range []string{"tidak baik", "kurang baik", "?"} {
		_, ok := Condition(bad)
		assert.False(t, ok, "input %q must stay unknown", bad)
	}

	kinds := map[string]string{
		"Bendung Tetap":               KindWeir,
		"Bangunan Bagi dan Sadap":     KindDivisionOfftake,
		"B. Bagi":                     KindDivision,
		"Bang. Sadap":                 KindOfftake,
		"Syphon":                      KindAuxiliary,
		"Bangunan Terjun":             KindAuxiliary,
		"Bangunan Pelengkap (Talang)": KindAuxiliary,
	}
	for in, want := range kinds {
		got, ok := AssetKind(in)
		assert.True(t, ok, "input %q", in)
		assert.Equal(t, want, got, "input %q", in)
	}

	level, ok := CanalLevel("Saluran Primer")
	assert.True(t, ok)
	assert.Equal(t, "primer", level)
}
