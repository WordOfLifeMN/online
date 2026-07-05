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

func (t *CatalogTestSuite) TestSeriesContainsName() {
	t.False(seriesContainsName(nil, "Alpha"))
	t.False(seriesContainsName([]catalog.CatalogSeri{{Name: "Beta"}}, "Alpha"))
	t.True(seriesContainsName([]catalog.CatalogSeri{{Name: "Alpha"}, {Name: "Beta"}}, "Alpha"))
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
	// resources go to Resources, not Booklets
	t.Len(seri.Resources, 1)
	t.Empty(seri.Booklets)
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

func (t *CatalogTestSuite) TestReadSeries() {
	// when
	series, err := readSeriesFromDocument(t.service, testDocumentID)
	t.NoError(err)

	// then
	t.Len(series, 3)

	// fully validate the first basic series
	sut := series[0]
	t.Equal("TEST-123", sut.ID)
	t.Equal("Public Series", sut.Name)
	t.Equal("A series that contains 3 public messages", sut.Description)
	t.Equal(catalog.MustParseDateOnly("2014-01-05"), sut.StartDate)
	t.Equal(catalog.MustParseDateOnly("2014-02-02"), sut.StopDate)
	t.Equal(catalog.Public, sut.Visibility)

	// validate booklet
	sut = series[1]
	t.Equal("Booklet", sut.Name)
	t.Len(sut.Booklets, 1)
	t.Equal(catalog.OnlineResource{URL: "http://book-one.pdf", Name: "book-one"}, sut.Booklets[0])

	// validate resources
	sut = series[2]
	t.Equal("Resources", sut.Name)
	t.Len(sut.Booklets, 0)
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
	t.Equal(catalog.OnlineResource{URL: "https://s3/2021/audio.mp3", Name: "audio"}, *sut.Audio)
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
