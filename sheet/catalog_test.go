package sheet

import (
	"errors"
	"testing"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/util"
	"github.com/stretchr/testify/suite"
)

func TestCatalogTestSuite(t *testing.T) {
	suite.Run(t, new(CatalogTestSuite))
}

type CatalogTestSuite struct {
	suite.Suite
}

// fakeSource is a Source held in memory: tab name to rows, titles first
type fakeSource struct {
	order []string
	tabs  map[string][][]string
}

func (f fakeSource) Title() string           { return "fake" }
func (f fakeSource) Tabs() ([]string, error) { return f.order, nil }
func (f fakeSource) Rows(tab string) ([][]string, error) {
	rows, ok := f.tabs[tab]
	if !ok {
		return nil, errors.New("no such tab")
	}
	return rows, nil
}

var messageTitles = []string{
	"Date", "Name", "Speaker", "Type", "Visibility",
	"Series Name", "Track", "Description", "Thumb", "Video", "Resources",
}

func (t *CatalogTestSuite) TestNewCatalogSeriFromMessageRow_Series() {
	// given a Series-type message row
	thumb := catalog.OnlineResource{URL: "http://thumb.png", Name: "thumb"}
	msg := catalog.CatalogMessage{
		Name:        "Grace and Truth",
		Description: "A series on grace",
		Visibility:  catalog.Public,
		Type:        catalog.Series,
		Thumb:       &thumb,
		Resources:   []catalog.OnlineResource{{URL: "http://notes.pdf", Name: "notes"}},
	}

	// when
	seri := newCatalogSeriFromMessageRow(msg)

	// then
	t.Equal("Grace and Truth", seri.Name)
	t.Equal("A series on grace", seri.Description)
	t.Equal(catalog.Public, seri.Visibility)
	t.Equal("http://thumb.png", seri.Thumbnail)
	t.Equal(util.ComputeHash("Grace and Truth"), seri.ID)
	// resources on a series row also become booklets
	t.Len(seri.Booklets, 1)
	t.Equal("http://notes.pdf", seri.Booklets[0].URL)
	t.Empty(seri.Resources)
	// dates left zero for Normalize() to compute later
	t.True(seri.StartDate.IsZero())
	t.True(seri.StopDate.IsZero())
}

func (t *CatalogTestSuite) TestNewCatalogSeriFromMessageRow_Booklet() {
	// given a Booklet-type message row
	msg := catalog.CatalogMessage{
		Name:        "Study Guide",
		Description: "Companion booklet",
		Visibility:  catalog.Partner,
		Type:        catalog.Booklet,
		Resources:   []catalog.OnlineResource{{URL: "http://guide.pdf", Name: "guide"}},
	}

	// when
	seri := newCatalogSeriFromMessageRow(msg)

	// then
	t.Equal("Study Guide", seri.Name)
	t.Equal(util.ComputeHash("Study Guide"), seri.ID)
	// resources go to Booklets for Booklet type
	t.Len(seri.Booklets, 1)
	t.Equal("http://guide.pdf", seri.Booklets[0].URL)
	t.Empty(seri.Resources)
	// no thumbnail when Thumb is nil
	t.Empty(seri.Thumbnail)
}

