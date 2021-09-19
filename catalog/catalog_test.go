package catalog

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type CatalogTestSuite struct {
	suite.Suite
	VariableThatShouldStartAtFive int
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

/******************************************************************************
 * Validation
 *****************************************************************************/

func (t *CatalogTestSuite) TestValidateMessageSeries() {
	// given - empty message list
	sut := Catalog{
		Series: []CatalogSeri{
			{
				Name: "SERIES-A",
			},
			{
				Name: "SERIES-B",
			},
		},
		Messages: []CatalogMessage{},
	}
	// then
	t.True(sut.IsMessageSeriesValid())

	// given - add message without references
	sut.Messages = append(sut.Messages,
		CatalogMessage{
			Name: "MESSAGE",
		},
	)
	// then
	t.True(sut.IsMessageSeriesValid())

	// given - add valid reference to message
	sut.Messages[0].Series = []SeriesReference{
		{
			Name: "SERIES-A",
		},
	}

	// then
	t.True(sut.IsMessageSeriesValid())

	// given - change message to invalid reference
	sut.Messages[0].Series[0].Name = "NUN-SUCH"
	// then
	t.False(sut.IsMessageSeriesValid())

	// given - change message to list of valid references
	sut.Messages[0].Series[0].Name = "SERIES-A"
	sut.Messages[0].Series = append(sut.Messages[0].Series, SeriesReference{
		Name: "SERIES-B",
	})
	// then
	t.True(sut.IsMessageSeriesValid())

	// given - change one reference in list to invalid
	sut.Messages[0].Series[1].Name = "NUN-SUCH"
	// then
	t.False(sut.IsMessageSeriesValid())
}

func (t *CatalogTestSuite) TestValidateMessageSeriesIndex_NoMessage() {
	sut := Catalog{
		Series: []CatalogSeri{
			{
				Name: "SERIES",
			},
		},
		Messages: []CatalogMessage{},
	}
	// then
	t.True(sut.IsMessageSeriesValid())
}

func (t *CatalogTestSuite) TestValidateMessageSeriesIndex_SingleMessage() {
	sut := Catalog{
		Series: []CatalogSeri{
			{
				Name: "SERIES",
			},
		},
		Messages: []CatalogMessage{
			{Name: "MESSAGE-1", Series: []SeriesReference{{Name: "SERIES", Index: 1}}},
		},
	}

	// then
	t.True(sut.IsMessageSeriesIndexValid())
}

func (t *CatalogTestSuite) TestValidateMessageSeriesIndex_MultipleMessage() {
	sut := Catalog{
		Series: []CatalogSeri{
			{
				Name: "SERIES",
			},
		},
		Messages: []CatalogMessage{
			{Name: "MESSAGE-1", Series: []SeriesReference{{Name: "SERIES", Index: 1}}},
			{Name: "MESSAGE-2", Series: []SeriesReference{{Name: "SERIES", Index: 2}}},
			{Name: "MESSAGE-3", Series: []SeriesReference{{Name: "SERIES", Index: 3}}},
		},
	}

	// then
	t.True(sut.IsMessageSeriesIndexValid())
}

func (t *CatalogTestSuite) TestValidateMessageSeriesIndex_SingleMessageWithZero() {
	sut := Catalog{
		Series: []CatalogSeri{
			{
				Name: "SERIES",
			},
		},
		Messages: []CatalogMessage{
			{Name: "MESSAGE-1", Series: []SeriesReference{{Name: "SERIES", Index: 1}}},
			{Name: "MESSAGE-2", Series: []SeriesReference{{Name: "SERIES", Index: 0}}},
		},
	}

	// then
	t.True(sut.IsMessageSeriesIndexValid())
}

func (t *CatalogTestSuite) TestValidateMessageSeriesIndex_MultipleMessageWithZero() {
	sut := Catalog{
		Series: []CatalogSeri{
			{
				Name: "SERIES",
			},
		},
		Messages: []CatalogMessage{
			{Name: "MESSAGE-1", Series: []SeriesReference{{Name: "SERIES", Index: 1}}},
			{Name: "MESSAGE-2", Series: []SeriesReference{{Name: "SERIES", Index: 2}}},
			{Name: "MESSAGE-3", Series: []SeriesReference{{Name: "SERIES", Index: 0}}},
			{Name: "MESSAGE-3", Series: []SeriesReference{{Name: "SERIES", Index: 0}}},
		},
	}

	// then
	t.True(sut.IsMessageSeriesIndexValid())
}

func (t *CatalogTestSuite) TestValidateMessageSeriesIndex_DuplicateIndex() {
	sut := Catalog{
		Series: []CatalogSeri{
			{
				Name: "SERIES",
			},
		},
		Messages: []CatalogMessage{
			{Name: "MESSAGE-1", Series: []SeriesReference{{Name: "SERIES", Index: 1}}},
			{Name: "MESSAGE-2", Series: []SeriesReference{{Name: "SERIES", Index: 1}}},
			{Name: "MESSAGE-3", Series: []SeriesReference{{Name: "SERIES", Index: 2}}},
		},
	}

	// then
	t.False(sut.IsMessageSeriesIndexValid())
}

func (t *CatalogTestSuite) TestValidateMessageSeriesIndex_IndexGap() {
	sut := Catalog{
		Series: []CatalogSeri{
			{
				Name: "SERIES",
			},
		},
		Messages: []CatalogMessage{
			{Name: "MESSAGE-1", Series: []SeriesReference{{Name: "SERIES", Index: 1}}},
			{Name: "MESSAGE-2", Series: []SeriesReference{{Name: "SERIES", Index: 3}}},
			{Name: "MESSAGE-3", Series: []SeriesReference{{Name: "SERIES", Index: 4}}},
		},
	}

	// then
	t.False(sut.IsMessageSeriesIndexValid())
}

func (t *CatalogTestSuite) TestValidateMessageSeriesIndex_Index1Missing() {
	sut := Catalog{
		Series: []CatalogSeri{
			{
				Name: "SERIES",
			},
		},
		Messages: []CatalogMessage{
			{Name: "MESSAGE-1", Series: []SeriesReference{{Name: "SERIES", Index: 2}}},
			{Name: "MESSAGE-2", Series: []SeriesReference{{Name: "SERIES", Index: 3}}},
		},
	}

	// then
	t.False(sut.IsMessageSeriesIndexValid())
}

func (t *CatalogTestSuite) TestValidateSeriesNames() {
	sut := Catalog{
		Series: []CatalogSeri{
			{Name: "SERIES-A"},
			{Name: "SERIES-B"},
		},
	}

	// then
	t.True(sut.IsSeriesNamesValid())
}

func (t *CatalogTestSuite) TestValidateSeriesNames_Duplicates() {
	sut := Catalog{
		Series: []CatalogSeri{
			{Name: "SERIES-A"},
			{Name: "SERIES-B"},
			{Name: "SERIES-A"},
		},
	}

	// then
	t.False(sut.IsSeriesNamesValid())
}

// func (t *CatalogTestSuite) TestValidateMessageNames() {
// 	sut := Catalog{
// 		Messages: []CatalogMessage{
// 			{Name: "MESSAGE-A"},
// 			{Name: "MESSAGE-B"},
// 		},
// 	}

// 	// then
// 	t.True(sut.IsMessageNamesValid())
// }

// func (t *CatalogTestSuite) TestValidateMessageNames_Duplicates() {
// 	sut := Catalog{
// 		Messages: []CatalogMessage{
// 			{Name: "MESSAGE-A"},
// 			{Name: "MESSAGE-B"},
// 			{Name: "MESSAGE-A"},
// 		},
// 	}

// 	// then
// 	t.False(sut.IsMessageNamesValid())
// }

// func (t *CatalogTestSuite) TestValidateSeriesMessageNames_Duplicate() {
// 	sut := Catalog{
// 		Series: []CatalogSeri{
// 			{Name: "SERIES-A"},
// 			{Name: "SERIES-B"},
// 		},
// 		Messages: []CatalogMessage{
// 			{Name: "SERIES-A"},
// 		},
// 	}

// 	// then should be invalid because the message isn't in a series
// 	t.False(sut.IsSeriesAndMessageNamesValid())
// }

// func (t *CatalogTestSuite) TestValidateSeriesMessageNames_MessageInSeriesNotChecked() {
// 	sut := Catalog{
// 		Series: []CatalogSeri{
// 			{Name: "SERIES-A"},
// 			{Name: "SERIES-B"},
// 		},
// 		Messages: []CatalogMessage{
// 			{
// 				Name:   "SERIES-A",
// 				Series: []SeriesReference{{Name: "SERIES-A"}},
// 			},
// 		},
// 	}

// 	// then should be valid because the message is in a different series
// 	t.True(sut.IsSeriesAndMessageNamesValid())
// }
