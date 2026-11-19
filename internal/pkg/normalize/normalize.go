// Package normalize turns free-form source text (nomenclature, condition,
// asset kind, dates, numbers) into the canonical values used by the API contract.
package normalize

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Canonical condition values required by the API contract.
const (
	ConditionGood     = "baik"
	ConditionLight    = "rusak_ringan"
	ConditionModerate = "rusak_sedang"
	ConditionHeavy    = "rusak_berat"
)

// Canonical asset kinds required by the API contract.
const (
	KindWeir            = "bendung"
	KindDivision        = "bangunan_bagi"
	KindOfftake         = "bangunan_sadap"
	KindDivisionOfftake = "bangunan_bagi_sadap"
	KindAuxiliary       = "bangunan_pelengkap"
)

// Canonical report statuses.
const (
	StatusActive    = "aktif"
	StatusCancelled = "batal"
)

var (
	nomenclaturePattern = regexp.MustCompile(`^B([A-Z]+)(\d+)([A-Z]*)$`)
	nonAlphanumeric     = regexp.MustCompile(`[^A-Z0-9]+`)
	whitespace          = regexp.MustCompile(`\s+`)
)

// Clean trims the value and collapses repeated whitespace.
func Clean(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	return whitespace.ReplaceAllString(strings.TrimSpace(s), " ")
}

// Nomenclature rewrites "BKW 1", "b-kw-1" or "b_kw_6a" as "B.KW.1" / "B.KW.6A".
// An empty value stays empty.
func Nomenclature(s string) string {
	upper := strings.ToUpper(Clean(s))
	if upper == "" {
		return ""
	}
	compact := nonAlphanumeric.ReplaceAllString(upper, "")
	if m := nomenclaturePattern.FindStringSubmatch(compact); m != nil {
		number := strings.TrimLeft(m[2], "0")
		if number == "" {
			number = "0"
		}
		return "B." + m[1] + "." + number + m[3]
	}
	// Unknown shape: keep it comparable by upper-casing and dot-separating.
	return strings.Trim(nonAlphanumeric.ReplaceAllString(upper, "."), ".")
}

