package google

import (
	"context"
	"testing"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/sheet"
	"github.com/WordOfLifeMN/online/util"
	"github.com/stretchr/testify/suite"
)

// These tests read a small Google Sheet kept for the purpose, so they need the
// credentials in ~/.wolm/credentials.json. The row parsing itself is tested without
// Google, in package sheet.

const testDocumentID = "1vvhIGMPvVF-DtWoYsEbVBvzk_VtLyKuIw_zyLdsB-JY"

func TestSpreadsheetTestSuite(t *testing.T) {
	suite.Run(t, new(SpreadsheetTestSuite))
}

type SpreadsheetTestSuite struct {
	suite.Suite
	src *Spreadsheet
}

func (t *SpreadsheetTestSuite) SetupSuite() {
	var err error
	t.src, err = Open(context.Background(), testDocumentID)
	t.Require().NoError(err)
}

// oneTab narrows a Source to a single tab
type oneTab struct {
	sheet.Source
	tab string
}

func (o oneTab) Tabs() ([]string, error) { return []string{o.tab}, nil }

func (t *SpreadsheetTestSuite) TestToStrings() {
	rows := toStrings([][]any{{"hello", 42, 1.5}, {}})
	t.Equal([][]string{{"hello", "42", "1.5"}, {}}, rows)
}

func (t *SpreadsheetTestSuite) TestReadColumns() {
	// when
	rows, err := t.src.Rows("Columns")
	t.Require().NoError(err)

	// then
	t.Require().NotEmpty(rows)
	t.Equal([]string{"One", "Two", "And-a-Three", "Four Space", "!# Five †"}, rows[0])
}

func (t *SpreadsheetTestSuite) TestReadMessageSheet() {
	// when
	cat, err := sheet.NewCatalog(oneTab{t.src, "Messages"})
	t.Require().NoError(err)
	msgs := cat.Messages

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

func (t *SpreadsheetTestSuite) TestNewCatalog() {
	// when
	cat, err := sheet.NewCatalog(t.src)

	// then
	t.Require().NoError(err)
	t.NotEmpty(cat.Messages)

	// every series extracted from message tabs must have a hash-based ID
	for _, s := range cat.Series {
		t.Equal(util.ComputeHash(s.Name), s.ID,
			"series %q should have a hash-based ID", s.Name)
	}
}
