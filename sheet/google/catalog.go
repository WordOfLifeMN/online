package google

// a Google Sheet as a sheet.Source

import (
	"context"
	"fmt"

	"github.com/WordOfLifeMN/online/sheet"
	"google.golang.org/api/sheets/v4"
)

// Spreadsheet is a Google Sheet, read through the Sheets API
type Spreadsheet struct {
	service    *sheets.Service
	documentID string
	title      string
	tabs       []string
}

var _ sheet.Source = (*Spreadsheet)(nil)

// Open connects to the Sheets API and reads the spreadsheet's title and list of tabs.
// The cells are read tab by tab, by Rows.
func Open(ctx context.Context, documentID string) (*Spreadsheet, error) {
	service, err := GetSheetService(ctx)
	if err != nil {
		return nil, err
	}

	document, err := service.Spreadsheets.Get(documentID).Do()
	if err != nil {
		return nil, err
	}

	s := &Spreadsheet{
		service:    service,
		documentID: documentID,
		title:      document.Properties.Title,
	}
	for _, tab := range document.Sheets {
		s.tabs = append(s.tabs, tab.Properties.Title)
	}
	return s, nil
}

// Title is the document's title and ID
func (s *Spreadsheet) Title() string {
	return fmt.Sprintf("%s (ID: %s)", s.title, s.documentID)
}

// Tabs lists the document's tabs, in order
func (s *Spreadsheet) Tabs() ([]string, error) {
	return s.tabs, nil
}

// Rows reads a whole tab. The Sheets API returns each cell as it is displayed (its
// default FORMATTED_VALUE rendering), and leaves off trailing empty cells and rows.
func (s *Spreadsheet) Rows(tab string) ([][]string, error) {
	values, err := s.service.Spreadsheets.Values.Get(s.documentID, fmt.Sprintf("'%s'!1:80000", tab)).Do()
	if err != nil {
		return nil, err
	}
	return toStrings(values.Values), nil
}

// toStrings converts the API's rows of cells to rows of text
func toStrings(values [][]any) [][]string {
	rows := make([][]string, len(values))
	for r, row := range values {
		rows[r] = make([]string, len(row))
		for c, cell := range row {
			rows[r][c] = fmt.Sprintf("%v", cell)
		}
	}
	return rows
}
