package gclient

// code that can read a spreadsheet and generate a catalog model from it

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/WordOfLifeMN/online/catalog"
	"google.golang.org/api/sheets/v4"
)

// NewCatalogFromSheet takes a valid spreadsheet service and a spreadsheet
// document ID and creates a catalog from the info in the spreadsheet
func NewCatalogFromSheet(service *sheets.Service, documentID string) (*catalog.Catalog, error) {
	// initialize the catalog
	catalog := catalog.Catalog{
		Created: time.Now(),
	}

	// get document information
	document, err := service.Spreadsheets.Get(documentID).Do()
	if err != nil {
		return &catalog, err
	}

	log.Printf("Reading catalog from spreadsheet %s (ID: %s)", document.Properties.Title, documentID)
	series, err := readSeriesFromDocument(service, documentID)
	if err != nil {
		return &catalog, err
	}
	catalog.Series = series

	messages, err := readMessagesFromDocument(service, documentID)
	if err != nil {
		return &catalog, err
	}
	catalog.Messages = messages

	return &catalog, nil
}

const (
	seriesName        string = "Name"
	seriesID          string = "ID"
	seriesDescription string = "Description"
	seriesStartDate   string = "Date Started"
	seriesEndDate     string = "Date Ended"
	seriesBooklets    string = "Booklets"
	seriesResources   string = "Resources"
	seriesVisibility  string = "Visibility"
	seriesCDJacket    string = "CD Jacket"
	seriesDVDJacket   string = "DVD Jacket"
	seriesThumbnail   string = "Cover Image"
)

var requiredSeriesColumns []string = []string{
	seriesName, seriesID, seriesDescription,
	seriesStartDate, seriesEndDate,
	seriesVisibility,
	seriesBooklets, seriesResources,
	seriesCDJacket, seriesDVDJacket, seriesThumbnail,
}

// readSeriesFromDocument finds the "Series" tab and reads the series data from
// it
func readSeriesFromDocument(service *sheets.Service, documentID string) ([]catalog.CatalogSeri, error) {
	tabName := "Series"
	log.Printf("Reading the Series from tab '%s'\n", tabName)

	// get the first row as column titles
	columns, err := getIndexOfColumns(service, documentID, tabName, 1)
	if err != nil {
		return nil, err
	}
	log.Printf("  Found %d columns\n", len(columns))
	// for k, v := range columns {
	// 	log.Printf("    Column %d: %s\n", v, k)
	// }

	// validate that the columns we are expecting are actually there
	for _, requiredColumn := range requiredSeriesColumns {
		if _, ok := columns[requiredColumn]; !ok {
			return nil, fmt.Errorf("required column '%s' cannot be found in sheet '%s'",
				requiredColumn, tabName)
		}
	}

	// prepare the series
	var series []catalog.CatalogSeri

	// read a the series data from the spreadsheet
	seriesRange := fmt.Sprintf("'%s'!2:80000", tabName)
	values, err := service.Spreadsheets.Values.Get(documentID, seriesRange).Do()
	if err != nil {
		log.Printf("Unable to read the series: %v", err)
		return series, err
	}

	// iterate through all the results, creating a new series for each one
	log.Printf("  Found %d series", len(values.Values))
	for seriesIndex, seriesRow := range values.Values {
		seri, err := newCatalogSeriFromRow(columns, seriesRow)
		if err != nil {
			log.Printf("Unable to read series from row %d: %s", seriesIndex+2, err)
		}
		series = append(series, seri)
	}

	return series, nil
}

