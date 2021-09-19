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
