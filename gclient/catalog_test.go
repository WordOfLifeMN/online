package gclient

import (
	"context"
	"testing"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/stretchr/testify/assert"
)

const testDocumentID = "1vvhIGMPvVF-DtWoYsEbVBvzk_VtLyKuIw_zyLdsB-JY"

func TestReadColumns(t *testing.T) {
	// given
	service, err := GetSheetService(context.Background())
	assert.NoError(t, err)

	// when
	columns, err := getIndexOfColumns(service, testDocumentID, "Columns", 1)
	assert.NoError(t, err)

	// then
	assert.NotNil(t, columns)
	assert.Len(t, columns, 5)

	assert.Equal(t, 0, columns["One"])
	assert.Equal(t, 1, columns["Two"])
	assert.Equal(t, 2, columns["And-a-Three"])
	assert.Equal(t, 3, columns["Four Space"])
	assert.Equal(t, 4, columns["!# Five †"])
}

func TestReadSeries(t *testing.T) {
	// given
	service, err := GetSheetService(context.Background())
	assert.NoError(t, err)

	// when
	series, err := readSeriesFromDocument(service, testDocumentID)
	assert.NoError(t, err)

	// then

	// overall validation
	assert.Len(t, series, 3)

	// fully validate the first basic series
	sut := series[0]
	assert.Equal(t, "TEST-123", sut.ID)
	assert.Equal(t, "Public Series", sut.Name)
	assert.Equal(t, "A series that contains 3 public messages", sut.Description)
	assert.Equal(t, catalog.MustParseDateOnly("2014-01-05"), sut.StartDate)
	assert.Equal(t, catalog.MustParseDateOnly("2014-02-02"), sut.StopDate)
	assert.Equal(t, catalog.Public, sut.Visibility)

	// validate booklet
	sut = series[1]
	assert.Equal(t, "Booklet", sut.Name)
	assert.Len(t, sut.Booklets, 1)
	assert.Equal(t, catalog.OnlineResource{URL: "http://book-one.pdf", Name: "book-one"}, sut.Booklets[0])

	// validate resources
	sut = series[2]
	assert.Equal(t, "Resources", sut.Name)
	assert.Len(t, sut.Booklets, 0)
	assert.Len(t, sut.Resources, 2)
	assert.Equal(t, catalog.OnlineResource{URL: "http://url-one.pdf", Name: "url-one"}, sut.Resources[0])
	assert.Equal(t, catalog.OnlineResource{URL: "https://url-two.pdf", Name: "url-two"}, sut.Resources[1])
}

func TestReadMessageSheet(t *testing.T) {
	// given
	service, err := GetSheetService(context.Background())
	assert.NoError(t, err)

	// when
	msgs, err := readMessagesFromSheet(service, testDocumentID, "Messages")
	assert.NoError(t, err)

	// then

	// overall validation
	// assert.Len(t, msgs, 12)

	// fully validate the first basic series
	sut := msgs[0]
	assert.Equal(t, catalog.MustParseDateOnly("2021-09-01"), sut.Date)
	assert.Equal(t, "Message Name", sut.Name)
	assert.Equal(t, "Generic description", sut.Description)
	assert.Len(t, sut.Speakers, 1)
	assert.Equal(t, "Speaker Jones", sut.Speakers[0])
	assert.Equal(t, catalog.WordOfLife, sut.Ministry)
	assert.Equal(t, catalog.Message, sut.Type)
	assert.Equal(t, catalog.Public, sut.Visibility)
	assert.Equal(t, "https://s3/2021/audio.mp3", sut.Audio)
	assert.Equal(t, "https://youtu.be/c/blahtyblah", sut.Video)

	// validate playlists
	sut = msgs[7]
	assert.Equal(t, "Playlists", sut.Name)
	assert.Len(t, sut.Playlist, 2)
	assert.Equal(t, "service", sut.Playlist[0])
	assert.Equal(t, "prayer", sut.Playlist[1])

	// validate resources
	sut = msgs[8]
	assert.Equal(t, "Resources", sut.Name)
	assert.Len(t, sut.Resources, 2)
	assert.Equal(t, catalog.OnlineResource{URL: "http://url-one.pdf", Name: "url-one"}, sut.Resources[0])
	assert.Equal(t, catalog.OnlineResource{URL: "https://url-two.pdf", Name: "url-two"}, sut.Resources[1])

	// validate series
	sut = msgs[9]
	assert.Equal(t, "One Series", sut.Name)
	assert.Len(t, sut.Series, 1)
	assert.Equal(t, catalog.SeriesReference{Name: "Serical", Index: 4}, sut.Series[0])

	// validate serieses
	sut = msgs[10]
	assert.Equal(t, "Multiple Series", sut.Name)
	assert.Len(t, sut.Series, 2)
	assert.Equal(t, catalog.SeriesReference{Name: "Prayer", Index: 1}, sut.Series[0])
	assert.Equal(t, catalog.SeriesReference{Name: "Grace", Index: 12}, sut.Series[1])

	// validate serieses missing index
	sut = msgs[11]
	assert.Equal(t, "Series Missing Index", sut.Name)
	assert.Len(t, sut.Series, 3)
	assert.Equal(t, catalog.SeriesReference{Name: "Prayer", Index: 0}, sut.Series[0])
	assert.Equal(t, catalog.SeriesReference{Name: "Grace", Index: 1}, sut.Series[1])
	assert.Equal(t, catalog.SeriesReference{Name: "Love", Index: 0}, sut.Series[2])

}