// newCatalogSeriFromRow generates a new CatalogSeri object from the raw sheet
// data. The columns contains the index of column names to column indices, and
// all the required columns must be present when called. rowData is the raw row
// data from the sheet
func newCatalogSeriFromRow(columns map[string]int, rowData []interface{}) (catalog.CatalogSeri, error) {
	seri := catalog.CatalogSeri{}

	// simple mapping
	seri.ID = getCellString(rowData, columns[seriesID])
	seri.Name = getCellString(rowData, columns[seriesName])
	seri.Description = getCellString(rowData, columns[seriesDescription])
	seri.Visibility = catalog.NewViewFromString(getCellString(rowData, columns[seriesVisibility]))
	seri.Thumbnail = getCellString(rowData, columns[seriesThumbnail])

	// get dates
	dString := getCellString(rowData, columns[seriesStartDate])
	if dString == "" {
		seri.StartDate = catalog.DateOnly{} // zero date
	} else if d, err := catalog.ParseDateOnly(dString); err == nil {
		seri.StartDate = d
	} else {
		log.Printf("WARNING: Cannot parse start date '%s' for series '%s'", dString, seri.Name)
		seri.StartDate = catalog.DateOnly{} // zero date
	}
	dString = getCellString(rowData, columns[seriesEndDate])
	if dString == "" {
		seri.StopDate = catalog.DateOnly{} // zero date
	} else if d, err := catalog.ParseDateOnly(dString); err == nil {
		seri.StopDate = d
	} else {
		log.Printf("WARNING: Cannot parse end date '%s' for series '%s'", dString, seri.Name)
		seri.StopDate = catalog.DateOnly{} // zero date
	}

	// jacket prefers the DVD, then CD
	seri.Jacket = getCellString(rowData, columns[seriesDVDJacket])
	if seri.Jacket == "" {
		seri.Jacket = getCellString(rowData, columns[seriesCDJacket])
	}

	// unpack resources
	seri.Booklets = catalog.NewResourcesFromString(getCellString(rowData, columns[seriesBooklets]))
	seri.Resources = catalog.NewResourcesFromString(getCellString(rowData, columns[seriesResources]))

	return seri, nil
}

const (
	msgName        string = "Name"
	msgDate        string = "Date"
	msgSpeakers    string = "Speaker"
	msgMinistry    string = "Ministry"
	msgType        string = "Type"
	msgVisibility  string = "Visibility"
	msgSeries      string = "Series Name"
	msgSeriesIndex string = "Track"
	msgPlaylist    string = "Playlist"
	msgDescription string = "Description"
	msgAudio       string = "Audio Link"
	msgVideo       string = "Video Link"
	msgResources   string = "Resources"
)

var requiredMessageColumns []string = []string{
	msgDate, msgName, msgDescription,
	msgSpeakers,
	msgMinistry, msgType, msgVisibility,
	msgSeries, msgSeriesIndex, msgPlaylist,
	msgAudio, msgVideo,
	msgResources,
}

// readMessagesFromDocument finds the "Messages" tabs and reads the message data
// from them
func readMessagesFromDocument(service *sheets.Service, documentID string) ([]catalog.CatalogMessage, error) {
	messages := []catalog.CatalogMessage{}

	// get information about all the sheets
	document, err := service.Spreadsheets.Get(documentID).Do()
	if err != nil {
		return messages, err
	}

	// iterate through all the sheets looking for those that start with "Messages"
	for _, sheet := range document.Sheets {
		title := strings.ToLower(sheet.Properties.Title)
		log.Printf("Checking sheet %s\n", title)
		if strings.HasPrefix(title, "messages") || strings.HasPrefix(title, "msgs") {
			sheetMessages, err := readMessagesFromSheet(service, documentID, sheet.Properties.Title)
			if err != nil {
				log.Printf("Unable to read messages from sheet '%s'", sheet.Properties.Title)
				continue
			}
			messages = append(messages, sheetMessages...)
		}
	}

	return messages, nil
}

