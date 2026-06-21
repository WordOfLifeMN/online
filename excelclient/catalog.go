package excelclient

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/xuri/excelize/v2"
)

// NewCatalog reads an Excel workbook at excelPath and returns a Catalog.
// Every sheet whose name does not start with "_" is read: a sheet named
// exactly "Series" populates the series list; all other eligible sheets
// populate the message list.
// Unreadable rows are collected and returned as a combined error alongside
// whatever partial data was successfully parsed.
func NewCatalog(excelPath string) (*catalog.Catalog, error) {
	cat := &catalog.Catalog{
		Created: time.Now(),
	}

	f, err := excelize.OpenFile(excelPath)
	if err != nil {
		return cat, err
	}
	defer f.Close()

	log.Printf("Reading catalog from Excel file %s", excelPath)

	var errs []string

	for _, sheetName := range f.GetSheetList() {
		if strings.HasPrefix(sheetName, "_") {
			continue
		}
		if sheetName == "Series" {
			series, sheetErrs := readSeriesFromSheet(f, sheetName)
			cat.Series = series
			errs = append(errs, sheetErrs...)
		} else {
			messages, sheetErrs := readMessagesFromSheet(f, sheetName)
			cat.Messages = append(cat.Messages, messages...)
			errs = append(errs, sheetErrs...)
		}
	}

	if len(errs) > 0 {
		return cat, fmt.Errorf("catalog read errors:\n  %s", strings.Join(errs, "\n  "))
	}
	return cat, nil
}

const (
	seriesName        = "Name"
	seriesID          = "ID"
	seriesDescription = "Description"
	seriesStartDate   = "Date Started"
	seriesEndDate     = "Date Ended"
	seriesBooklets    = "Booklets"
	seriesVisibility  = "Visibility"
	seriesCDJacket    = "CD Jacket"
	seriesDVDJacket   = "DVD Jacket"
	seriesThumbnail   = "Cover Art"
)

var requiredSeriesColumns = []string{
	seriesName, seriesID, seriesDescription,
	seriesStartDate, seriesEndDate,
	seriesVisibility,
	seriesBooklets,
	seriesCDJacket, seriesDVDJacket, seriesThumbnail,
}

func readSeriesFromSheet(f *excelize.File, sheetName string) ([]catalog.CatalogSeri, []string) {
	log.Printf("Reading the Series from tab '%s'\n", sheetName)

	rows, err := f.GetRows(sheetName, excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, []string{fmt.Sprintf("cannot read sheet '%s': %v", sheetName, err)}
	}
	if len(rows) == 0 {
		return nil, nil
	}

	columns := buildColumnIndex(rows[0])
	log.Printf("  Found %d columns\n", len(columns))

	for _, required := range requiredSeriesColumns {
		if _, ok := columns[required]; !ok {
			return nil, []string{fmt.Sprintf("required column '%s' not found in sheet '%s'", required, sheetName)}
		}
	}

	var series []catalog.CatalogSeri
	var errs []string

	for i, row := range rows[1:] {
		seri, err := newCatalogSeriFromRow(columns, row)
		if err != nil {
			errs = append(errs, fmt.Sprintf("sheet '%s' row %d: %v", sheetName, i+2, err))
		}
		series = append(series, seri)
	}

	log.Printf("  Found %d series", len(series))
	return series, errs
}

