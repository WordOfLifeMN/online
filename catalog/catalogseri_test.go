package catalog

import (
	"sort"
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

func (t *CatalogSeriTestSuite) TestNewSeriesFromStandAloneMessage() {
	// given
	msg := CatalogMessage{
		Date:        MustParseDateOnly("2006-07-08"),
		Name:        "A MESSAGE",
		Description: "A DESCRIPTION",
		Visibility:  Partner,
		Speakers:    []string{"OLLIE", "SVEN"},
		Resources: []OnlineResource{
			{URL: "http://ollie.png", Name: "Ollie Portrait"},
			{URL: "http://sven.png", Name: "Sven Portrait"},
		},
	}

	// when
	sut := NewSeriesFromMessage(&msg)

	// then
	t.Equal(MustParseDateOnly("2006-07-08"), sut.StartDate)
	t.Equal(MustParseDateOnly("2006-07-08"), sut.StopDate)
	t.Equal("A MESSAGE", sut.Name)
	t.Equal("A DESCRIPTION", sut.Description)
	t.Equal([]string{"OLLIE", "SVEN"}, sut.Speakers)
	t.Equal(Partner, sut.Visibility)
	t.Equal(State_Complete, sut.State)

	// resources
	t.Nil(sut.Booklets)
	t.Len(sut.Resources, 2)
	t.Equal("Ollie Portrait", sut.Resources[0].Name)
	t.Equal("Sven Portrait", sut.Resources[1].Name)

	// messages
	t.Len(sut.Messages, 1)
	t.Equal("A MESSAGE", sut.Messages[0].Name)
	t.Len(sut.Messages[0].Series, 1)
	t.Equal(SeriesReference{Name: "A MESSAGE", Index: 1}, sut.Messages[0].Series[0])

	// verify message in series is not the message used for the copy
	t.NotSame(&sut.Messages[0], &msg)
	t.Len(msg.Series, 0)
}

func (t *CatalogSeriTestSuite) TestNewSeriesFromMessageInSeries() {
	// given
	msg := CatalogMessage{
		Date:        MustParseDateOnly("2006-07-08"),
		Name:        "A MESSAGE",
		Description: "A DESCRIPTION",
		Visibility:  Partner,
		Speakers:    []string{"OLLIE", "SVEN"},
		Resources: []OnlineResource{
			{URL: "http://ollie.png", Name: "Ollie Portrait"},
			{URL: "http://sven.png", Name: "Sven Portrait"},
		},
		Series: []SeriesReference{
			{Name: "SOME OTHER SERIES", Index: 2},
			{Name: "SAM", Index: 1},
		},
	}

	// when
	sut := NewSeriesFromMessage(&msg)

	// then
	t.Equal(MustParseDateOnly("2006-07-08"), sut.StartDate)
	t.Equal(MustParseDateOnly("2006-07-08"), sut.StopDate)
	t.Equal("A MESSAGE", sut.Name)
	t.Equal("A DESCRIPTION", sut.Description)
	t.Equal([]string{"OLLIE", "SVEN"}, sut.Speakers)
	t.Equal(Partner, sut.Visibility)

	// resources
	t.Nil(sut.Booklets)
	t.Len(sut.Resources, 2)
	t.Equal("Ollie Portrait", sut.Resources[0].Name)
	t.Equal("Sven Portrait", sut.Resources[1].Name)

	// messages
	t.Len(sut.Messages, 1)
	t.Equal("A MESSAGE", sut.Messages[0].Name)
	t.Len(sut.Messages[0].Series, 1)
	t.Equal(SeriesReference{Name: "A MESSAGE", Index: 1}, sut.Messages[0].Series[0])

	// verify message in series is not the message used for the copy
	t.NotSame(&sut.Messages[0], &msg)
	t.Len(msg.Series, 2)
}

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
	t.Len(sut.Speakers, 4)
	t.Equal("Tim", sut.Speakers[0])
	t.Equal("Sam", sut.Speakers[1])
	t.Equal("Ollie", sut.Speakers[2])
	t.Equal("Sven", sut.Speakers[3])
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
	t.Len(sut.Resources, 3)
	t.Equal("First Study Notes", sut.Resources[0].Name) // notes from 1st message
	t.Equal("Sidetrack", sut.Resources[1].Name)         // unique notes from 2nd message
	t.Equal("Skizzle", sut.Resources[2].Name)           // note from 0th message
}

