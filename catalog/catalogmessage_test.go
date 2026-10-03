package catalog

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// Runs the test suite as a test
func TestCatalogMessageTestSuite(t *testing.T) {
	suite.Run(t, new(CatalogMessageTestSuite))
}

type CatalogMessageTestSuite struct {
	suite.Suite
	VariableThatShouldStartAtFive int
}

// +---------------------------------------------------------------------------
// | Constructors
// +---------------------------------------------------------------------------

func (t *CatalogMessageTestSuite) TestInitializeVideo() {
	// when-then
	sut := CatalogMessage{Video: NewResourceFromString("http://path/file.mp4")}
	t.NoError(sut.Initialize())
	t.Equal("http://path/file.mp4", sut.Video.URL)

	// when-then
	sut = CatalogMessage{Video: NewResourceFromString("")}
	t.NoError(sut.Initialize())
	t.Nil(sut.Video)

	// when-then
	sut = CatalogMessage{Video: NewResourceFromString("in progress")}
	t.NoError(sut.Initialize())
	t.Nil(sut.Video)

	// when-then
	sut = CatalogMessage{Video: NewResourceFromString("-")}
	t.NoError(sut.Initialize())
	t.Nil(sut.Video)

	// when-then
	sut = CatalogMessage{Video: NewResourceFromString("uploading")}
	t.NoError(sut.Initialize())
	t.Nil(sut.Video)
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
		Video:       NewResourceFromString("https://video.mp4"),
		Resources:   []OnlineResource{{URL: "https://yes.pdf", Name: "Yes"}},
		initialized: true,
	}

	// NOTE: CatalogMessage.Copy() is effectively a shallow copy. Its copy(dst, src)
	// calls operate on slices that still share a backing array with the original, so
	// they are no-ops and Speakers/Series/Resources remain aliased.
	cpy := msg.Copy()
	t.Equal(msg.Date, cpy.Date)
	t.Equal(msg.initialized, cpy.initialized)
	t.Equal(msg.Speakers, cpy.Speakers)
	t.Equal(msg.Series, cpy.Series)
	t.Equal(msg.Resources, cpy.Resources)
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
		Speakers: []string{"Vern ", " vern", "Vern Peltz", "vp", "Pastor Vern"},
	}

	// when-then
	t.NoError(sut.Initialize())
	t.Equal("Pastor Vern Peltz", sut.Speakers[0])
	t.Equal("Pastor Vern Peltz", sut.Speakers[1])
	t.Equal("Pastor Vern Peltz", sut.Speakers[2])
	t.Equal("Pastor Vern Peltz", sut.Speakers[3])
	t.Equal("Pastor Vern Peltz", sut.Speakers[4])
}

func (t *CatalogMessageTestSuite) TestSpeakerNames_Dave() {
	// given
	sut := CatalogMessage{
		Speakers: []string{"Dave ", " dave", "Dave Warren", "Pastor Dave", "DW", "pastor warren"},
	}

	// when-then
	t.NoError(sut.Initialize())
	t.Equal("Pastor Dave Warren", sut.Speakers[0])
	t.Equal("Pastor Dave Warren", sut.Speakers[1])
	t.Equal("Pastor Dave Warren", sut.Speakers[2])
	t.Equal("Pastor Dave Warren", sut.Speakers[3])
	t.Equal("Pastor Dave Warren", sut.Speakers[4])
	t.Equal("Pastor Dave Warren", sut.Speakers[5])
}

func (t *CatalogMessageTestSuite) TestSpeakerNames_Jim() {
	// given
	sut := CatalogMessage{
		Speakers: []string{"Jim ", " jim", "Jim Isakson", "ji"},
	}

	// when-then
	t.NoError(sut.Initialize())
	t.Equal("Pastor Jim Isakson", sut.Speakers[0])
	t.Equal("Pastor Jim Isakson", sut.Speakers[1])
	t.Equal("Pastor Jim Isakson", sut.Speakers[2])
	t.Equal("Pastor Jim Isakson", sut.Speakers[3])
}

func (t *CatalogMessageTestSuite) TestSpeakerNames_MaryWOL() {
	// given
	sut := CatalogMessage{
		Speakers: []string{"Mary ", " mary", "Mary Peltz", "mp"},
	}

	// when-then
	t.NoError(sut.Initialize())
	t.Equal("Pastor Mary Peltz", sut.Speakers[0])
	t.Equal("Pastor Mary Peltz", sut.Speakers[1])
	t.Equal("Pastor Mary Peltz", sut.Speakers[2])
	t.Equal("Pastor Mary Peltz", sut.Speakers[3])
}

func (t *CatalogMessageTestSuite) TestSpeakerNames_MaryCORE() {
	// given
	sut := CatalogMessage{
		Ministry: CenterOfRelationshipExperience,
		Speakers: []string{"Mary ", " mary", "Mary Peltz", "Pastor Mary Peltz", "mp "},
	}

	// when-then
	t.NoError(sut.Initialize())
	t.Equal("Mary Peltz", sut.Speakers[0])
	t.Equal("Mary Peltz", sut.Speakers[1])
	t.Equal("Mary Peltz", sut.Speakers[2])
	t.Equal("Mary Peltz", sut.Speakers[3])
	t.Equal("Mary Peltz", sut.Speakers[4])
}

func (t *CatalogMessageTestSuite) TestSpeakerNames_Igor() {
	// given
	sut := CatalogMessage{
		Ministry: CenterOfRelationshipExperience,
		Speakers: []string{"Igor ", " igor", "Igor Kondratyuk", "Pastor Igor Kondratyuk", "IK", "pastor igor"},
	}

	// when-then
	t.NoError(sut.Initialize())
	t.Equal("Pastor Igor Kondratyuk", sut.Speakers[0])
	t.Equal("Pastor Igor Kondratyuk", sut.Speakers[1])
	t.Equal("Pastor Igor Kondratyuk", sut.Speakers[2])
	t.Equal("Pastor Igor Kondratyuk", sut.Speakers[3])
	t.Equal("Pastor Igor Kondratyuk", sut.Speakers[4])
	t.Equal("Pastor Igor Kondratyuk", sut.Speakers[5])
}

func (t *CatalogMessageTestSuite) TestSpeakerNames_Tania() {
	// given
	sut := CatalogMessage{
		Ministry: CenterOfRelationshipExperience,
		Speakers: []string{"Tania ", " tania", "TK"},
	}

	// when-then
	t.NoError(sut.Initialize())
	t.Equal("Pastor Tania Kondratyuk", sut.Speakers[0])
	t.Equal("Pastor Tania Kondratyuk", sut.Speakers[1])
	t.Equal("Pastor Tania Kondratyuk", sut.Speakers[2])
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