func newCatalogSeriFromRow(columns map[string]int, row []string) (catalog.CatalogSeri, error) {
	seri := catalog.CatalogSeri{}

	seri.ID = getCellString(row, columns[seriesID])
	seri.Name = getCellString(row, columns[seriesName])
	seri.Description = getCellString(row, columns[seriesDescription])
	seri.Visibility = catalog.NewViewFromString(getCellString(row, columns[seriesVisibility]))
	seri.Thumbnail = getCellString(row, columns[seriesThumbnail])

	var errs []string

	if dStr := getCellString(row, columns[seriesStartDate]); dStr != "" {
		if d, err := parseDateCell(dStr); err == nil {
			seri.StartDate = d
		} else {
			errs = append(errs, fmt.Sprintf("invalid start date '%s' for series '%s'", dStr, seri.Name))
		}
	}

	if dStr := getCellString(row, columns[seriesEndDate]); dStr != "" {
		if d, err := parseDateCell(dStr); err == nil {
			seri.StopDate = d
		} else {
			errs = append(errs, fmt.Sprintf("invalid end date '%s' for series '%s'", dStr, seri.Name))
		}
	}

	// DVD jacket preferred over CD jacket
	seri.Jacket = getCellString(row, columns[seriesDVDJacket])
	if seri.Jacket == "" {
		seri.Jacket = getCellString(row, columns[seriesCDJacket])
	}

	seri.Booklets = catalog.NewResourcesFromString(getCellString(row, columns[seriesBooklets]))

	if len(errs) > 0 {
		return seri, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return seri, nil
}

const (
	msgName        = "Name"
	msgDate        = "Date"
	msgSpeakers    = "Speaker"
	msgMinistry    = "Ministry"
	msgType        = "Type"
	msgVisibility  = "Visibility"
	msgSeries      = "Series Name"
	msgSeriesIndex = "Track"
	msgDescription = "Description"
	msgThumb       = "Thumb"
	msgAudio       = "Audio"
	msgVideo       = "Video"
	msgResources   = "Resources"
)

var requiredMessageColumns = []string{
	msgDate, msgName, msgDescription,
	msgSpeakers,
	msgMinistry, msgType, msgVisibility,
	msgSeries, msgSeriesIndex,
	msgThumb, msgAudio, msgVideo,
	msgResources,
}

func readMessagesFromSheet(f *excelize.File, sheetName string) ([]catalog.CatalogMessage, []string) {
	log.Printf("Reading the Messages from tab '%s'\n", sheetName)

	rows, err := f.GetRows(sheetName, excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, []string{fmt.Sprintf("cannot read sheet '%s': %v", sheetName, err)}
	}
	if len(rows) == 0 {
		return nil, nil
	}

	columns := buildColumnIndex(rows[0])
	log.Printf("  Found %d columns\n", len(columns))

	for _, required := range requiredMessageColumns {
		if _, ok := columns[required]; !ok {
			return nil, []string{fmt.Sprintf("required column '%s' not found in sheet '%s'", required, sheetName)}
		}
	}

	var messages []catalog.CatalogMessage
	var errs []string

	for i, row := range rows[1:] {
		msg, err := newCatalogMessageFromRow(columns, row)
		if err != nil {
			errs = append(errs, fmt.Sprintf("sheet '%s' row %d: %v", sheetName, i+2, err))
		}
		messages = append(messages, msg)
	}

	log.Printf("  Found %d messages", len(messages))
	return messages, errs
}

func newCatalogMessageFromRow(columns map[string]int, row []string) (catalog.CatalogMessage, error) {
	msg := catalog.CatalogMessage{}

	msg.Name = getCellString(row, columns[msgName])
	msg.Description = getCellString(row, columns[msgDescription])
	msg.Audio = catalog.NewResourceFromString(getCellString(row, columns[msgAudio]))
	msg.Video = catalog.NewResourceFromString(getCellString(row, columns[msgVideo]))

	var errs []string

	if dStr := getCellString(row, columns[msgDate]); dStr != "" {
		if d, err := parseDateCell(dStr); err == nil {
			msg.Date = d
		} else {
			errs = append(errs, fmt.Sprintf("invalid date '%s' for message '%s'", dStr, msg.Name))
		}
	}

	msg.Ministry = catalog.NewMinistryFromString(getCellString(row, columns[msgMinistry]))
	msg.Type = catalog.NewMessageTypeFromString(getCellString(row, columns[msgType]))
	msg.Visibility = catalog.NewViewFromString(getCellString(row, columns[msgVisibility]))

	for _, speaker := range strings.Split(getCellString(row, columns[msgSpeakers]), ";") {
		if s := strings.TrimSpace(speaker); s != "" {
			msg.Speakers = append(msg.Speakers, s)
		}
	}

	msg.Series = catalog.NewSeriesReferencesFromStrings(
		getCellString(row, columns[msgSeries]),
		getCellString(row, columns[msgSeriesIndex]),
	)

	msg.Resources = catalog.NewResourcesFromString(getCellString(row, columns[msgResources]))

	if len(errs) > 0 {
		return msg, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return msg, nil
}

// buildColumnIndex returns a map from column header name to zero-based column index.
func buildColumnIndex(headerRow []string) map[string]int {
	columns := make(map[string]int, len(headerRow))
	for i, name := range headerRow {
		columns[strings.TrimSpace(name)] = i
	}
	return columns
}

// getCellString returns the trimmed string value at index in row, or "" if out of range.
func getCellString(row []string, index int) string {
	if index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

// parseDateCell converts an Excel date cell value to a DateOnly.
// Excel stores dates as floating-point serial numbers; if the raw value parses
// as a float it is converted via ExcelDateToTime. Otherwise it falls back to
// catalog.ParseDateOnly for string-formatted dates.
func parseDateCell(raw string) (catalog.DateOnly, error) {
	raw = strings.TrimSpace(raw)
	if serial, err := strconv.ParseFloat(raw, 64); err == nil {
		t, err := excelize.ExcelDateToTime(serial, false)
		if err == nil {
			return catalog.NewDateOnly(t), nil
		}
	}
	return catalog.ParseDateOnly(raw)
}
