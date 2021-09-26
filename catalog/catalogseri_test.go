package catalog

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type CatalogSeriTestSuite struct {
	suite.Suite
}

// Runs the test suite as a test
func TestCatalogSeriTestSuite(t *testing.T) {
	suite.Run(t, new(CatalogSeriTestSuite))
}

// +---------------------------------------------------------------------------
// | Construction
// +---------------------------------------------------------------------------

// TODO - Normalize: no messages, start date, stop date, speakers, messages that are not
// relevant

func (t *CatalogSeriTestSuite) TestSeriesNormalization_Sorting() {
	// given
	sut := CatalogSeri{
		Name: "SERIES",
		View: Public,
		messages: []CatalogMessage{
			{
				Name:       "MSG2",
				Series:     []SeriesReference{{Name: "SERIES", Index: 2}},
				Visibility: Public,
			},
			{
				Name:       "MSG0",
				Series:     []SeriesReference{{Name: "SERIES", Index: 0}},
				Visibility: Public,
			},
			{
				Name:       "MSG1",
				Series:     []SeriesReference{{Name: "SERIES", Index: 1}},
				Visibility: Public,
			},
		},
	}

	// when
	sut.Normalize()

	// then
	t.Equal("MSG1", sut.messages[0].Name)
	t.Equal("MSG2", sut.messages[1].Name)
	t.Equal("MSG0", sut.messages[2].Name)
}

func (t *CatalogSeriTestSuite) TestSeriesNormalization_Dates() {
	// given
	sut := CatalogSeri{
		Name: "SERIES",
		View: Public,
		messages: []CatalogMessage{
			{
				Name:       "MSG2",
				Series:     []SeriesReference{{Name: "SERIES", Index: 2}},
				Date:       MustParseDateOnly("2021-06-08"),
				Visibility: Public,
			},
			{
				Name:       "MSG0",
				Series:     []SeriesReference{{Name: "SERIES", Index: 0}},
				Date:       MustParseDateOnly("2021-06-15"),
				Visibility: Public,
			},
			{
				Name:       "MSG1",
				Series:     []SeriesReference{{Name: "SERIES", Index: 1}},
				Date:       MustParseDateOnly("2021-06-01"),
				Visibility: Public,
			},
		},
	}

	// when
	sut.Normalize()

	// then
	t.Equal(MustParseDateOnly("2021-06-01"), sut.StartDate)
	t.Equal(MustParseDateOnly("2021-06-15"), sut.StopDate)
}

func (t *CatalogSeriTestSuite) TestSeriesNormalization_Speakers() {
	// given
	sut := CatalogSeri{
		Name: "SERIES",
		View: Public,
		messages: []CatalogMessage{
			{
				Name:       "MSG2",
				Series:     []SeriesReference{{Name: "SERIES", Index: 2}},
				Visibility: Public,
				Speakers:   []string{"Ollie", "Tim"},
			},
			{
				Name:       "MSG0",
				Series:     []SeriesReference{{Name: "SERIES", Index: 0}},
				Visibility: Public,
				Speakers:   []string{"Sven"},
			},
			{
				Name:       "MSG1",
				Series:     []SeriesReference{{Name: "SERIES", Index: 1}},
				Visibility: Public,
				Speakers:   []string{"Tim", "Sam"},
			},
		},
	}

	// when
	sut.Normalize()

	// then - names should be in order of index, with duplicates ignored
	t.Len(sut.speakers, 4)
	t.Equal("Tim", sut.speakers[0])
	t.Equal("Sam", sut.speakers[1])
	t.Equal("Ollie", sut.speakers[2])
	t.Equal("Sven", sut.speakers[3])
}

func (t *CatalogSeriTestSuite) TestSeriesNormalization_Resources() {
	// given
	sut := CatalogSeri{
		Name: "SERIES",
		View: Public,
		Resources: []OnlineResource{
			{URL: "https://series/notes.pdf", Name: "Series Notes"},
		},
		messages: []CatalogMessage{
			{
				Name:       "MSG2",
				Series:     []SeriesReference{{Name: "SERIES", Index: 2}},
				Visibility: Public,
				Resources: []OnlineResource{
					{URL: "https://notes", Name: "Second Study Notes"},
					{URL: "https://aside", Name: "Sidetrack"},
				},
			},
			{
				Name:       "MSG0",
				Series:     []SeriesReference{{Name: "SERIES", Index: 0}},
				Visibility: Public,
				Resources: []OnlineResource{
					{URL: "https://skizzle", Name: "Skizzle"},
				},
			},
			{
				Name:       "MSG1",
				Series:     []SeriesReference{{Name: "SERIES", Index: 1}},
				Visibility: Public,
				Resources: []OnlineResource{
					{URL: "https://notes", Name: "First Study Notes"},
				},
			},
		},
	}

	// when
	sut.Normalize()

	// then - names should be in order of index with duplicate URLs ignored
	t.Len(sut.Resources, 4)
	t.Equal("Series Notes", sut.Resources[0].Name)      // keep notes from series
	t.Equal("First Study Notes", sut.Resources[1].Name) // notes from 1st message
	t.Equal("Sidetrack", sut.Resources[2].Name)         // unique notes from 2nd message
	t.Equal("Skizzle", sut.Resources[3].Name)           // note from 0th message
}

