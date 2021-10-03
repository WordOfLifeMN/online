package catalog

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type CatalogTestSuite struct {
	suite.Suite
}

// Runs the test suite as a test
func TestCatalogTestSuite(t *testing.T) {
	suite.Run(t, new(CatalogTestSuite))
}

// +---------------------------------------------------------------------------
// | Construction
// +---------------------------------------------------------------------------

func (t *CatalogTestSuite) TestAddMessagesToTheirSeries() {
	// given
	sut := Catalog{
		Series: []CatalogSeri{
			{Name: "SERIES-1"},
			{Name: "SERIES-2"},
		},
		Messages: []CatalogMessage{
			{
				Date: MustParseDateOnly("2021-01-01"),
				Name: "MSG-A",
				Series: []SeriesReference{
					{Name: "SERIES-1", Index: 1},
				},
			},
			{
				Date: MustParseDateOnly("2021-01-02"),
				Name: "MSG-B",
				Series: []SeriesReference{
					{Name: "SERIES-1", Index: 2},
					{Name: "SERIES-2", Index: 1},
				},
			},
			{
				Date: MustParseDateOnly("2021-01-03"),
				Name: "MSG-C",
				Series: []SeriesReference{
					{Name: "SERIES-1", Index: 3},
					{Name: "SERIES-2", Index: 2},
					{Name: "SERIES-3", Index: 1}, // bogus reference
				},
			},
			{
				Date: MustParseDateOnly("2021-01-04"),
				Name: "MSG-D",
				Series: []SeriesReference{
					{Name: "SERIES-2", Index: 3},
					{Name: "SAM", Index: 0},
				},
			},
		},
	}

	// when
	sut.addMessagesToTheirSeries()

	// then
	t.Len(sut.Series, 2)

	// then series 1
	s := sut.Series[0]
	t.Len(s.Messages, 3)
	t.Equal("MSG-A", s.Messages[0].Name)
	t.Equal("MSG-B", s.Messages[1].Name)
	t.Equal("MSG-C", s.Messages[2].Name)
	t.Equal(MustParseDateOnly("2021-01-01"), s.StartDate)
	t.Equal(MustParseDateOnly("2021-01-03"), s.StopDate)

	// then series 2
	s = sut.Series[1]
	t.Len(s.Messages, 3)
	t.Equal("MSG-B", s.Messages[0].Name)
	t.Equal("MSG-C", s.Messages[1].Name)
	t.Equal("MSG-D", s.Messages[2].Name)
	t.Equal(MustParseDateOnly("2021-01-02"), s.StartDate)
	t.Equal(MustParseDateOnly("2021-01-04"), s.StopDate)
}
func (t *CatalogTestSuite) TestCreatingSeriesForStandaloneMessages() {
	// given
	sut := Catalog{
		Series: []CatalogSeri{},
		Messages: []CatalogMessage{
			{
				Date: MustParseDateOnly("2021-01-01"),
				Name: "MSG-A",
			},
			{
				Date: MustParseDateOnly("2021-01-02"),
				Name: "MSG-B",
				Series: []SeriesReference{
					{Name: "SERIES-1", Index: 2},
					{Name: "SAM", Index: 1},
				},
			},
			{
				Date: MustParseDateOnly("2021-01-03"),
				Name: "MSG-C",
				Series: []SeriesReference{
					{Name: "SAM", Index: 2},
				},
			},
		},
	}

	// when
	sut.createStandAloneMessageSeries()

	// then
	t.Len(sut.Series, 3)

	// then series 1
	s := sut.Series[0]
	t.Len(s.Messages, 1)
	t.Equal("MSG-A", s.Name)
	t.Equal("MSG-A", s.Messages[0].Name)
	t.Equal(MustParseDateOnly("2021-01-01"), s.StartDate)
	t.Equal(MustParseDateOnly("2021-01-01"), s.StopDate)

	// then series 2
	s = sut.Series[1]
	t.Len(s.Messages, 1)
	t.Equal("MSG-B", s.Name)
	t.Equal("MSG-B", s.Messages[0].Name)
	t.Equal(MustParseDateOnly("2021-01-02"), s.StartDate)
	t.Equal(MustParseDateOnly("2021-01-02"), s.StopDate)

	// then series 3
	s = sut.Series[2]
	t.Len(s.Messages, 1)
	t.Equal("MSG-C", s.Name)
	t.Equal("MSG-C", s.Messages[0].Name)
	t.Equal(MustParseDateOnly("2021-01-03"), s.StartDate)
	t.Equal(MustParseDateOnly("2021-01-03"), s.StopDate)

}

// +---------------------------------------------------------------------------
// | Access
// +---------------------------------------------------------------------------

func (t *CatalogTestSuite) TestFindSeries() {
	// given
	sut := Catalog{
		Series: []CatalogSeri{
			{
				Name: "TEST-A",
			},
		},
		Messages: []CatalogMessage{},
	}

	// then
	seri, ok := sut.FindSeriByName("TEST-A")
	t.True(ok)
	t.NotNil(seri)
	t.Equal("TEST-A", seri.Name)

	seri, ok = sut.FindSeriByName("NUN-SUCH")
	t.False(ok)
	t.Nil(seri)
}

