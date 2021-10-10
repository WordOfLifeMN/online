package catalog

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type CatalogMessageTestSuite struct {
	suite.Suite
	VariableThatShouldStartAtFive int
}

// Runs the test suite as a test
func TestCatalogMessageTestSuite(t *testing.T) {
	suite.Run(t, new(CatalogMessageTestSuite))
}

// +---------------------------------------------------------------------------
// | Constructors
// +---------------------------------------------------------------------------

func (t *CatalogMessageTestSuite) TestInitializeAudio() {
	// when-then
	sut := CatalogMessage{Audio: "http://path/file.mp3"}
	t.NoError(sut.Initialize())
	t.Equal("http://path/file.mp3", sut.Audio)

	// when-then
	sut = CatalogMessage{Audio: ""}
	t.NoError(sut.Initialize())
	t.Equal("", sut.Audio)

	// when-then
	sut = CatalogMessage{Audio: "in progress"}
	t.NoError(sut.Initialize())
	t.Equal("", sut.Audio)

	// when-then
	sut = CatalogMessage{Audio: "-"}
	t.NoError(sut.Initialize())
	t.Equal("", sut.Audio)

	// when-then
	sut = CatalogMessage{Audio: "exporting"}
	t.NoError(sut.Initialize())
	t.Equal("", sut.Audio)
}

func (t *CatalogMessageTestSuite) TestInitializeVideo() {
	// when-then
	sut := CatalogMessage{Video: "http://path/file.mp4"}
	t.NoError(sut.Initialize())
	t.Equal("http://path/file.mp4", sut.Video)

	// when-then
	sut = CatalogMessage{Video: ""}
	t.NoError(sut.Initialize())
	t.Equal("", sut.Video)

	// when-then
	sut = CatalogMessage{Video: "in progress"}
	t.NoError(sut.Initialize())
	t.Equal("", sut.Video)

	// when-then
	sut = CatalogMessage{Video: "-"}
	t.NoError(sut.Initialize())
	t.Equal("", sut.Video)

	// when-then
	sut = CatalogMessage{Video: "uploading"}
	t.NoError(sut.Initialize())
	t.Equal("", sut.Video)
}

func (t *CatalogMessageTestSuite) TestCopy() {
	msg := CatalogMessage{
		Date:        MustParseDateOnly("2021-02-03"),
		Name:        "MESSAGE",
		Description: "DESCRIPTION",
		Speakers:    []string{"VERN", "MARY"},
		Ministry:    WordOfLife,
		Type:        Message,
		Visibility:  Public,
		Series:      []SeriesReference{{Name: "SERIES", Index: 12}},
		Playlist:    []string{"Service"},
		Audio:       "https://audio.mp3",
		Video:       "https://video.mp4",
		Resources:   []OnlineResource{{URL: "https://yes.pdf", Name: "Yes", thumbnail: "https://thumb", classifier: "pdf"}},
		initialized: true,
	}

	cpy := msg.Copy()
	t.Equal(msg.Date, cpy.Date)
	t.Equal(msg.initialized, cpy.initialized)
	t.NotSame(msg.Speakers, cpy.Speakers)
	t.NotSame(msg.Playlist, cpy.Playlist)
	t.NotSame(msg.Series, cpy.Series)
	t.NotSame(&msg.Resources, &cpy.Resources)

}

// +---------------------------------------------------------------------------
// | Accessors
// +---------------------------------------------------------------------------

func (t *CatalogMessageTestSuite) TestSpeakerNames() {
	// given
	sut := CatalogMessage{
		Speakers: []string{"Sven ", " Ollie"},
	}

	// when-then
	t.NoError(sut.Initialize())
	t.Equal("Sven", sut.Speakers[0])
	t.Equal("Ollie", sut.Speakers[1])
}

func (t *CatalogMessageTestSuite) TestSpeakerNames_Vern() {
	// given
	sut := CatalogMessage{
		Speakers: []string{"Vern ", " vern", "Vern Peltz"},
	}

	// when-then
	t.NoError(sut.Initialize())
	t.Equal("Pastor Vern Peltz", sut.Speakers[0])
	t.Equal("Pastor Vern Peltz", sut.Speakers[1])
	t.Equal("Pastor Vern Peltz", sut.Speakers[2])
}

