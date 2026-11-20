package ingest

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/xuri/excelize/v2"
)

// byteOrderMark is the UTF-8 BOM some editors put at the start of a file.
const byteOrderMark = string(rune(0xFEFF))

// Row is one table row: lower-cased column name -> raw value.
type Row map[string]string

func columnKey(s string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(s, byteOrderMark)))
}

// toRows treats the first non-empty line as the header and skips blank lines.
func toRows(table [][]string) []Row {
	var header []string
	var rows []Row
	for _, cells := range table {
		blank := true
		for _, v := range cells {
			if strings.TrimSpace(v) != "" {
				blank = false
				break
			}
		}
		if blank {
			continue
		}
		if header == nil {
			for _, v := range cells {
				header = append(header, columnKey(v))
			}
			continue
		}
		row := Row{}
		for i, v := range cells {
			if i < len(header) && header[i] != "" {
				row[header[i]] = v
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// ReadCSV reads a comma- or semicolon-separated file.
func ReadCSV(path string) ([]Row, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ingest: read %s: %w", path, err)
	}
	content = bytes.TrimPrefix(content, []byte(byteOrderMark))
	r := csv.NewReader(bytes.NewReader(content))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	if header, _, _ := bytes.Cut(content, []byte("\n")); bytes.Count(header, []byte(";")) > bytes.Count(header, []byte(",")) {
		r.Comma = ';'
	}
	table, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("ingest: parse %s: %w", path, err)
	}
	return toRows(table), nil
}

// ReadXLSX reads the first sheet. Cells are read raw, so a date-typed cell
// arrives as its Excel serial number instead of locale-dependent formatted text.
func ReadXLSX(path string) ([]Row, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("ingest: open %s: %w", path, err)
	}
	defer f.Close()
	return workbookRows(f, path)
}

// ReadXLSXFrom reads the first sheet of a workbook that is not on disk, such
// as an uploaded file. name only labels the error.
func ReadXLSXFrom(r io.Reader, name string) ([]Row, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("ingest: open %s: %w", name, err)
	}
	defer f.Close()
	return workbookRows(f, name)
}

func workbookRows(f *excelize.File, path string) ([]Row, error) {
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("ingest: %s has no sheet", path)
	}
	table, err := f.GetRows(sheets[0], excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, fmt.Errorf("ingest: parse %s: %w", path, err)
	}
	return toRows(table), nil
}

// Feature is one GeoJSON feature.
type Feature struct {
	Properties   map[string]any
	GeometryType string
	Coordinates  json.RawMessage
}

// ReadGeoJSON reads a FeatureCollection.
func ReadGeoJSON(path string) ([]Feature, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ingest: read %s: %w", path, err)
	}
	var collection struct {
		Features []struct {
			Properties map[string]any `json:"properties"`
			Geometry   *struct {
				Type        string          `json:"type"`
				Coordinates json.RawMessage `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}
	if err := json.Unmarshal(bytes.TrimPrefix(content, []byte(byteOrderMark)), &collection); err != nil {
		return nil, fmt.Errorf("ingest: parse %s: %w", path, err)
	}
	features := make([]Feature, 0, len(collection.Features))
	for _, f := range collection.Features {
		feature := Feature{Properties: map[string]any{}}
		for k, v := range f.Properties {
			feature.Properties[columnKey(k)] = v
		}
		if f.Geometry != nil {
			feature.GeometryType, feature.Coordinates = f.Geometry.Type, f.Geometry.Coordinates
		}
		features = append(features, feature)
	}
	return features, nil
}