func (t *CatalogSeriTestSuite) TestCopy() {
	seri := CatalogSeri{
		ID:          "ID",
		Name:        "SERIES",
		Description: "DESCRIPTION",
		Speakers:    []string{"VERN", "MARY"},
		Booklets:    []OnlineResource{{URL: "https://thing.pdf", Name: "Thing", thumbnail: "https://thumb.jpg", classifier: "pdf"}},
		Resources:   []OnlineResource{{URL: "https://thing.pdf", Name: "Thing", thumbnail: "https://thumb.jpg", classifier: "pdf"}},
		Visibility:  Public,
		Jacket:      "https://jacket.pdf",
		Thumbnail:   "https://thumb.jpg",
		StartDate:   MustParseDateOnly("2021-02-03"),
		StopDate:    MustParseDateOnly("2021-05-14"),
		State:       State_InProgress,
		View:        Public,
		Messages:    []CatalogMessage{},
		initialized: true,
	}

	cpy := seri.Copy()

	t.Equal(seri.Name, cpy.Name)
	t.Equal(seri.State, cpy.State)
	t.True(cpy.initialized)
	t.NotSame(seri.Speakers, cpy.Speakers)
	t.NotSame(seri.Booklets, cpy.Booklets)
	t.NotSame(seri.Resources, cpy.Resources)
	t.NotSame(seri.Messages, cpy.Messages)
	t.NotSame(seri.Speakers, cpy.Speakers)
	t.NotSame(seri.Resources, cpy.Resources)
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

func (t *CatalogSeriTestSuite) TestDateString() {
	// given message in the future
	sut := CatalogSeri{
		Name: "SERIES",
	}
	sut.Initialize()
	t.Equal("Coming Soon", sut.DateString())

	// given message with start date
	sut = CatalogSeri{
		Name:      "SERIES",
		StartDate: MustParseDateOnly("2006-07-08"),
	}
	sut.Initialize()
	t.Equal("Started Jul 8, 2006", sut.DateString())

	// given message with start/end date
	sut = CatalogSeri{
		Name:      "SERIES",
		StartDate: MustParseDateOnly("2006-07-08"),
		StopDate:  MustParseDateOnly("2008-09-01"),
	}
	sut.Initialize()
	t.Equal("Jul 8, 2006 - Sep 1, 2008", sut.DateString())

	// given message completed in same year
	sut = CatalogSeri{
		Name:      "SERIES",
		StartDate: MustParseDateOnly("2006-07-08"),
		StopDate:  MustParseDateOnly("2006-09-01"),
	}
	sut.Initialize()
	t.Equal("Jul 8 - Sep 1, 2006", sut.DateString())

	// given message completed in same month
	sut = CatalogSeri{
		Name:      "SERIES",
		StartDate: MustParseDateOnly("2006-07-08"),
		StopDate:  MustParseDateOnly("2006-07-21"),
	}
	sut.Initialize()
	t.Equal("Jul 8-21, 2006", sut.DateString())

	// given message completed on same day
	sut = CatalogSeri{
		Name:      "SERIES",
		StartDate: MustParseDateOnly("2006-07-08"),
		StopDate:  MustParseDateOnly("2006-07-08"),
	}
	sut.Initialize()
	t.Equal("Jul 8, 2006", sut.DateString())
}

func (t *CatalogSeriTestSuite) TestSpeakerString() {
	// given
	sut := CatalogSeri{
		Name: "SERIES",
	}
	t.Equal("", sut.SpeakerString())

	// given
	sut.Speakers = append(sut.Speakers, "SVEN")
	t.Equal("SVEN", sut.SpeakerString())

	// given
	sut.Speakers = append(sut.Speakers, "OLLIE")
	t.Equal("SVEN, OLLIE", sut.SpeakerString())
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

func (t *CatalogSeriTestSuite) TestFilterByMinistries() {
	cat, err := NewCatalogFromJSON("../testdata/small-catalog.json")
	t.NoError(err)
	t.NoError(cat.Initialize())

	// when
	result := FilterSeriesByMinistry(cat.Series, WordOfLife, CenterOfRelationshipExperience)

	// then
	t.Len(result, 4)
	t.Equal("A People of Conviction, or Convenience?", result[0].Name)
	t.Equal("Summerfest 2014", result[1].Name)
	t.Equal("Word (Feb 2, 2014)", result[2].Name)
	t.Equal("Attachment Disorder", result[3].Name)
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
	t.Equal(Public, result[0].View)
	t.Equal("SERIES-1", result[0].Name)

	// when, then
	result = FilterSeriesByView(corpus, Partner)
	t.Len(result, 2)
	t.Equal(Partner, result[0].View)
	t.Equal("SERIES-1", result[0].Name)
	t.Equal("SERIES-2", result[1].Name)

	// when, then
	result = FilterSeriesByView(corpus, Private)
	t.Len(result, 3)
	t.Equal(Private, result[0].View)
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
	t.Equal(Public, result[0].View)
	t.Equal("SERIES-1", result[0].Name)
	t.Len(result[0].Messages, 1)
	t.Equal("MSG-A", result[0].Messages[0].Name)

	// when, then
	result = FilterSeriesByView(corpus, Partner)
	t.Len(result, 2)

	t.Equal("SERIES-1", result[0].Name)
	t.Len(result[0].Messages, 1)
	t.Equal(Partner, result[0].View)
	t.Equal("MSG-A", result[0].Messages[0].Name)

	t.Equal("SERIES-2", result[1].Name)
	t.Equal(Partner, result[0].View)
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
	t.Equal(Public, seri.View)
	t.Equal([]string{"VERN"}, seri.Speakers)
	t.Equal(MustParseDateOnly("2021-03-04"), seri.StartDate)
	t.Equal(MustParseDateOnly("2021-03-04"), seri.StopDate)
	t.Len(seri.Messages, 1)
	t.Equal("MSG-A", seri.Messages[0].Name)

	// when, then
	result = FilterSeriesByView(corpus, Partner)
	seri = result[0]
	t.Equal(Partner, seri.View)
	t.Equal([]string{"DAVE", "VERN"}, seri.Speakers)
	t.Equal(MustParseDateOnly("2021-03-04"), seri.StartDate)
	t.Equal(MustParseDateOnly("2021-03-18"), seri.StopDate)
	t.Len(seri.Messages, 2)
	t.Equal("MSG-C", seri.Messages[0].Name)
	t.Equal("MSG-A", seri.Messages[1].Name)

	// when, then
	result = FilterSeriesByView(corpus, Private)
	seri = result[0]
	t.Equal(Private, seri.View)
	t.Equal([]string{"DAVE", "VERN", "MARY"}, seri.Speakers)
	t.Equal(MustParseDateOnly("2021-03-04"), seri.StartDate)
	t.Equal(MustParseDateOnly("2021-03-18"), seri.StopDate)
	t.Len(seri.Messages, 3)
	t.Equal("MSG-C", seri.Messages[0].Name)
	t.Equal("MSG-A", seri.Messages[1].Name)
	t.Equal("MSG-B", seri.Messages[2].Name)

}

func (t *CatalogSeriTestSuite) TestSortByName() {
	// given
	corpus := []CatalogSeri{
		{
			Name:       "CHARLIE-SERIES",
			Visibility: Public,
			Speakers:   []string{"NANA"},
			StartDate:  MustParseDateOnly("2001-01-01"),
			StopDate:   MustParseDateOnly("2001-01-02"),
		},
		{
			Name:       "ALFA-SERIES",
			Visibility: Public,
			Speakers:   []string{"NANA"},
			StartDate:  MustParseDateOnly("2001-02-01"),
			StopDate:   MustParseDateOnly("2001-02-02"),
		},
		{
			Name:       "BRAVO-SERIES",
			Visibility: Public,
			Speakers:   []string{"NANA"},
			StartDate:  MustParseDateOnly("2001-03-01"),
			StopDate:   MustParseDateOnly("2001-03-02"),
		},
	}

	// when
	sort.Sort(SortSeriByName(corpus))
	t.Equal("ALFA-SERIES", corpus[0].Name)
	t.Equal("BRAVO-SERIES", corpus[1].Name)
	t.Equal("CHARLIE-SERIES", corpus[2].Name)
}

func (t *CatalogSeriTestSuite) TestSortOldestToNewest() {
	// given
	corpus := []CatalogSeri{
		{
			Name:       "CHARLIE-SERIES",
			Visibility: Public,
			Speakers:   []string{"NANA"},
			StartDate:  MustParseDateOnly("2001-01-01"),
			StopDate:   MustParseDateOnly("2001-01-02"),
		},
		{
			Name:       "ALFA-SERIES",
			Visibility: Public,
			Speakers:   []string{"NANA"},
			StartDate:  MustParseDateOnly("2001-02-01"),
			StopDate:   MustParseDateOnly("2001-02-02"),
		},
		{
			Name:       "BRAVO-SERIES",
			Visibility: Public,
			Speakers:   []string{"NANA"},
			StartDate:  MustParseDateOnly("2001-03-01"),
			StopDate:   MustParseDateOnly("2001-03-02"),
		},
	}

	// when
	sort.Sort(SortSeriOldestToNewest(corpus))
	t.Equal("CHARLIE-SERIES", corpus[0].Name)
	t.Equal("ALFA-SERIES", corpus[1].Name)
	t.Equal("BRAVO-SERIES", corpus[2].Name)
}

func (t *CatalogSeriTestSuite) TestSortNewestToOldest() {
	// given
	corpus := []CatalogSeri{
		{
			Name:       "CHARLIE-SERIES",
			Visibility: Public,
			Speakers:   []string{"NANA"},
			StartDate:  MustParseDateOnly("2001-01-01"),
			StopDate:   MustParseDateOnly("2001-01-02"),
		},
		{
			Name:       "ALFA-SERIES",
			Visibility: Public,
			Speakers:   []string{"NANA"},
			StartDate:  MustParseDateOnly("2001-02-01"),
			StopDate:   MustParseDateOnly("2001-02-02"),
		},
		{
			Name:       "BRAVO-SERIES",
			Visibility: Public,
			Speakers:   []string{"NANA"},
			StartDate:  MustParseDateOnly("2001-03-01"),
			StopDate:   MustParseDateOnly("2001-03-02"),
		},
	}

	// when
	sort.Sort(SortSeriNewestToOldest(corpus))
	t.Equal("BRAVO-SERIES", corpus[0].Name)
	t.Equal("ALFA-SERIES", corpus[1].Name)
	t.Equal("CHARLIE-SERIES", corpus[2].Name)
}
