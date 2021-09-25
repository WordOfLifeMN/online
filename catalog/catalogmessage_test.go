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
	// given
	sut := CatalogMessage{
		Audio: "http://path/file.mp3",
	}

	// when-then
	t.NoError(sut.initialize())
	t.Equal("http://path/file.mp3", sut.Audio)

	// when-then
	sut.Audio = ""
	t.NoError(sut.initialize())
	t.Equal("", sut.Audio)

	// when-then
	sut.Audio = "in progress"
	t.NoError(sut.initialize())
	t.Equal("", sut.Audio)

	// when-then
	sut.Audio = "-"
	t.NoError(sut.initialize())
	t.Equal("", sut.Audio)
}

func (t *CatalogMessageTestSuite) TestInitializeVideo() {
	// given
	sut := CatalogMessage{
		Video: "http://path/file.mp4",
	}

	// when-then
	t.NoError(sut.initialize())
	t.Equal("http://path/file.mp4", sut.Video)

	// when-then
	sut.Video = ""
	t.NoError(sut.initialize())
	t.Equal("", sut.Video)

	// when-then
	sut.Video = "in progress"
	t.NoError(sut.initialize())
	t.Equal("", sut.Video)

	// when-then
	sut.Video = "-"
	t.NoError(sut.initialize())
	t.Equal("", sut.Video)
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
