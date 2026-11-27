// Package sheet turns a spreadsheet of messages into a catalog, whatever the
// spreadsheet is kept in.
//
// The spreadsheet itself is reached through a Source. sheet/google reads a Google
// Sheet and sheet/microsoft reads an Excel workbook. This package depends on neither,
// so choosing between them is left to the caller.
package sheet

// Source is a spreadsheet the catalog can be read from: a set of named tabs, each a
// grid of cells.
//
// Every implementation is read-only. The spreadsheet is the hand-maintained system of
// record for the whole media catalog, and nothing here ever writes to it.
type Source interface {
	// Title names the spreadsheet for messages to the operator, e.g. the document's
	// title or the workbook's file name
	Title() string

	// Tabs lists the names of the spreadsheet's tabs, in the order they appear
	Tabs() ([]string, error)

	// Rows returns every row of a tab as displayed text, the column titles first. A row
	// may be shorter than the title row, when its trailing cells are empty.
	Rows(tab string) ([][]string, error)
}
