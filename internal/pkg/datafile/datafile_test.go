package datafile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func write(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestReadCSVDetectsTheSeparatorAndSkipsBlankLines(t *testing.T) {
	path := write(t, "aset.csv", byteOrderMark+"Kode_Aset; Kondisi \n\nKW-0001;RB\n;;\nKW-0002;baik\n")

	rows, err := ReadCSV(path)

	require.NoError(t, err)
	assert.Equal(t, []Row{
		{"kode_aset": "KW-0001", "kondisi": "RB"},
		{"kode_aset": "KW-0002", "kondisi": "baik"},
	}, rows)
}

func TestReadCSVReportsAMissingFile(t *testing.T) {
	_, err := ReadCSV(filepath.Join(t.TempDir(), "tidak-ada.csv"))
	assert.Error(t, err)
}

func TestReadGeoJSONLowersPropertyNames(t *testing.T) {
	path := write(t, "saluran.geojson", `{"type":"FeatureCollection","features":[
		{"type":"Feature","properties":{"ID_Ruas":"R1"},"geometry":{"type":"LineString","coordinates":[[0,0],[1,1]]}},
		{"type":"Feature","properties":{"id_ruas":"R2"},"geometry":null}]}`)

	features, err := ReadGeoJSON(path)

	require.NoError(t, err)
	require.Len(t, features, 2)
	assert.Equal(t, "R1", features[0].Properties["id_ruas"])
	assert.Equal(t, "LineString", features[0].GeometryType)
	assert.Empty(t, features[1].GeometryType, "a feature without geometry is kept")
}

func TestReadXLSXRejectsAFileThatIsNotAWorkbook(t *testing.T) {
	_, err := ReadXLSX(write(t, "laporan.xlsx", "bukan workbook"))
	assert.Error(t, err)
}
