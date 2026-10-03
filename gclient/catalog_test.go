package gclient

import (
	"context"
	"testing"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/util"
	"github.com/stretchr/testify/suite"
	"google.golang.org/api/sheets/v4"
)

const testDocumentID = "1vvhIGMPvVF-DtWoYsEbVBvzk_VtLyKuIw_zyLdsB-JY"

func TestCatalogTestSuite(t *testing.T) {
	suite.Run(t, new(CatalogTestSuite))
}

type CatalogTestSuite struct {
	suite.Suite
	service *sheets.Service
}

func (t *CatalogTestSuite) SetupSuite() {
	var err error
	t.service, err = GetSheetService(context.Background())
	t.NoError(err)
}

// +---------------------------------------------------------------------------
// | Unit tests (no Google API)
// +---------------------------------------------------------------------------

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

func (t *CatalogTestSuite) TestGetCellString() {
	row := []any{"hello", "  world  ", 42}
	t.Equal("hello", getCellString(row, 0))
	t.Equal("world", getCellString(row, 1)) // trims whitespace
	t.Equal("42", getCellString(row, 2))    // non-string value coerced via fmt
	t.Equal("", getCellString(row, 99))     // out-of-range returns empty
	t.Equal("", getCellString(nil, 0))      // nil slice returns empty
}

func seriColumnsForTest() map[string]int {
	return map[string]int{
		"Name": 0, "ID": 1, "Description": 2,
		"Date Started": 3, "Date Ended": 4,
		"Visibility": 5, "Booklets": 6,
		"CD Jacket": 7, "DVD Jacket": 8, "Cover Art": 9,
	}
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
	rowData := []any{
		"2021-06-01", "Test Message", "A description",
		"Speaker One", "message", "public",
		"", "", "", "", "",
	}

	msg, err := newCatalogMessageFromRow(msgColumnsForTest(), rowData, "wol")
	t.NoError(err)
	t.Nil(msg.Thumb)
}

func (t *CatalogTestSuite) TestNewCatalogMessageFromRow_DefaultMinistry() {
	rowData := []any{
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
	rowWithEmptyMinistry := append(append([]any{}, rowData...), "")

	msg, err = newCatalogMessageFromRow(columnsWithMinistry, rowWithEmptyMinistry, "wol")
	t.NoError(err)
	t.Equal(catalog.WordOfLife, msg.Ministry)
}

func (t *CatalogTestSuite) TestNewCatalogMessageFromRow_MinistryFromColumn() {
	// Ministry column present with non-empty value → overrides defaultMinistry
	columns := msgColumnsForTest()
	columns["Ministry"] = 11
	rowData := []any{
		"2021-06-01", "Test Message", "",
		"", "message", "public",
		"", "", "", "", "", "tbo",
	}

	msg, err := newCatalogMessageFromRow(columns, rowData, "wol")
	t.NoError(err)
	t.Equal(catalog.TheBridgeOutreach, msg.Ministry)
}

func (t *CatalogTestSuite) TestNewCatalogMessageFromRow_MultipleSpeakers() {
	rowData := []any{
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
	rowData := []any{
		"not-a-date", "Test Message", "",
		"", "message", "public",
		"", "", "", "", "",
	}

	msg, err := newCatalogMessageFromRow(msgColumnsForTest(), rowData, "wol")
	t.NoError(err)
	t.True(msg.Date.IsZero())
}

// +---------------------------------------------------------------------------
// | Integration tests (require Google API)
// +---------------------------------------------------------------------------

func (t *CatalogTestSuite) TestReadColumns() {
	// when
	columns, err := getIndexOfColumns(t.service, testDocumentID, "Columns", 1)
	t.NoError(err)

	// then
	t.NotNil(columns)
	t.Len(columns, 5)
	t.Equal(0, columns["One"])
	t.Equal(1, columns["Two"])
	t.Equal(2, columns["And-a-Three"])
	t.Equal(3, columns["Four Space"])
	t.Equal(4, columns["!# Five †"])
}

func (t *CatalogTestSuite) TestReadMessageSheet() {
	// when
	msgs, series, err := readMessagesFromSheet(t.service, testDocumentID, "Messages", "Messages")
	t.NoError(err)
	_ = series

	// fully validate the first message
	sut := msgs[0]
	t.Equal(catalog.MustParseDateOnly("2021-09-01"), sut.Date)
	t.Equal("Message Name", sut.Name)
	t.Equal("Generic description", sut.Description)
	t.Len(sut.Speakers, 1)
	t.Equal("Speaker Jones", sut.Speakers[0])
	t.Equal(catalog.WordOfLife, sut.Ministry)
	t.Equal(catalog.Message, sut.Type)
	t.Equal(catalog.Public, sut.Visibility)
	t.Equal(catalog.OnlineResource{URL: "https://youtu.be/c/blahtyblah", Name: "blahtyblah"}, *sut.Video)

	// validate resources
	sut = msgs[8]
	t.Equal("Resources", sut.Name)
	t.Len(sut.Resources, 2)
	t.Equal(catalog.OnlineResource{URL: "http://url-one.pdf", Name: "url-one"}, sut.Resources[0])
	t.Equal(catalog.OnlineResource{URL: "https://url-two.pdf", Name: "url-two"}, sut.Resources[1])

	// validate series reference
	sut = msgs[9]
	t.Equal("One Series", sut.Name)
	t.Len(sut.Series, 1)
	t.Equal(catalog.SeriesReference{Name: "Serical", Index: 4}, sut.Series[0])

	// validate multiple series references
	sut = msgs[10]
	t.Equal("Multiple Series", sut.Name)
	t.Len(sut.Series, 2)
	t.Equal(catalog.SeriesReference{Name: "Prayer", Index: 1}, sut.Series[0])
	t.Equal(catalog.SeriesReference{Name: "Grace", Index: 12}, sut.Series[1])

	// validate series references with missing index
	sut = msgs[11]
	t.Equal("Series Missing Index", sut.Name)
	t.Len(sut.Series, 3)
	t.Equal(catalog.SeriesReference{Name: "Prayer", Index: 2}, sut.Series[0])
	t.Equal(catalog.SeriesReference{Name: "Grace", Index: 2}, sut.Series[1])
	t.Equal(catalog.SeriesReference{Name: "Love", Index: 2}, sut.Series[2])
}

func (t *CatalogTestSuite) TestReadMessagesFromDocument() {
	// when
	messages, series, err := readMessagesFromDocument(t.service, testDocumentID)

	// then
	t.NoError(err)
	t.NotEmpty(messages)
	// every series extracted from message tabs must have a hash-based ID
	for _, s := range series {
		t.Equal(util.ComputeHash(s.Name), s.ID,
			"series %q should have a hash-based ID", s.Name)
	}
}

func (t *CatalogTestSuite) TestNewCatalogFromSheet() {
	// when
	cat, err := NewCatalogFromSheet(t.service, testDocumentID)

	// then
	t.NoError(err)
	t.NotNil(cat)
	t.NotEmpty(cat.Messages)

	// Series are no longer read from a separate tab, so this fixture produces none:
	// every row in its message tabs is a plain message. Series now come from rows
	// typed Series or Booklet, which is covered by
	// TestNewCatalogSeriFromMessageRow_Series and _Booklet, and the IDs they get are
	// checked against the live sheet by TestReadMessagesFromDocument.
	for _, s := range cat.Series {
		t.NotEmpty(s.ID, "series %q should have an ID", s.Name)
	}
}
