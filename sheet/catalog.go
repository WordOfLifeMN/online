package sheet

// code that reads a spreadsheet and generates a catalog model from it

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/util"
)

// NewCatalog reads every message tab of a spreadsheet and creates a catalog from it
func NewCatalog(src Source) (*catalog.Catalog, error) {
	// initialize the catalog
	catalog := catalog.Catalog{
		Created: time.Now(),
	}

	log.Printf("Reading catalog from spreadsheet %s", src.Title())
	messages, msgSeries, err := readMessages(src)
	if err != nil {
		return &catalog, err
	}
	catalog.Messages = messages
	catalog.Series = msgSeries

	return &catalog, nil
}

// DateColumn is the title of the column holding each message's date. A Source may need
// it to read dates by value rather than as displayed.
const DateColumn = "Date"

const (
	msgName        string = "Name"
	msgDate        string = DateColumn
	msgSpeakers    string = "Speaker"
	msgMinistry    string = "Ministry"
	msgType        string = "Type"
	msgVisibility  string = "Visibility"
	msgSeries      string = "Series Name"
	msgSeriesIndex string = "Track"
	msgDescription string = "Description"
	msgThumb       string = "Thumb"
	msgVideo       string = "Video"
	msgResources   string = "Resources"
)

var requiredMessageColumns []string = []string{
	msgDate, msgName, msgDescription,
	msgSpeakers,
	msgType, msgVisibility,
	msgSeries, msgSeriesIndex,
	msgVideo,
	msgResources,
}

// readMessages reads the message data from every tab of the spreadsheet. It also
// extracts any Series/Booklet rows and returns them as a separate series list.
func readMessages(src Source) ([]catalog.CatalogMessage, []catalog.CatalogSeri, error) {
	var messages []catalog.CatalogMessage
	var series []catalog.CatalogSeri

	tabs, err := src.Tabs()
	if err != nil {
		return messages, series, err
	}

	// iterate through all the tabs: skip "_"-prefixed tabs, and treat all others as
	// message tabs using the tab name as the default ministry
	for _, tab := range tabs {
		log.Printf("Checking sheet %s\n", tab)
		if strings.HasPrefix(tab, "_") {
			log.Printf("Ignoring sheet '%s' (starts with '_')\n", tab)
			continue
		}
		tabMessages, tabSeries, err := readMessagesFromTab(src, tab, tab)
		if err != nil {
			log.Printf("Unable to read messages from sheet '%s': %s", tab, err)
			continue
		}
		messages = append(messages, tabMessages...)
		series = append(series, tabSeries...)
	}

	return messages, series, nil
}

// readMessagesFromTab reads the messages from a single tab of the spreadsheet.
// defaultMinistry is used for any message that does not have an explicit Ministry column value.
// Rows with type Series or Booklet are returned as CatalogSeri rather than CatalogMessage.
func readMessagesFromTab(src Source, tab string, defaultMinistry string) ([]catalog.CatalogMessage, []catalog.CatalogSeri, error) {
	log.Printf("Reading the Messages from tab '%s'\n", tab)

	rows, err := src.Rows(tab)
	if err != nil {
		log.Printf("Unable to read the messages: %v", err)
		return nil, nil, err
	}
	if len(rows) == 0 {
		return nil, nil, fmt.Errorf("sheet '%s' is empty", tab)
	}

	// the first row holds the column titles
	columns := getIndexOfColumns(rows[0])
	log.Printf("  Found %d columns:\n", len(columns))

	// validate that the columns we are expecting are actually there
	for _, requiredColumn := range requiredMessageColumns {
		if _, ok := columns[requiredColumn]; !ok {
			return nil, nil, fmt.Errorf("required column '%s' cannot be found in sheet '%s'",
				requiredColumn, tab)
		}
	}

	var messages []catalog.CatalogMessage
	var series []catalog.CatalogSeri

	log.Printf("  Found %d rows", len(rows)-1)
	for messageIndex, messageRow := range rows[1:] {
		message, err := newCatalogMessageFromRow(columns, messageRow, defaultMinistry)
		if err != nil {
			log.Printf("Unable to read message from row %d: %s", messageIndex+2, err)
		}
		switch message.Type {
		case catalog.Series, catalog.Booklet:
			series = append(series, newCatalogSeriFromMessageRow(message))
		default:
			if message.Type != catalog.UnknownType {
				messages = append(messages, message)
			}
		}
	}
	log.Printf("    %d series, %d messages", len(series), len(messages))

	return messages, series, nil
}

