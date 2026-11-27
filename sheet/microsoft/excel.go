// Package microsoft reads the catalog spreadsheet from an Excel workbook, typically one
// kept in OneDrive or SharePoint and synced to this machine.
package microsoft

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/WordOfLifeMN/online/sheet"
	"github.com/xuri/excelize/v2"
)

// CRITICAL: The workbook MUST only ever be read.
//
// It is the hand-maintained system of record for the entire media catalog, as the
// Google Sheet was before it, and a damaged copy syncs to OneDrive within seconds. So
// the file is opened for reading only (see readShared), parsed from a copy in memory,
// and excelize's Save, SaveAs and Write are never called.
//
// Do NOT add a write path here as an incidental part of some other change. If writing
// to the workbook is ever genuinely wanted, raise it, discuss it, and get explicit
// approval first.

// Workbook is an Excel workbook, read in full when opened
type Workbook struct {
	path     string
	modTime  time.Time
	savedAt  time.Time // zero when the workbook doesn't record it
	file     *excelize.File
	date1904 bool
}

var _ sheet.Source = (*Workbook)(nil)

// Open reads a workbook into memory, even while Excel has it open. The file is closed
// again before Open returns.
func Open(path string) (*Workbook, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".xlsx", ".xlsm":
	default:
		return nil, fmt.Errorf("%s is not an Excel workbook: only .xlsx and .xlsm files are supported", path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot find the workbook: %w", err)
	}

	data, err := readShared(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read the workbook: %w", err)
	}

	file, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("cannot read %s as a workbook: %w", path, err)
	}

	w := &Workbook{path: path, modTime: info.ModTime(), file: file}
	if props, err := file.GetWorkbookProps(); err == nil && props.Date1904 != nil {
		w.date1904 = *props.Date1904
	}
	if props, err := file.GetDocProps(); err == nil && props.Modified != "" {
		if t, err := time.Parse(time.RFC3339, props.Modified); err == nil {
			w.savedAt = t.Local()
		}
	}
	return w, nil
}

// Title is the workbook's file name
func (w *Workbook) Title() string {
	return filepath.Base(w.path)
}

// Saved is when the version of the workbook that was read was saved, as Excel records
// it inside the file (docProps/core.xml). False when the workbook doesn't record it.
//
// This says which edits the copy that was read includes. The file's modification time
// doesn't: OneDrive stamps a file it brings down with the server's time, which was seen
// to go backwards from one run to the next, and it changes when the file is rewritten
// even if its contents don't.
func (w *Workbook) Saved() (time.Time, bool) {
	return w.savedAt, !w.savedAt.IsZero()
}

// ModTime is the local file's modification time. Prefer Saved: this only stands in
// for it when the workbook doesn't record when it was saved.
func (w *Workbook) ModTime() time.Time {
	return w.modTime
}

// Tabs lists the workbook's tabs, in order
func (w *Workbook) Tabs() ([]string, error) {
	return w.file.GetSheetList(), nil
}

// Rows reads a whole tab, each cell as it is displayed, which is what the Sheets API
// returns for a Google Sheet. A whole number reads "1", not "1.0".
//
// Hyperlinks are ignored: a cell's text is what it says, whatever it links to.
//
// The one exception is the Date column. A cell holding a real Excel date is read by
// value, as yyyy-mm-dd, however it is formatted. Otherwise a date typed in Excel's US
// format would display, and so read, as "11/27/2026", which the catalog doesn't
// accept.
func (w *Workbook) Rows(tab string) ([][]string, error) {
	rows, err := w.file.GetRows(tab)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return rows, nil
	}

	dateCol := -1
	for i, title := range rows[0] {
		if title == sheet.DateColumn {
			dateCol = i
		}
	}
	if dateCol < 0 {
		return rows, nil
	}

	for r := 1; r < len(rows); r++ {
		if dateCol >= len(rows[r]) || strings.TrimSpace(rows[r][dateCol]) == "" {
			continue
		}
		cell, err := excelize.CoordinatesToCellName(dateCol+1, r+1)
		if err != nil {
			continue
		}
		if date, ok := w.dateValue(tab, cell); ok {
			rows[r][dateCol] = date
		}
	}
	return rows, nil
}

// dateValue reads a cell holding an Excel date serial as yyyy-mm-dd. A cell holding
// text, even text that looks like a date, is left alone.
func (w *Workbook) dateValue(tab, cell string) (string, bool) {
	cellType, err := w.file.GetCellType(tab, cell)
	if err != nil {
		return "", false
	}
	switch cellType {
	case excelize.CellTypeUnset, excelize.CellTypeNumber, excelize.CellTypeDate:
	default:
		return "", false
	}

	raw, err := w.file.GetCellValue(tab, cell, excelize.Options{RawCellValue: true})
	if err != nil {
		return "", false
	}
	serial, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || serial < 1 {
		return "", false
	}
	t, err := excelize.ExcelDateToTime(serial, w.date1904)
	if err != nil {
		return "", false
	}
	return t.Format("2006-01-02"), true
}