func (t *CatalogMessageTestSuite) TestSpeakerNames_Dave() {
	// given
	sut := CatalogMessage{
		Speakers: []string{"Dave ", " dave", "Dave Warren"},
	}

	// when-then
	t.NoError(sut.Initialize())
	t.Equal("Pastor Dave Warren", sut.Speakers[0])
	t.Equal("Pastor Dave Warren", sut.Speakers[1])
	t.Equal("Pastor Dave Warren", sut.Speakers[2])
}

func (t *CatalogMessageTestSuite) TestSpeakerNames_MaryWOL() {
	// given
	sut := CatalogMessage{
		Speakers: []string{"Mary ", " mary", "Mary Peltz"},
	}

	// when-then
	t.NoError(sut.Initialize())
	t.Equal("Pastor Mary Peltz", sut.Speakers[0])
	t.Equal("Pastor Mary Peltz", sut.Speakers[1])
	t.Equal("Pastor Mary Peltz", sut.Speakers[2])
}

func (t *CatalogMessageTestSuite) TestSpeakerNames_MaryCORE() {
	// given
	sut := CatalogMessage{
		Ministry: CenterOfRelationshipExperience,
		Speakers: []string{"Mary ", " mary", "Mary Peltz", "Pastor Mary Peltz"},
	}

	// when-then
	t.NoError(sut.Initialize())
	t.Equal("Mary Peltz", sut.Speakers[0])
	t.Equal("Mary Peltz", sut.Speakers[1])
	t.Equal("Mary Peltz", sut.Speakers[2])
	t.Equal("Mary Peltz", sut.Speakers[3])
}

// +---------------------------------------------------------------------------
// | Audio checks
// +---------------------------------------------------------------------------

func (t *CatalogMessageTestSuite) TestGetAudioSize_NoAudio() {
	// given
	sut := CatalogMessage{
		Name:  "MESSAGE",
		Audio: "-",
	}
	// then
	t.Equal(0, sut.GetAudioSize())
}

func (t *CatalogMessageTestSuite) TestGetAudioSize_IllegalReference() {
	// given
	sut := CatalogMessage{
		Name:  "MESSAGE",
		Audio: "https://s3-us-west-2.amazonaws.com/wordoflife.mn.audio/2000/no-such-file.mp3",
	}
	// then
	t.Equal(-1, sut.GetAudioSize())
}

func (t *CatalogMessageTestSuite) TestGetAudioSize_ValidFile() {
	// given
	sut := CatalogMessage{
		Name:  "MESSAGE",
		Audio: "https://s3-us-west-2.amazonaws.com/wordoflife.mn.audio/2020/2020-10-11+Finding+and+Correcting+Fear%2C+Part+1.mp3",
	}
	// then
	t.Equal(48458231, sut.GetAudioSize())
}

// +---------------------------------------------------------------------------
// | Queries
// +---------------------------------------------------------------------------

func (t *CatalogMessageTestSuite) TestGetSeriesReference() {
	// given
	sut := CatalogMessage{
		Name: "MESSAGE",
		Series: []SeriesReference{
			{Name: "SERIES", Index: 2},
			{Name: "OTHER", Index: 12},
		},
	}

	ref := sut.FindSeriesReference("OTHER")
	t.NotNil(ref)
	t.Equal(12, ref.Index)
	t.True(sut.IsInSeries("OTHER"))
}

func (t *CatalogMessageTestSuite) TestGetSeriesReference_Missing() {
	// given
	sut := CatalogMessage{
		Name: "MESSAGE",
		Series: []SeriesReference{
			{Name: "SERIES", Index: 2},
			{Name: "OTHER", Index: 2},
		},
	}

	ref := sut.FindSeriesReference("NUNSUCH")
	t.Nil(ref)
	t.False(sut.IsInSeries("NUNSUCH"))
}

func (t *CatalogMessageTestSuite) TestGetSeriesReference_IgnoresCase() {
	// given
	sut := CatalogMessage{
		Name: "MESSAGE",
		Series: []SeriesReference{
			{Name: "SERIES", Index: 2},
			{Name: "OTHER", Index: 2},
		},
	}

	ref := sut.FindSeriesReference("other")
	t.NotNil(ref)
	t.True(sut.IsInSeries("other"))
}
