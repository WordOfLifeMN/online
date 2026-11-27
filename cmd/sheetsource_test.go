package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifySheetRef(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name string
		ref  string
		kind sheetKind
		want string
		err  string
	}{
		{"real Google Sheet ID", "1z4XIiEPMFPpeRgGpdhshiQpmY7A45KzCyZzQ7Ohe85E",
			googleSheet, "1z4XIiEPMFPpeRgGpdhshiQpmY7A45KzCyZzQ7Ohe85E", ""},
		{"ID with surrounding space", "  1vvhIGMPvVF-DtWoYsEbVBvzk_VtLyKuIw_zyLdsB-JY ",
			googleSheet, "1vvhIGMPvVF-DtWoYsEbVBvzk_VtLyKuIw_zyLdsB-JY", ""},
		{"real OneDrive path",
			`C:\Users\WordofLifeMNMedia\OneDrive - Word of Life Ministries\Documents\WOLCatalog\MediaCatalog.xlsx`,
			excelWorkbook,
			`C:\Users\WordofLifeMNMedia\OneDrive - Word of Life Ministries\Documents\WOLCatalog\MediaCatalog.xlsx`, ""},
		{"forward slashes", "C:/Catalog/MediaCatalog.xlsx", excelWorkbook, "C:/Catalog/MediaCatalog.xlsx", ""},
		{"home relative", "~/Documents/catalog.xlsx", excelWorkbook, home + "/Documents/catalog.xlsx", ""},
		{"bare file name", "catalog.xlsx", excelWorkbook, "catalog.xlsx", ""},
		{"macro workbook, any case", "Catalog.XLSM", excelWorkbook, "Catalog.XLSM", ""},
		{"path of the wrong type is still a path", `C:\data\catalog.csv`, excelWorkbook, `C:\data\catalog.csv`, ""},
		{"Google URL", "https://docs.google.com/spreadsheets/d/1z4X/edit", 0, "", "is a URL"},
		{"SharePoint URL", "HTTPS://wordoflifeministries.sharepoint.com/x.xlsx", 0, "", "is a URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, got, err := classifySheetRef(tt.ref)
			if tt.err != "" {
				assert.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.kind, kind)
			assert.Equal(t, tt.want, got)
		})
	}
}

// a path is never sent to Google, even when there is no file there
func TestOpenSheetSource_MissingWorkbook(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "MediaCatalog.xlsx")
	_, err := openSheetSource(context.Background(), missing)
	require.Error(t, err)
	assert.Contains(t, err.Error(), missing)
	assert.NotContains(t, err.Error(), "googleapi")
}

func TestOpenSheetSource_NotAWorkbook(t *testing.T) {
	csv := filepath.Join(t.TempDir(), "catalog.csv")
	require.NoError(t, os.WriteFile(csv, []byte("a,b\n"), 0o644))
	_, err := openSheetSource(context.Background(), csv)
	assert.ErrorContains(t, err, "only .xlsx and .xlsm")
}

func TestDescribeSaved(t *testing.T) {
	now := time.Date(2026, 11, 27, 11, 4, 52, 0, time.Local)
	tests := []struct {
		saved time.Time
		want  string
	}{
		{now.Add(-40 * time.Second), "11:04:12, 40 s ago"},
		{now.Add(-3*time.Minute - 10*time.Second), "11:01:42, 3 min ago"},
		{now.Add(-2*time.Hour - 5*time.Minute), "08:59, 2 h ago"},
		{now.Add(-50 * time.Hour), "Nov 25 09:04"},
		{now.Add(2 * time.Second), "11:04:54"}, // clocks disagree: no age
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, describeSaved(tt.saved, now))
	}
}

func TestPrintWorkbookSaved(t *testing.T) {
	now := time.Date(2026, 11, 27, 11, 4, 52, 0, time.Local)
	fileTime := now.Add(-10 * time.Second)

	// the save time recorded in the workbook is used when there is one
	var out bytes.Buffer
	printWorkbookSaved(&out, "MediaCatalog.xlsx", now.Add(-40*time.Second), true, fileTime, now)
	assert.Equal(t, "Reading MediaCatalog.xlsx (includes edits up to 11:04:12, 40 s ago)\n", out.String())

	// the file's time stands in, labelled as such, when there isn't
	out.Reset()
	printWorkbookSaved(&out, "MediaCatalog.xlsx", time.Time{}, false, fileTime, now)
	assert.Equal(t, "Reading MediaCatalog.xlsx (file dated 11:04:42, 10 s ago)\n", out.String())
}
