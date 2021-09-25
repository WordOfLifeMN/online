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

/******************************************************************************
 * Access
 *****************************************************************************/

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
	seri, ok := sut.FindSeries("TEST-A")
	t.True(ok)
	t.NotNil(seri)
	t.Equal("TEST-A", seri.Name)

	seri, ok = sut.FindSeries("NUN-SUCH")
	t.False(ok)
	t.Nil(seri)
}

func (t *CatalogTestSuite) TestFindSeries_Empty() {
	// given
	sut := Catalog{}

	// then
	seri, ok := sut.FindSeries("NUN-SUCH")
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
