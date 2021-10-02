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
		Name:     "SERIES",
		View:     Public,
		Speakers: []string{"Frodo", "Sam"},
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
	t.Len(sut.allSpeakers, 5)
	t.Equal("Frodo", sut.allSpeakers[0])
	t.Equal("Sam", sut.allSpeakers[1])
	t.Equal("Tim", sut.allSpeakers[2])
	t.Equal("Ollie", sut.allSpeakers[3])
	t.Equal("Sven", sut.allSpeakers[4])
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
	t.Len(sut.allResources, 4)
	t.Equal("Series Notes", sut.allResources[0].Name)      // keep notes from series
	t.Equal("First Study Notes", sut.allResources[1].Name) // notes from 1st message
	t.Equal("Sidetrack", sut.allResources[2].Name)         // unique notes from 2nd message
	t.Equal("Skizzle", sut.allResources[3].Name)           // note from 0th message
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

// +---------------------------------------------------------------------------
// | Filters
// +---------------------------------------------------------------------------

func (t *CatalogSeriTestSuite) TestFilterByMinistry_Empty() {
	corpus := []CatalogSeri{}
	t.Nil(FilterSeriesByMinistry(corpus, WordOfLife))
}

func (t *CatalogSeriTestSuite) TestFilterByMinistry_None() {
	// given
	corpus := []CatalogSeri{
		{
			Name: "SERIES-1",
			messages: []CatalogMessage{
				{Name: "MSG-A", Ministry: WordOfLife},
			},
		},
		{
			Name: "SERIES-2",
			messages: []CatalogMessage{
				{Name: "MSG-B", Ministry: WordOfLife},
			},
		},
	}

	// then
	t.Nil(FilterSeriesByMinistry(corpus, TheBridgeOutreach))
}

func (t *CatalogSeriTestSuite) TestFilterByMinistry() {
	// given
	corpus := []CatalogSeri{
		{
			Name: "SERIES-1",
			messages: []CatalogMessage{
				{Name: "MSG-A", Ministry: WordOfLife},
			},
		},
		{
			Name: "SERIES-2",
			messages: []CatalogMessage{
				{Name: "MSG-B", Ministry: AskThePastor},
			},
		},
		{
			Name: "SERIES-3",
			messages: []CatalogMessage{
				{Name: "MSG-C", Ministry: WordOfLife},
			},
		},
	}

	// when
	result := FilterSeriesByMinistry(corpus, WordOfLife)

	// then
	t.Len(result, 2)
	t.Equal("SERIES-1", result[0].Name)
	t.Equal("SERIES-3", result[1].Name)
}