// readMessagesFromSheet reads a series of messages from a single sheet in a document
func readMessagesFromSheet(service *sheets.Service, documentID string, sheetName string) ([]catalog.CatalogMessage, error) {
	log.Printf("Reading the Messages from tab '%s'\n", sheetName)

	// get the first row as column titles
	columns, err := getIndexOfColumns(service, documentID, sheetName, 1)
	if err != nil {
		return nil, err
	}
	log.Printf("  Found %d columns:\n", len(columns))
	// for k, v := range columns {
	// 	log.Printf("    Column %d: %s\n", v, k)
	// }

	// validate that the columns we are expecting are actually there
	for _, requiredColumn := range requiredMessageColumns {
		if _, ok := columns[requiredColumn]; !ok {
			return nil, fmt.Errorf("required column '%s' cannot be found in sheet '%s'",
				requiredColumn, sheetName)
		}
	}

	// prepare the series
	var messages []catalog.CatalogMessage

	// read a the series data from the spreadsheet
	messageRange := fmt.Sprintf("'%s'!2:80000", sheetName)
	values, err := service.Spreadsheets.Values.Get(documentID, messageRange).Do()
	if err != nil {
		log.Printf("Unable to read the messages: %v", err)
		return messages, err
	}

	// iterate through all the results, creating a new series for each one
	log.Printf("  Found %d messages", len(values.Values))
	for messageIndex, messageRow := range values.Values {
		message, err := newCatalogMessageFromRow(columns, messageRow)
		if err != nil {
			log.Printf("Unable to read message from row %d: %s", messageIndex+2, err)
		}
		messages = append(messages, message)
	}

	return messages, nil

}

// newCatalogMessageFromRow generates a new CatalogMessage object from the raw
// sheet data. The columns contains the index of column names to column indices,
// and all the required columns must be present when called. rowData is the raw
// row data from the sheet
func newCatalogMessageFromRow(columns map[string]int, rowData []interface{}) (catalog.CatalogMessage, error) {
	msg := catalog.CatalogMessage{}

	// simple mapping
	msg.Name = getCellString(rowData, columns[msgName])
	msg.Description = getCellString(rowData, columns[msgDescription])
	msg.Audio = getCellString(rowData, columns[msgAudio])
	msg.Video = getCellString(rowData, columns[msgVideo])

	// get date
	dString := getCellString(rowData, columns[msgDate])
	if d, err := catalog.ParseDateOnly(dString); err == nil {
		msg.Date = d
	} else {
		log.Printf("WARNING: Cannot parse date '%s' for message '%s'", dString, msg.Name)
	}

	// enums
	msg.Ministry = catalog.NewMinistryFromString(getCellString(rowData, columns[msgMinistry]))
	msg.Type = catalog.NewMessageTypeFromString(getCellString(rowData, columns[msgType]))
	msg.Visibility = catalog.NewViewFromString(getCellString(rowData, columns[msgVisibility]))

	// speakers
	s := getCellString(rowData, columns[msgSpeakers])
	for _, speaker := range strings.Split(s, ";") {
		if speaker != "" {
			msg.Speakers = append(msg.Speakers, speaker)
		}
	}

	// series
	msg.Series = catalog.NewSeriesReferencesFromStrings(
		getCellString(rowData, columns[msgSeries]),
		getCellString(rowData, columns[msgSeriesIndex]),
	)

	// playlist
	s = getCellString(rowData, columns[msgPlaylist])
	for _, playlist := range strings.Split(s, ";") {
		playlist = strings.TrimSpace(playlist)
		playlist = strings.ToLower(playlist)
		if playlist != "" {
			msg.Playlist = append(msg.Playlist, playlist)
		}
	}

	// unpack resources
	msg.Resources = catalog.NewResourcesFromString(getCellString(rowData, columns[msgResources]))

	return msg, nil
}

// getIndexOfColumns takes a sheet name and returns all the column titles in a
// map where the key is the column name, and the value is the index of the
// column
func getIndexOfColumns(service *sheets.Service, documentID string, tabName string, titleRow int) (map[string]int, error) {
	// the range of the column titles is always the entire row
	titleRange := fmt.Sprintf("'%s'!%d:%d", tabName, titleRow, titleRow)

	values, err := service.Spreadsheets.Values.Get(documentID, titleRange).Do()
	if err != nil {
		return nil, err
	}

	columns := map[string]int{}
	for columnIndex, columnName := range values.Values[0] {
		columns[columnName.(string)] = columnIndex
	}

	return columns, nil
}

// getCellString takes a row of data and returns the string version of the data in
// the index'th column of the row. Returns "" if the index is out of range
func getCellString(rowData []interface{}, index int) string {
	if index >= len(rowData) {
		return ""
	}

	return strings.TrimSpace(fmt.Sprintf("%v", rowData[index]))
}