// newCatalogSeriFromMessageRow converts a CatalogMessage with type Series or Booklet into a
// CatalogSeri. The ID is derived from the name so it is consistent across loads. StartDate and
// StopDate are left zero — they are computed later by Normalize() once messages are attached.
func newCatalogSeriFromMessageRow(msg catalog.CatalogMessage) catalog.CatalogSeri {
	seri := catalog.CatalogSeri{}
	seri.Name = msg.Name
	seri.Description = msg.Description
	seri.Visibility = msg.Visibility
	if msg.Thumb != nil {
		seri.Thumbnail = msg.Thumb.URL
	}
	seri.ID = util.ComputeHash(msg.Name)

	// The Video column of a Series row holds the series' playlist, or "-" when it has none
	// on purpose. The name made up from the URL ("playlist?list=PL...") says nothing, so
	// only a name typed in the cell is kept
	switch {
	case msg.Type != catalog.Series || msg.Video == nil || msg.Video.URL == "":
	case msg.Video.URL == "-":
		seri.NoPlaylist = true
	default:
		playlist := *msg.Video
		if playlist.Name == playlist.GetNameFromURL() {
			playlist.Name = ""
		}
		seri.Playlist = &playlist
	}

	// Resources on the row become booklets so they appear on the booklet page.
	// A Booklet row is only a booklet. A Series row still becomes a full series once
	// its messages are attached in Catalog.Initialize(); the booklets ride along via
	// seri.Booklets, which Normalize() preserves. A Series row with no resources has
	// an empty Booklets slice and remains a plain series.
	seri.Booklets = msg.Resources

	return seri
}

// newCatalogMessageFromRow generates a new CatalogMessage object from the raw
// sheet data. The columns contains the index of column names to column indices,
// and all the required columns must be present when called. rowData is the raw
// row data from the sheet
func newCatalogMessageFromRow(columns map[string]int, rowData []string, defaultMinistry string) (catalog.CatalogMessage, error) {
	msg := catalog.CatalogMessage{}

	// simple mapping
	msg.Name = getCellString(rowData, columns[msgName])
	msg.Description = getCellString(rowData, columns[msgDescription])

	if colIdx, ok := columns[msgThumb]; ok {
		msg.Thumb = catalog.NewResourceFromString(getCellString(rowData, colIdx))
	}
	msg.Video = catalog.NewResourceFromString(getCellString(rowData, columns[msgVideo]))

	// get date
	dString := getCellString(rowData, columns[msgDate])
	if d, err := catalog.ParseDateOnly(dString); err == nil {
		msg.Date = d
	} else {
		log.Printf("WARNING: Cannot parse date '%s' for message '%s'", dString, msg.Name)
	}

	// ministry: use the column value if the column exists and has a value, otherwise use the tab name
	ministryStr := defaultMinistry
	if colIdx, ok := columns[msgMinistry]; ok {
		if v := getCellString(rowData, colIdx); v != "" {
			ministryStr = v
		}
	}
	msg.Ministry = catalog.NewMinistryFromString(ministryStr)
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

	// unpack resources
	msg.Resources = catalog.NewResourcesFromString(getCellString(rowData, columns[msgResources]))

	return msg, nil
}

// getIndexOfColumns takes the title row of a tab and returns all the column titles in
// a map where the key is the column name, and the value is the index of the column
func getIndexOfColumns(titleRow []string) map[string]int {
	columns := map[string]int{}
	for columnIndex, columnName := range titleRow {
		columns[columnName] = columnIndex
	}
	return columns
}

// getCellString takes a row of data and returns the data in the index'th column of the
// row, trimmed. Returns "" if the index is out of range
func getCellString(rowData []string, index int) string {
	if index >= len(rowData) {
		return ""
	}

	return strings.TrimSpace(rowData[index])
}
