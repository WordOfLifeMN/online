package gclient

import (
	"github.com/WordOfLifeMN/online/catalog"
	"google.golang.org/api/sheets/v4"
)

// code that can read a spreadsheet and generate a catalog model from it

// NewCatalogFromSheet takes a valid spreadsheet service and a spreadsheet ID
// and creates a catalog from the info in the spreadsheet
func NewCatalogFromSheet(service *sheets.Service, sheetID string) (*catalog.Catalog, error) {
	// TODO - implement
	return nil, nil
}