// +---------------------------------------------------------------------------
// | Accessors
// +---------------------------------------------------------------------------

func (t *CatalogSeriTestSuite) TestSeriesID_NoMessage() {
	// given
	sut := CatalogSeri{
		Name:     "SERIES",
		messages: []CatalogMessage{},
	}
	// then
	t.Equal("", sut.GetID())
}

func (t *CatalogSeriTestSuite) TestSeriesID_UnknownMinistry() {
	// given
	sut := CatalogSeri{
		Name: "SERIES",
		messages: []CatalogMessage{
			{Name: "MESSAGE", Ministry: UnknownMinistry},
		},
	}
	// then
	t.Equal("ID-MTA1OTgwMDE3Ng", sut.GetID())
}

func (t *CatalogSeriTestSuite) TestSeriesID_WOL() {
	// given
	sut := CatalogSeri{
		Name: "SERIES",
		messages: []CatalogMessage{
			{Name: "MESSAGE", Ministry: WordOfLife},
		},
	}
	// then
	t.Equal("WOLS-MTA1OTgwMDE3Ng", sut.GetID())
}

func (t *CatalogSeriTestSuite) TestSeriesID_AskPastor() {
	// given
	sut := CatalogSeri{
		Name: "SERIES",
		messages: []CatalogMessage{
			{Name: "MESSAGE", Ministry: AskThePastor},
		},
	}
	// then
	t.Equal("ATP-MTA1OTgwMDE3Ng", sut.GetID())
}

func (t *CatalogSeriTestSuite) TestSeriesID_CORE() {
	// given
	sut := CatalogSeri{
		Name: "SERIES",
		messages: []CatalogMessage{
			{Name: "MESSAGE", Ministry: CenterOfRelationshipExperience},
		},
	}
	// then
	t.Equal("CORE-MTA1OTgwMDE3Ng", sut.GetID())
}

func (t *CatalogSeriTestSuite) TestSeriesID_FaithAndFreedom() {
	// given
	sut := CatalogSeri{
		Name: "SERIES",
		messages: []CatalogMessage{
			{Name: "MESSAGE", Ministry: FaithAndFreedom},
		},
	}
	// then
	t.Equal("FandF-MTA1OTgwMDE3Ng", sut.GetID())
}

func (t *CatalogSeriTestSuite) TestSeriesID_TBO() {
	// given
	sut := CatalogSeri{
		Name: "SERIES",
		messages: []CatalogMessage{
			{Name: "MESSAGE", Ministry: TheBridgeOutreach},
		},
	}
	// then
	t.Equal("TBO-MTA1OTgwMDE3Ng", sut.GetID())
}

func (t *CatalogSeriTestSuite) TestSeriesID_Explicit() {
	// given
	sut := CatalogSeri{
		Name: "SERIES",
		ID:   "MY-ID",
		messages: []CatalogMessage{
			{Name: "MESSAGE", Ministry: FaithAndFreedom},
		},
	}
	// then
	t.Equal("MY-ID", sut.GetID())
}

// +---------------------------------------------------------------------------
// | Queries
// +---------------------------------------------------------------------------

func (t *CatalogSeriTestSuite) TestBooklet() {
	// given a series with no booklet reference
	sut := CatalogSeri{
		Name: "SERIES",
	}

	// then
	t.False(sut.IsBooklet())

	// when the series has a booklet (but no message or ID)
	sut = CatalogSeri{
		Name: "SERIES",
		Booklets: []OnlineResource{
			{URL: "http://blah"},
		},
	}

	// then
	t.True(sut.IsBooklet())

	// when the series has an ID
	sut = CatalogSeri{
		Name: "SERIES",
		ID:   "MY-ID",
		Booklets: []OnlineResource{
			{URL: "http://blah"},
		},
	}

	// then
	t.False(sut.IsBooklet())

	// when the series has a message
	sut = CatalogSeri{
		Name: "SERIES",
		Booklets: []OnlineResource{
			{URL: "http://blah"},
		},
		messages: []CatalogMessage{
			{Name: "MESSAGE"},
		},
	}

	// then
	t.False(sut.IsBooklet())
}