func (t *CatalogTestSuite) TestFindSeries_Empty() {
	// given
	sut := Catalog{}

	// then
	seri, ok := sut.FindSeriByName("NUN-SUCH")
	t.False(ok)
	t.Nil(seri)
}

func (t *CatalogTestSuite) TestFindMessagesInSeries_Empty() {
	// given
	sut := Catalog{}

	t.Len(sut.FindMessagesInSeries("SERIES"), 0)
}

func (t *CatalogTestSuite) TestFindMessagesInSeries_None() {
	// given
	sut := Catalog{
		Messages: []CatalogMessage{
			{
				Name: "MSG-1",
				Series: []SeriesReference{
					{
						Name: "OTHER",
					},
				},
			},
		},
	}

	t.Len(sut.FindMessagesInSeries("SERIES"), 0)
}

func (t *CatalogTestSuite) TestFindMessagesInSeries_Single() {
	// given
	sut := Catalog{
		Messages: []CatalogMessage{
			{
				Name: "MSG-1",
				Series: []SeriesReference{
					{Name: "SERIES", Index: 1},
				},
			},
		},
	}

	msgs := sut.FindMessagesInSeries("SERIES")
	t.Len(msgs, 1)
	t.Equal("MSG-1", msgs[0].Name)
}

func (t *CatalogTestSuite) TestFindMessagesInSeries_MultipleMessages() {
	// given
	sut := Catalog{
		Messages: []CatalogMessage{
			{
				Name: "MSG-1",
				Series: []SeriesReference{
					{Name: "SERIES", Index: 1},
				},
			},
			{
				Name: "MSG-2",
				Series: []SeriesReference{
					{Name: "SERIES", Index: 1},
				},
			},
		},
	}

	msgs := sut.FindMessagesInSeries("SERIES")
	t.Len(msgs, 2)
	t.Equal("MSG-1", msgs[0].Name)
	t.Equal("MSG-2", msgs[1].Name)
}

func (t *CatalogTestSuite) TestFindMessagesInSeries_ExtraSeriesRemoved() {
	// given
	sut := Catalog{
		Messages: []CatalogMessage{
			{
				Name: "MSG-1",
				Series: []SeriesReference{
					{Name: "SERIES-A", Index: 1},
					{Name: "SERIES-B", Index: 1},
					{Name: "SERIES-C", Index: 1},
				},
			},
		},
	}

	msgs := sut.FindMessagesInSeries("SERIES-B")
	t.Len(msgs, 1)
	t.Equal("MSG-1", msgs[0].Name)
	t.Len(msgs[0].Series, 1)
	t.Equal("SERIES-B", msgs[0].Series[0].Name)
}

func (t *CatalogTestSuite) TestFindMessagesInSeries_SeriesSortedCorrectly() {
	// given
	sut := Catalog{
		Messages: []CatalogMessage{
			{
				Name: "MSG-1",
				Series: []SeriesReference{
					{Name: "SERIES", Index: 1},
				},
			},
			{
				Name: "MSG-3",
				Series: []SeriesReference{
					{Name: "SERIES", Index: 3},
				},
			},
			{
				Name: "MSG-2",
				Series: []SeriesReference{
					{Name: "SERIES", Index: 2},
				},
			},
		},
	}

	msgs := sut.FindMessagesInSeries("SERIES")
	t.Len(msgs, 3)
	t.Equal("MSG-1", msgs[0].Name)
	t.Equal("MSG-2", msgs[1].Name)
	t.Equal("MSG-3", msgs[2].Name)
}

func (t *CatalogTestSuite) TestFindMessagesInSeries_IndexZeroSortedToEnd() {
	// given
	sut := Catalog{
		Messages: []CatalogMessage{
			{
				Name: "MSG-1",
				Series: []SeriesReference{
					{Name: "SERIES", Index: 0},
				},
			},
			{
				Name: "MSG-3",
				Series: []SeriesReference{
					{Name: "SERIES", Index: 2},
				},
			},
			{
				Name: "MSG-2",
				Series: []SeriesReference{
					{Name: "SERIES", Index: 1},
				},
			},
		},
	}

	msgs := sut.FindMessagesInSeries("SERIES")
	t.Len(msgs, 3)
	t.Equal("MSG-2", msgs[0].Name)
	t.Equal("MSG-3", msgs[1].Name)
	t.Equal("MSG-1", msgs[2].Name)
}

func (t *CatalogTestSuite) TestFindMessagesInSeries_IndexZeroSortedStable() {
	// given
	sut := Catalog{
		Messages: []CatalogMessage{
			{
				Name: "MSG-1",
				Series: []SeriesReference{
					{Name: "SERIES", Index: 0},
				},
			},
			{
				Name: "MSG-3",
				Series: []SeriesReference{
					{Name: "SERIES", Index: 1},
				},
			},
			{
				Name: "MSG-2",
				Series: []SeriesReference{
					{Name: "SERIES", Index: 0},
				},
			},
		},
	}

	msgs := sut.FindMessagesInSeries("SERIES")
	t.Len(msgs, 3)
	t.Equal("MSG-3", msgs[0].Name)
	t.Equal("MSG-1", msgs[1].Name)
	t.Equal("MSG-2", msgs[2].Name)
}
