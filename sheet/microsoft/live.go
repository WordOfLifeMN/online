package microsoft

// reading the workbook from the running desktop Excel, when it has it open. Excel picks
// up edits made in the browser within seconds, while the local OneDrive copy has been
// measured at 4 to 14 minutes behind.

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/WordOfLifeMN/online/sheet"
)

// Reading says where a workbook was read from
type Reading struct {
	// Live is true when it was read from the running Excel
	Live bool

	// Fallback says why the running Excel couldn't be used, when there was a reason
	// worth telling the operator: Excel was busy, say. It is empty when it was read
	// live, or when Excel simply isn't running or doesn't have the workbook open.
	Fallback string
}

// Open reads a workbook from the running Excel if Excel has it open, and from the file
// otherwise. Excel is never started, and the workbook is never opened in it.
func Open(path string) (sheet.Source, Reading, error) {
	if err := checkExtension(path); err != nil {
		return nil, Reading{}, err
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}

	live, err := readLive(path)
	if err == nil {
		return live, Reading{Live: true}, nil
	}
	log.Printf("Not reading %s from Excel: %v", filepath.Base(path), err)

	var reading Reading
	switch {
	case errors.Is(err, errNotRunning), errors.Is(err, errNotOpen):
		// the usual case when Excel is closed: nothing to explain
	case errors.Is(err, errBusy), errors.Is(err, errAmbiguous), errors.Is(err, errUnreachable),
		errors.Is(err, errLiveTimeout):
		reading.Fallback = err.Error()
	default:
		reading.Fallback = fmt.Sprintf("Couldn't read from Excel (%v)", err)
	}

	w, err := OpenFile(path)
	if err != nil {
		return nil, reading, err
	}
	return w, reading, nil
}

var (
	errNotRunning  = errors.New("Excel is not running")
	errNotOpen     = errors.New("the workbook is not open in Excel")
	errBusy        = errors.New("Excel is busy")
	errAmbiguous   = errors.New("Excel has more than one workbook open that could be this one")
	errUnreachable = errors.New("Excel is running but can't be reached (is one of them running as Administrator?)")
	errLiveTimeout = errors.New("Excel didn't answer")
)

// matchWorkbook reports whether a workbook open in Excel is the one at path.
//
// A workbook opened from OneDrive is known to Excel by its SharePoint URL, not by the
// local path it is synced to, so the URL is matched on its last two parts, the folder
// and the file name: ".../WOLCatalog/MediaCatalog.xlsx" matches
// "C:\...\WOLCatalog\MediaCatalog.xlsx". The name alone isn't enough: a backup copy
// elsewhere, with the same name, must not read the live workbook. A workbook opened
// from a local path must be at exactly that path.
func matchWorkbook(path, name, fullName string) bool {
	if !strings.EqualFold(name, filepath.Base(path)) {
		return false
	}
	lowered := strings.ToLower(fullName)
	if strings.HasPrefix(lowered, "https://") || strings.HasPrefix(lowered, "http://") {
		if unescaped, err := url.PathUnescape(fullName); err == nil {
			fullName = unescaped
		}
		return strings.EqualFold(folderAndName(fullName), folderAndName(path))
	}
	return strings.EqualFold(filepath.Clean(fullName), filepath.Clean(path))
}

// folderAndName is the last two parts of a path or URL, "WOLCatalog/MediaCatalog.xlsx"
func folderAndName(p string) string {
	parts := strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) > 2 {
		parts = parts[len(parts)-2:]
	}
	return strings.Join(parts, "/")
}

// liveWorkbook is a workbook as the running Excel holds it, read in full
type liveWorkbook struct {
	name string
	tabs []string
	rows map[string][][]string
}

var _ sheet.Source = (*liveWorkbook)(nil)

// Title is the workbook's file name
func (w *liveWorkbook) Title() string {
	return w.name
}

// Tabs lists the workbook's worksheets, in order
func (w *liveWorkbook) Tabs() ([]string, error) {
	return w.tabs, nil
}

// Rows returns a worksheet's rows, read when the workbook was
func (w *liveWorkbook) Rows(tab string) ([][]string, error) {
	rows, ok := w.rows[tab]
	if !ok {
		return nil, fmt.Errorf("no sheet named %q in %s", tab, w.name)
	}
	return rows, nil
}
