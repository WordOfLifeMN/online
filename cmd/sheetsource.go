package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/WordOfLifeMN/online/sheet"
	"github.com/WordOfLifeMN/online/sheet/google"
	"github.com/WordOfLifeMN/online/sheet/microsoft"
	"github.com/WordOfLifeMN/online/util"
	"github.com/spf13/viper"
)

// sheetKind says what a sheet-id names
type sheetKind int

const (
	googleSheet   sheetKind = iota // the ID of a Google Sheet
	excelWorkbook                  // the path of an Excel workbook
)

// classifySheetRef works out what a sheet-id names, and returns it ready to open.
//
// A path to an Excel workbook is anything with a path separator, a leading "~", or an
// Excel extension. Anything else is a Google Sheet ID, which is base64url and so can
// never look like a path. A URL is neither, and is refused rather than guessed at.
func classifySheetRef(ref string) (sheetKind, string, error) {
	ref = strings.TrimSpace(ref)
	lowered := strings.ToLower(ref)

	switch {
	case strings.HasPrefix(lowered, "http://"), strings.HasPrefix(lowered, "https://"):
		return 0, "", fmt.Errorf("sheet-id %q is a URL: it takes the ID of a Google Sheet, "+
			"or the path of an Excel workbook", ref)
	case strings.ContainsAny(ref, `\/`),
		strings.HasPrefix(ref, "~"),
		strings.HasSuffix(lowered, ".xlsx"),
		strings.HasSuffix(lowered, ".xlsm"):
		return excelWorkbook, util.NormalizePath(ref), nil
	default:
		return googleSheet, ref, nil
	}
}

// sheetIsWorkbook reports whether the configured sheet-id names an Excel workbook
func sheetIsWorkbook() bool {
	kind, _, err := classifySheetRef(viper.GetString("sheet-id"))
	return err == nil && kind == excelWorkbook
}

// openSheetSource opens the spreadsheet the sheet-id names. A workbook is opened
// directly, and is never mistaken for a Google Sheet: a missing one is an error that
// names it.
func openSheetSource(ctx context.Context, ref string) (sheet.Source, error) {
	kind, ref, err := classifySheetRef(ref)
	if err != nil {
		return nil, err
	}

	if kind == googleSheet {
		return google.Open(ctx, ref)
	}

	w, err := microsoft.Open(ref)
	if err != nil {
		return nil, err
	}
	saved, ok := w.Saved()
	printWorkbookSaved(os.Stderr, w.Title(), saved, ok, w.ModTime(), time.Now())
	return w, nil
}

// printWorkbookSaved says how up to date the workbook that was read is. Edits reach
// the local copy only when OneDrive brings them down, seconds or minutes after they are
// made, so this is how the operator can tell whether the last edit is in yet.
//
// It uses the save time Excel records inside the workbook, which travels with the
// contents. The file's own time is shown only when that is missing, and is labelled as
// such, since OneDrive sets it to the server's time and it was seen to go backwards.
//
// It goes to stderr, so it can't corrupt 'online dump -o stdout'.
func printWorkbookSaved(w io.Writer, name string, saved time.Time, ok bool, fileTime, now time.Time) {
	if ok {
		fmt.Fprintf(w, "Reading %s (includes edits up to %s)\n", name, describeSaved(saved, now))
		return
	}
	fmt.Fprintf(w, "Reading %s (file dated %s)\n", name, describeSaved(fileTime, now))
}

// describeSaved puts a save time the way an operator glancing at it needs it: the clock
// time, and how long ago that was
func describeSaved(saved, now time.Time) string {
	age := now.Sub(saved)
	switch {
	case age < 0:
		return saved.Format("15:04:05")
	case age < time.Minute:
		return fmt.Sprintf("%s, %d s ago", saved.Format("15:04:05"), int(age.Seconds()))
	case age < time.Hour:
		return fmt.Sprintf("%s, %d min ago", saved.Format("15:04:05"), int(age.Minutes()))
	case age < 24*time.Hour:
		return fmt.Sprintf("%s, %d h ago", saved.Format("15:04"), int(age.Hours()))
	default:
		return saved.Format("Jan 2 15:04")
	}
}
