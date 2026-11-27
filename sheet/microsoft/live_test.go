package microsoft

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestMatchWorkbook(t *testing.T) {
	local := `C:\Users\someone\OneDrive - Word of Life Ministries\Documents\WOLCatalog\MediaCatalog.xlsx`
	tests := []struct {
		name     string
		path     string
		wbName   string
		fullName string
		want     bool
	}{
		{"SharePoint copy of the synced file", local, "MediaCatalog.xlsx",
			"https://wordoflifeministries.sharepoint.com/sites/VideoDownloads/Shared Documents/WOLCatalog/MediaCatalog.xlsx", true},
		{"URL with escaped spaces", `C:\x\WOL Catalog\MediaCatalog.xlsx`, "MediaCatalog.xlsx",
			"https://example.sharepoint.com/Shared%20Documents/WOL%20Catalog/MediaCatalog.xlsx", true},
		{"case differs", local, "mediacatalog.XLSX",
			"https://example.sharepoint.com/sites/x/wolcatalog/MEDIACATALOG.xlsx", true},
		{"same name, different folder", `C:\backup\MediaCatalog.xlsx`, "MediaCatalog.xlsx",
			"https://example.sharepoint.com/sites/x/WOLCatalog/MediaCatalog.xlsx", false},
		{"different name", local, "Other.xlsx",
			"https://example.sharepoint.com/sites/x/WOLCatalog/Other.xlsx", false},
		{"opened from the local path itself", local, "MediaCatalog.xlsx", local, true},
		{"opened from another local path", local, "MediaCatalog.xlsx",
			`D:\copies\WOLCatalog\MediaCatalog.xlsx`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, matchWorkbook(tt.path, tt.wbName, tt.fullName))
		})
	}
}

func TestFolderAndName(t *testing.T) {
	assert.Equal(t, "WOLCatalog/MediaCatalog.xlsx", folderAndName(`C:\a\WOLCatalog\MediaCatalog.xlsx`))
	assert.Equal(t, "WOLCatalog/MediaCatalog.xlsx", folderAndName("https://h/s/WOLCatalog/MediaCatalog.xlsx"))
	assert.Equal(t, "MediaCatalog.xlsx", folderAndName("MediaCatalog.xlsx"))
}

func TestLiveWorkbook(t *testing.T) {
	w := &liveWorkbook{
		name: "MediaCatalog.xlsx",
		tabs: []string{"WOL"},
		rows: map[string][][]string{"WOL": {{"Date"}}},
	}
	assert.Equal(t, "MediaCatalog.xlsx", w.Title())
	tabs, err := w.Tabs()
	assert.NoError(t, err)
	assert.Equal(t, []string{"WOL"}, tabs)
	rows, err := w.Rows("WOL")
	assert.NoError(t, err)
	assert.Equal(t, [][]string{{"Date"}}, rows)
	_, err = w.Rows("TBO")
	assert.ErrorContains(t, err, "TBO")
}

// A workbook with the live one's name, somewhere else, is read from its file even
// when Excel has the real one open. This runs against whatever Excel is doing on the
// machine, and holds either way.
func TestOpen_SameNameElsewhereReadsTheFile(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	require.NoError(t, f.SetSheetName("Sheet1", "WOL"))
	require.NoError(t, f.SetSheetRow("WOL", "A1", &[]any{"Date", "Name"}))
	path := filepath.Join(t.TempDir(), "MediaCatalog.xlsx")
	require.NoError(t, f.SaveAs(path))

	src, reading, err := Open(path)
	require.NoError(t, err)
	assert.False(t, reading.Live)
	_, isFile := src.(*Workbook)
	assert.True(t, isFile)
}

func TestOpen_RefusesNonWorkbook(t *testing.T) {
	csv := filepath.Join(t.TempDir(), "catalog.csv")
	require.NoError(t, os.WriteFile(csv, []byte("a,b\n"), 0o644))
	_, _, err := Open(csv)
	assert.ErrorContains(t, err, "only .xlsx and .xlsm")
}

func TestOpen_MissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "Missing.xlsx")
	_, _, err := Open(missing)
	assert.ErrorContains(t, err, missing)
}
