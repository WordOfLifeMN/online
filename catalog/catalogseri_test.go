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
		Messages: []CatalogMessage{
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
	t.Equal("MSG1", sut.Messages[0].Name)
	t.Equal("MSG2", sut.Messages[1].Name)
	t.Equal("MSG0", sut.Messages[2].Name)
}

func (t *CatalogSeriTestSuite) TestSeriesNormalization_Dates() {
	// given
	sut := CatalogSeri{
		Name: "SERIES",
		View: Public,
		Messages: []CatalogMessage{
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
		Messages: []CatalogMessage{
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
	t.Len(sut.AllSpeakers, 5)
	t.Equal("Frodo", sut.AllSpeakers[0])
	t.Equal("Sam", sut.AllSpeakers[1])
	t.Equal("Tim", sut.AllSpeakers[2])
	t.Equal("Ollie", sut.AllSpeakers[3])
	t.Equal("Sven", sut.AllSpeakers[4])
}

func (t *CatalogSeriTestSuite) TestSeriesNormalization_Resources() {
	// given
	sut := CatalogSeri{
		Name: "SERIES",
		View: Public,
		Resources: []OnlineResource{
			{URL: "https://series/notes.pdf", Name: "Series Notes"},
		},
		Messages: []CatalogMessage{
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
	t.Len(sut.AllResources, 4)
	t.Equal("Series Notes", sut.AllResources[0].Name)      // keep notes from series
	t.Equal("First Study Notes", sut.AllResources[1].Name) // notes from 1st message
	t.Equal("Sidetrack", sut.AllResources[2].Name)         // unique notes from 2nd message
	t.Equal("Skizzle", sut.AllResources[3].Name)           // note from 0th message
}

func (t *CatalogSeriTestSuite) TestCopy() {
	seri := CatalogSeri{
		ID:           "ID",
		Name:         "SERIES",
		Description:  "DESCRIPTION",
		Speakers:     []string{"VERN", "MARY"},
		Booklets:     []OnlineResource{{URL: "https://thing.pdf", Name: "Thing", thumbnail: "https://thumb.jpg", classifier: "pdf"}},
		Resources:    []OnlineResource{{URL: "https://thing.pdf", Name: "Thing", thumbnail: "https://thumb.jpg", classifier: "pdf"}},
		Visibility:   Public,
		Jacket:       "https://jacket.pdf",
		Thumbnail:    "https://thumb.jpg",
		StartDate:    MustParseDateOnly("2021-02-03"),
		StopDate:     MustParseDateOnly("2021-05-14"),
		View:         Public,
		Messages:     []CatalogMessage{},
		AllSpeakers:  []string{"Vern"},
		AllResources: []OnlineResource{{URL: "https://thing.pdf", Name: "Thing", thumbnail: "https://thumb.jpg", classifier: "pdf"}},
		initialized:  true,
	}

	cpy := seri.Copy()

	t.Equal(seri.Name, cpy.Name)
	t.NotSame(seri.Speakers, cpy.Speakers)
	t.NotSame(seri.Booklets, cpy.Booklets)
	t.NotSame(seri.Resources, cpy.Resources)
	t.NotSame(seri.Messages, cpy.Messages)
	t.NotSame(seri.AllSpeakers, cpy.AllSpeakers)
	t.NotSame(seri.AllResources, cpy.AllResources)
}

// +---------------------------------------------------------------------------
// | Accessors
// +---------------------------------------------------------------------------

func (t *CatalogSeriTestSuite) TestSeriesID_NoMessage() {
	// given
	sut := CatalogSeri{
		Name:     "SERIES",
		Messages: []CatalogMessage{},
	}
	// then
	t.Equal("", sut.GetID())
}

func (t *CatalogSeriTestSuite) TestSeriesID_UnknownMinistry() {
	// given
	sut := CatalogSeri{
		Name: "SERIES",
		Messages: []CatalogMessage{
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
		Messages: []CatalogMessage{
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
		Messages: []CatalogMessage{
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
		Messages: []CatalogMessage{
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
		Messages: []CatalogMessage{
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
		Messages: []CatalogMessage{
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
		Messages: []CatalogMessage{
			{Name: "MESSAGE", Ministry: FaithAndFreedom},
		},
	}
	// then
	t.Equal("MY-ID", sut.GetID())
}

func (t *CatalogSeriTestSuite) TestSeriesViewID() {
	// given
	sut := CatalogSeri{
		Name: "SERIES",
		Messages: []CatalogMessage{
			{Name: "MESSAGE", Ministry: WordOfLife},
		},
	}

	// when
	baseID := sut.GetID()
	publicID := sut.GetViewID(Public)
	partnerID := sut.GetViewID(Partner)
	privateID := sut.GetViewID(Private)

	// then
	t.Equal(baseID, publicID)
	t.NotEqual(publicID, partnerID)
	t.NotEqual(publicID, privateID)
	t.NotEqual(partnerID, privateID)
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
		Messages: []CatalogMessage{
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
			Messages: []CatalogMessage{
				{Name: "MSG-A", Ministry: WordOfLife},
			},
		},
		{
			Name: "SERIES-2",
			Messages: []CatalogMessage{
				{Name: "MSG-B", Ministry: WordOfLife},
			},
		},
	}

	// then
	t.Nil(FilterSeriesByMinistry(corpus, TheBridgeOutreach))
}

// func (t *CatalogSeriTestSuite) TestFilterByMinistry() {
// 	// given
// 	corpus := []CatalogSeri{
// 		{
// 			Name: "SERIES-1",
// 			Messages: []CatalogMessage{
// 				{Name: "MSG-A", Ministry: WordOfLife},
// 			},
// 		},
// 		{
// 			Name: "SERIES-2",
// 			Messages: []CatalogMessage{
// 				{Name: "MSG-B", Ministry: AskThePastor},
// 			},
// 		},
// 		{
// 			Name: "SERIES-3",
// 			Messages: []CatalogMessage{
// 				{Name: "MSG-C", Ministry: WordOfLife},
// 			},
// 		},
// 	}

// 	// when
// 	result := FilterSeriesByMinistry(corpus, WordOfLife)

// 	// then
// 	t.Len(result, 2)
// 	t.Equal("SERIES-1", result[0].Name)
// 	t.Equal("SERIES-3", result[1].Name)
// }

func (t *CatalogSeriTestSuite) TestFilterByMinistry() {
	cat, err := NewCatalogFromJSON("../testdata/small-catalog.json")
	t.NoError(err)
	t.NoError(cat.Initialize())

	// when
	result := FilterSeriesByMinistry(cat.Series, WordOfLife)

	// then
	t.Len(result, 3)
	t.Equal("A People of Conviction, or Convenience?", result[0].Name)
	t.Equal("Summerfest 2014", result[1].Name)
	t.Equal("Word (Feb 2, 2014)", result[2].Name)

	// when
	result = FilterSeriesByMinistry(cat.Series, CenterOfRelationshipExperience)

	// then
	t.Len(result, 1)
	t.Equal("Attachment Disorder", result[0].Name)

	// when
	result = FilterSeriesByMinistry(cat.Series, AskThePastor)

	// then
	t.Len(result, 0)
}

func (t *CatalogSeriTestSuite) TestFilterByView_Series() {
	// given
	corpus := []CatalogSeri{
		{
			Name:       "SERIES-1",
			Visibility: Public,
			Messages: []CatalogMessage{
				{Name: "MSG-A", Ministry: WordOfLife, Visibility: Public},
			},
		},
		{
			Name:       "SERIES-2",
			Visibility: Partner,
			Messages: []CatalogMessage{
				{Name: "MSG-B", Ministry: AskThePastor, Visibility: Public},
			},
		},
		{
			Name:       "SERIES-3",
			Visibility: Private,
			Messages: []CatalogMessage{
				{Name: "MSG-C", Ministry: WordOfLife, Visibility: Public},
			},
		},
		{
			Name:       "SERIES-4",
			Visibility: Raw,
			Messages: []CatalogMessage{
				{Name: "MSG-D", Ministry: WordOfLife, Visibility: Public},
			},
		},
	}

	// when, then
	result := FilterSeriesByView(corpus, Public)
	t.Len(result, 1)
	t.Equal("SERIES-1", result[0].Name)

	// when, then
	result = FilterSeriesByView(corpus, Partner)
	t.Len(result, 2)
	t.Equal("SERIES-1", result[0].Name)
	t.Equal("SERIES-2", result[1].Name)

	// when, then
	result = FilterSeriesByView(corpus, Private)
	t.Len(result, 3)
	t.Equal("SERIES-1", result[0].Name)
	t.Equal("SERIES-2", result[1].Name)
	t.Equal("SERIES-3", result[2].Name)
}

func (t *CatalogSeriTestSuite) TestFilterByView_SeriesWithoutVisibleMessages() {
	// given
	corpus := []CatalogSeri{
		{
			Name:       "SERIES-1",
			Visibility: Public,
			Messages: []CatalogMessage{
				{Name: "MSG-A", Ministry: WordOfLife, Visibility: Raw},
				{Name: "MSG-B", Ministry: WordOfLife, Visibility: Private},
			},
		},
		{
			Name:       "SERIES-2",
			Visibility: Public,
			Messages: []CatalogMessage{
				{Name: "MSG-C", Ministry: AskThePastor, Visibility: Partner},
				{Name: "MSG-D", Ministry: AskThePastor, Visibility: Partner},
			},
		},
	}

	// when, then
	result := FilterSeriesByView(corpus, Public)
	t.Len(result, 0)
}

func (t *CatalogSeriTestSuite) TestFilterByView_OnlyVisibleMessagesIncluded() {
	// given
	corpus := []CatalogSeri{
		{
			Name:       "SERIES-1",
			Visibility: Public,
			Messages: []CatalogMessage{
				{Name: "MSG-A", Ministry: WordOfLife, Visibility: Public},
				{Name: "MSG-B", Ministry: WordOfLife, Visibility: Private},
			},
		},
		{
			Name:       "SERIES-2",
			Visibility: Public,
			Messages: []CatalogMessage{
				{Name: "MSG-C", Ministry: AskThePastor, Visibility: Partner},
				{Name: "MSG-D", Ministry: AskThePastor, Visibility: Partner},
			},
		},
	}

	// when, then
	result := FilterSeriesByView(corpus, Public)
	t.Len(result, 1)
	t.Equal("SERIES-1", result[0].Name)
	t.Len(result[0].Messages, 1)
	t.Equal("MSG-A", result[0].Messages[0].Name)

	// when, then
	result = FilterSeriesByView(corpus, Partner)
	t.Len(result, 2)

	t.Equal("SERIES-1", result[0].Name)
	t.Len(result[0].Messages, 1)
	t.Equal("MSG-A", result[0].Messages[0].Name)

	t.Equal("SERIES-2", result[1].Name)
	t.Equal("MSG-C", result[1].Messages[0].Name)
	t.Equal("MSG-D", result[1].Messages[1].Name)
}

func (t *CatalogSeriTestSuite) TestFilterByView_SeriesAreInitialized() {
	// given
	corpus := []CatalogSeri{
		{
			Name:       "SERIES-1",
			Visibility: Public,
			Speakers:   []string{"NANA"},
			StartDate:  MustParseDateOnly("2001-01-01"),
			StopDate:   MustParseDateOnly("2001-01-02"),
			Messages: []CatalogMessage{
				{
					Name: "MSG-A", Date: MustParseDateOnly("2021-03-04"),
					Ministry: WordOfLife, Visibility: Public,
					Speakers: []string{"VERN"},
					Series:   []SeriesReference{{Name: "SERIES-1", Index: 2}},
				},
				{
					Name: "MSG-B", Date: MustParseDateOnly("2021-03-11"),
					Ministry: WordOfLife, Visibility: Private,
					Speakers: []string{"MARY"},
					Series:   []SeriesReference{{Name: "SERIES-1", Index: 3}},
				},
				{
					Name: "MSG-C", Date: MustParseDateOnly("2021-03-18"),
					Ministry: WordOfLife, Visibility: Partner,
					Speakers: []string{"DAVE"},
					Series:   []SeriesReference{{Name: "SERIES-1", Index: 1}},
				},
			},
		},
	}

	// when, then
	result := FilterSeriesByView(corpus, Public)
	seri := result[0]
	t.Equal([]string{"NANA", "VERN"}, seri.AllSpeakers)
	t.Equal(MustParseDateOnly("2021-03-04"), seri.StartDate)
	t.Equal(MustParseDateOnly("2021-03-04"), seri.StopDate)
	t.Len(seri.Messages, 1)
	t.Equal("MSG-A", seri.Messages[0].Name)

	// when, then
	result = FilterSeriesByView(corpus, Partner)
	seri = result[0]
	t.Equal([]string{"NANA", "DAVE", "VERN"}, seri.AllSpeakers)
	t.Equal(MustParseDateOnly("2021-03-04"), seri.StartDate)
	t.Equal(MustParseDateOnly("2021-03-18"), seri.StopDate)
	t.Len(seri.Messages, 2)
	t.Equal("MSG-C", seri.Messages[0].Name)
	t.Equal("MSG-A", seri.Messages[1].Name)

	// when, then
	result = FilterSeriesByView(corpus, Private)
	seri = result[0]
	t.Equal([]string{"NANA", "DAVE", "VERN", "MARY"}, seri.AllSpeakers)
	t.Equal(MustParseDateOnly("2021-03-04"), seri.StartDate)
	t.Equal(MustParseDateOnly("2021-03-18"), seri.StopDate)
	t.Len(seri.Messages, 3)
	t.Equal("MSG-C", seri.Messages[0].Name)
	t.Equal("MSG-A", seri.Messages[1].Name)
	t.Equal("MSG-B", seri.Messages[2].Name)

}