func (t *CatalogTestSuite) TestNewCatalogSeriFromMessageRow_Playlist() {
	// given a Series row with a playlist in the Video column
	msg := catalog.CatalogMessage{
		Name:  "Here I Am!",
		Type:  catalog.Series,
		Video: catalog.NewResourceFromString("https://www.youtube.com/playlist?list=PLIv9NPKsgJlo"),
	}

	// when
	seri := newCatalogSeriFromMessageRow(msg)

	// then the link is kept, without the name made up from the URL
	t.Require().NotNil(seri.Playlist)
	t.Equal("https://www.youtube.com/playlist?list=PLIv9NPKsgJlo", seri.Playlist.URL)
	t.Empty(seri.Playlist.Name)

	// a name typed in the cell is kept
	msg.Video = catalog.NewResourceFromString("Here I Am|https://www.youtube.com/playlist?list=PLIv9NPKsgJlo")
	t.Equal("Here I Am", newCatalogSeriFromMessageRow(msg).Playlist.Name)

	t.False(seri.NoPlaylist)

	// a blank Video column has no playlist, and doesn't say why
	msg.Video = catalog.NewResourceFromString("")
	seri = newCatalogSeriFromMessageRow(msg)
	t.Nil(seri.Playlist)
	t.False(seri.NoPlaylist)

	// "-" says the series has no playlist on purpose
	msg.Video = catalog.NewResourceFromString("-")
	seri = newCatalogSeriFromMessageRow(msg)
	t.Nil(seri.Playlist)
	t.True(seri.NoPlaylist)

	// anything else is kept for validation to reject
	msg.Video = catalog.NewResourceFromString("pending")
	t.Equal("pending", newCatalogSeriFromMessageRow(msg).Playlist.URL)
	msg.Video = catalog.NewResourceFromString("https://www.youtube.com/playlist?list=PLIv9NPKsgJlo")
	msg.Type = catalog.Booklet
	t.Nil(newCatalogSeriFromMessageRow(msg).Playlist)
}

func (t *CatalogTestSuite) TestGetCellString() {
	row := []string{"hello", "  world  ", "42"}
	t.Equal("hello", getCellString(row, 0))
	t.Equal("world", getCellString(row, 1)) // trims whitespace
	t.Equal("42", getCellString(row, 2))
	t.Equal("", getCellString(row, 99)) // out-of-range returns empty
	t.Equal("", getCellString(nil, 0))  // nil slice returns empty
}

func (t *CatalogTestSuite) TestGetIndexOfColumns() {
	columns := getIndexOfColumns([]string{"One", "Two", "And-a-Three", "Four Space", "!# Five †"})
	t.Len(columns, 5)
	t.Equal(0, columns["One"])
	t.Equal(1, columns["Two"])
	t.Equal(2, columns["And-a-Three"])
	t.Equal(3, columns["Four Space"])
	t.Equal(4, columns["!# Five †"])
}

func msgColumnsForTest() map[string]int {
	return map[string]int{
		"Date": 0, "Name": 1, "Description": 2,
		"Speaker": 3, "Type": 4, "Visibility": 5,
		"Series Name": 6, "Track": 7,
		"Video": 9, "Resources": 10,
		// "Ministry" and "Thumb" intentionally omitted to test optional-column paths
	}
}

func (t *CatalogTestSuite) TestNewCatalogMessageFromRow_NoThumbColumn() {
	// Thumb column absent from the columns map → msg.Thumb must be nil
	rowData := []string{
		"2021-06-01", "Test Message", "A description",
		"Speaker One", "message", "public",
		"", "", "", "", "",
	}

	msg, err := newCatalogMessageFromRow(msgColumnsForTest(), rowData, "wol")
	t.NoError(err)
	t.Nil(msg.Thumb)
}

func (t *CatalogTestSuite) TestNewCatalogMessageFromRow_DefaultMinistry() {
	rowData := []string{
		"2021-06-01", "Test Message", "",
		"", "message", "public",
		"", "", "", "", "",
	}

	// No Ministry column → defaultMinistry is used
	msg, err := newCatalogMessageFromRow(msgColumnsForTest(), rowData, "wol")
	t.NoError(err)
	t.Equal(catalog.WordOfLife, msg.Ministry)

	// Ministry column present but cell is empty → still falls back to defaultMinistry
	columnsWithMinistry := msgColumnsForTest()
	columnsWithMinistry["Ministry"] = 11
	rowWithEmptyMinistry := append(append([]string{}, rowData...), "")

	msg, err = newCatalogMessageFromRow(columnsWithMinistry, rowWithEmptyMinistry, "wol")
	t.NoError(err)
	t.Equal(catalog.WordOfLife, msg.Ministry)
}