// lettersOnly lower-cases the value and drops everything that is not a letter.
func lettersOnly(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var conditions = map[string]string{
	"baik": ConditionGood, "b": ConditionGood,
	"rusakringan": ConditionLight, "rr": ConditionLight,
	"rusaksedang": ConditionModerate, "rs": ConditionModerate,
	"rusakberat": ConditionHeavy, "rb": ConditionHeavy,
}

// Condition maps variants such as "RR", "Rusak Ringan" or "rusak ringan " to
// the canonical value.
func Condition(s string) (string, bool) {
	c, ok := conditions[lettersOnly(s)]
	return c, ok
}

// ConditionWeight is the priority-score weight of a condition.
func ConditionWeight(condition string) float64 {
	switch condition {
	case ConditionLight:
		return 1
	case ConditionModerate:
		return 2
	case ConditionHeavy:
		return 4
	}
	return 0
}

var assetKinds = map[string]string{
	"bendung":           KindWeir,
	"bangunanbagi":      KindDivision,
	"bagi":              KindDivision,
	"bangunansadap":     KindOfftake,
	"sadap":             KindOfftake,
	"bangunanbagisadap": KindDivisionOfftake,
	"bagisadap":         KindDivisionOfftake,
	"bangunanpelengkap": KindAuxiliary,
	"pelengkap":         KindAuxiliary,
	"goronggorong":      KindAuxiliary,
	"talang":            KindAuxiliary,
	"siphon":            KindAuxiliary,
	"sipon":             KindAuxiliary,
	"sifon":             KindAuxiliary,
	"terjunan":          KindAuxiliary,
}

// AssetKind maps a structure type to one of the five contract values.
func AssetKind(s string) (string, bool) {
	k, ok := assetKinds[lettersOnly(s)]
	return k, ok
}

// CanalLevel maps a canal level to "primer" or "sekunder".
func CanalLevel(s string) (string, bool) {
	switch level := lettersOnly(s); level {
	case "primer", "sekunder":
		return level, true
	default:
		return strings.ToLower(Clean(s)), false
	}
}

// ReportStatus maps a report status; anything that is not a cancellation is active.
func ReportStatus(s string) string {
	switch lettersOnly(s) {
	case "batal", "dibatalkan":
		return StatusCancelled
	}
	return StatusActive
}

var months = map[string]int{
	"januari": 1, "jan": 1, "january": 1,
	"februari": 2, "feb": 2, "pebruari": 2, "february": 2,
	"maret": 3, "mar": 3, "march": 3,
	"april": 4, "apr": 4,
	"mei": 5, "may": 5,
	"juni": 6, "jun": 6, "june": 6,
	"juli": 7, "jul": 7, "july": 7,
	"agustus": 8, "agu": 8, "ags": 8, "agt": 8, "aug": 8, "august": 8,
	"september": 9, "sep": 9, "sept": 9,
	"oktober": 10, "okt": 10, "oct": 10, "october": 10,
	"november": 11, "nov": 11, "nopember": 11,
	"desember": 12, "des": 12, "dec": 12, "december": 12,
}

var (
	dateSeparator = regexp.MustCompile(`[\s/\-.,]+`)
	dateWithTime  = regexp.MustCompile(`^(\d{4}-\d{1,2}-\d{1,2})[T ]\d{1,2}:.*$`)
	excelEpoch    = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
)

func isoDate(year, month, day int) (string, bool) {
	if year < 100 {
		year += 2000
	}
	if month < 1 || month > 12 || day < 1 || day > 31 || year < 1800 || year > 2200 {
		return "", false
	}
	d := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if d.Day() != day { // e.g. 31 February
		return "", false
	}
	return d.Format(time.DateOnly), true
}

// Date parses the date formats found in the source files and returns
// YYYY-MM-DD. Ambiguous numeric dates are read day first. A bare number is
// read as an Excel serial date, which is how raw date cells arrive.
func Date(s string) (string, bool) {
	s = Clean(s)
	if s == "" {
		return "", false
	}
	if m := dateWithTime.FindStringSubmatch(s); m != nil {
		s = m[1]
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		if len(s) == 8 && !strings.ContainsAny(s, ".eE") { // YYYYMMDD
			year, _ := strconv.Atoi(s[:4])
			month, _ := strconv.Atoi(s[4:6])
			day, _ := strconv.Atoi(s[6:])
			return isoDate(year, month, day)
		}
		if f >= 1 && f < 200000 {
			return excelEpoch.AddDate(0, 0, int(math.Floor(f))).Format(time.DateOnly), true
		}
		return "", false
	}
	parts := dateSeparator.Split(s, -1)
	if len(parts) != 3 {
		return "", false
	}
	numbers := make([]int, 3)
	monthAt := -1
	for i, p := range parts {
		if n, err := strconv.Atoi(p); err == nil {
			numbers[i] = n
			continue
		}
		m, ok := months[strings.ToLower(p)]
		if !ok || monthAt >= 0 {
			return "", false
		}
		monthAt, numbers[i] = i, m
	}
	switch {
	case monthAt == 1 && len(parts[0]) == 4: // 2026 Juli 1
		return isoDate(numbers[0], numbers[1], numbers[2])
	case monthAt == 1: // 1 Juli 2026
		return isoDate(numbers[2], numbers[1], numbers[0])
	case monthAt == 0: // Juli 1, 2026
		return isoDate(numbers[2], numbers[0], numbers[1])
	case monthAt == 2:
		return "", false
	case len(parts[0]) == 4: // 2026-07-01
		return isoDate(numbers[0], numbers[1], numbers[2])
	}
	day, month := numbers[0], numbers[1]
	if month > 12 && day <= 12 { // not ambiguous: clearly month first
		day, month = month, day
	}
	return isoDate(numbers[2], month, day)
}

// Number parses a number written with either a decimal point or a decimal comma.
func Number(s string) (float64, bool) {
	s = strings.ReplaceAll(Clean(s), " ", "")
	if s == "" {
		return 0, false
	}
	commas, dots := strings.Count(s, ","), strings.Count(s, ".")
	switch {
	case commas > 0 && dots > 0: // the separator that appears last is the decimal one
		if strings.LastIndex(s, ",") > strings.LastIndex(s, ".") {
			s = strings.ReplaceAll(s, ".", "")
			s = strings.Replace(s, ",", ".", 1)
		} else {
			s = strings.ReplaceAll(s, ",", "")
		}
	case commas == 1:
		s = strings.Replace(s, ",", ".", 1)
	case commas > 1:
		s = strings.ReplaceAll(s, ",", "")
	case dots > 1:
		s = strings.ReplaceAll(s, ".", "")
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// NumberFromAny parses a number from a decoded JSON value (number or text).
func NumberFromAny(v any) (float64, bool) {
	switch x := v.(type) {
	case nil:
		return 0, false
	case float64:
		return x, !math.IsNaN(x)
	case string:
		return Number(x)
	default:
		return Number(fmt.Sprint(x))
	}
}

// Text converts a decoded JSON value to cleaned text.
func Text(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return Clean(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return Clean(fmt.Sprint(x))
	}
}