func (t *CatalogTestSuite) TestNewCatalogMessageFromRow_MinistryFromColumn() {
	// Ministry column present with non-empty value → overrides defaultMinistry
	columns := msgColumnsForTest()
	columns["Ministry"] = 11
	rowData := []string{
		"2021-06-01", "Test Message", "",
		"", "message", "public",
		"", "", "", "", "", "tbo",
	}

	msg, err := newCatalogMessageFromRow(columns, rowData, "wol")
	t.NoError(err)
	t.Equal(catalog.TheBridgeOutreach, msg.Ministry)
}

func (t *CatalogTestSuite) TestNewCatalogMessageFromRow_MultipleSpeakers() {
	rowData := []string{
		"2021-06-01", "Test Message", "",
		"Alice;Bob;Charlie", "message", "public",
		"", "", "", "", "",
	}

	msg, err := newCatalogMessageFromRow(msgColumnsForTest(), rowData, "wol")
	t.NoError(err)
	t.Len(msg.Speakers, 3)
	t.Equal("Alice", msg.Speakers[0])
	t.Equal("Bob", msg.Speakers[1])
	t.Equal("Charlie", msg.Speakers[2])
}

func (t *CatalogTestSuite) TestNewCatalogMessageFromRow_InvalidDate() {
	// unparseable date → zero DateOnly, no error returned
	rowData := []string{
		"not-a-date", "Test Message", "",
		"", "message", "public",
		"", "", "", "", "",
	}

	msg, err := newCatalogMessageFromRow(msgColumnsForTest(), rowData, "wol")
	t.NoError(err)
	t.True(msg.Date.IsZero())
}

func (t *CatalogTestSuite) TestNewCatalog() {
	// given a spreadsheet with two message tabs, one ignored tab, and one tab missing
	// required columns
	src := fakeSource{
		order: []string{"WOL", "_validate", "Broken", "TBO"},
		tabs: map[string][][]string{
			"WOL": {
				messageTitles,
				{"2021-06-01", "First", "Pastor Vern", "message", "public", "Grace", "1"},
				{"2021-06-08", "Grace", "", "series", "public", "", "", "A series", "", "-"},
				{"2021-06-15", "Not a type", "", "bogus", "public"},
				{}, // a blank row
			},
			"_validate": {{"Types"}, {"message"}},
			"Broken":    {{"Date", "Name"}, {"2021-06-01", "Ignored"}},
			"TBO": {
				messageTitles,
				{"2021-07-01", "Bridge", "Speaker", "message", "public"},
			},
		},
	}

	// when
	cat, err := NewCatalog(src)

	// then messages come from both message tabs, in order, with the tab as ministry
	t.Require().NoError(err)
	t.Require().Len(cat.Messages, 2)
	t.Equal("First", cat.Messages[0].Name)
	t.Equal(catalog.MustParseDateOnly("2021-06-01"), cat.Messages[0].Date)
	t.Equal(catalog.WordOfLife, cat.Messages[0].Ministry)
	t.Equal([]catalog.SeriesReference{{Name: "Grace", Index: 1}}, cat.Messages[0].Series)
	t.Equal("Bridge", cat.Messages[1].Name)
	t.Equal(catalog.TheBridgeOutreach, cat.Messages[1].Ministry)

	// and the Series row becomes a series
	t.Require().Len(cat.Series, 1)
	t.Equal("Grace", cat.Series[0].Name)
	t.Equal(util.ComputeHash("Grace"), cat.Series[0].ID)
	t.True(cat.Series[0].NoPlaylist)
}

func (t *CatalogTestSuite) TestNewCatalog_EmptyTab() {
	// an empty tab is skipped, not a failure
	src := fakeSource{
		order: []string{"Empty", "WOL"},
		tabs: map[string][][]string{
			"Empty": {},
			"WOL":   {messageTitles, {"2021-06-01", "First", "", "message", "public"}},
		},
	}

	cat, err := NewCatalog(src)
	t.Require().NoError(err)
	t.Len(cat.Messages, 1)
}
