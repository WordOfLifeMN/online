package catalog

import (
	"testing"

	"github.com/WordOfLifeMN/online/util"
	"github.com/stretchr/testify/suite"
)

type ValidateTestSuite struct {
	suite.Suite
	Report *util.IndentingReport
}

// Runs the test suite as a test
func TestValidateTestSuite(t *testing.T) {
	suite.Run(t, new(ValidateTestSuite))
}

func (t *ValidateTestSuite) SetupTest() {
	t.Report = util.NewIndentingReport(util.ReportSilent)
}

// +---------------------------------------------------------------------------
// | Messages
// +---------------------------------------------------------------------------

func getValidTestMessage() CatalogMessage {
	return CatalogMessage{
		Date:        MustParseDateOnly("2020-02-02"),
		Name:        "MSG",
		Description: "MESSAGE DESCRIPTION",
		Ministry:    WordOfLife,
		Type:        Message,
		Visibility:  Public,
		Audio:       "https://path/to/file.mp3",
		Video:       "https://path/to/file.mp4",
	}
}
func (t *ValidateTestSuite) TestMessageDate() {
	sut := getValidTestMessage()
	sut.Date = DateOnly{}

	t.False(sut.IsValid(t.Report))
	t.Contains(t.Report.String(), "Has no date")
}

func (t *ValidateTestSuite) TestMessageName() {
	sut := getValidTestMessage()
	sut.Name = ""

	t.False(sut.IsValid(t.Report))
	t.Contains(t.Report.String(), "Has no name")
	t.Contains(t.Report.String(), "2020-02-02")
}

func (t *ValidateTestSuite) TestMessageMinistryMissing() {
	sut := getValidTestMessage()
	sut.Ministry = ""

	t.False(sut.IsValid(t.Report))
	t.Contains(t.Report.String(), "No ministry")
}

func (t *ValidateTestSuite) TestMessageMinistryUnknown() {
	sut := getValidTestMessage()
	sut.Ministry = UnknownMinistry

	t.False(sut.IsValid(t.Report))
	t.Contains(t.Report.String(), "Unknown ministry")
}

func (t *ValidateTestSuite) TestMessageTypeMissing() {
	sut := getValidTestMessage()
	sut.Type = ""

	t.False(sut.IsValid(t.Report))
	t.Contains(t.Report.String(), "No type")
}

func (t *ValidateTestSuite) TestMessageTypeUnknown() {
	sut := getValidTestMessage()
	sut.Type = UnknownType

	t.False(sut.IsValid(t.Report))
	t.Contains(t.Report.String(), "Unknown type")
}

func (t *ValidateTestSuite) TestMessageVisibilityMissing() {
	sut := getValidTestMessage()
	sut.Visibility = ""

	t.False(sut.IsValid(t.Report))
	t.Contains(t.Report.String(), "No visibility")
}

func (t *ValidateTestSuite) TestMessageVisibilityUnknown() {
	sut := getValidTestMessage()
	sut.Visibility = UnknownView

	t.False(sut.IsValid(t.Report))
	t.Contains(t.Report.String(), "Unknown visibility")
}

func (t *ValidateTestSuite) TestMessageAudio() {
	sut := getValidTestMessage()
	sut.Audio = "random string"

	t.False(sut.IsValid(t.Report))
	t.Contains(t.Report.String(), "Audio isn't valid")
}

func (t *ValidateTestSuite) TestMessageVideo() {
	sut := getValidTestMessage()
	sut.Video = "random string"

	t.False(sut.IsValid(t.Report))
	t.Contains(t.Report.String(), "Video isn't valid")
}

func (t *ValidateTestSuite) TestMessageResource() {
	sut := getValidTestMessage()
	sut.Resources = []OnlineResource{
		{URL: "not a url", Name: "broken"},
	}

	t.False(sut.IsValid(t.Report))
	t.Contains(t.Report.String(), "not contain a valid URL")
}

// +---------------------------------------------------------------------------
// | Series
// +---------------------------------------------------------------------------

func (t *ValidateTestSuite) TestSeriName() {
	sut := CatalogSeri{
		ID:          "ID-123",
		Description: "SERIES DESCRIPTION",
		Visibility:  Public,
	}

	t.False(sut.IsValid(t.Report))
	t.Contains(t.Report.String(), "Has no name")
}

func (t *ValidateTestSuite) TestSeriID() {
	sut := CatalogSeri{
		Name:        "SERIES",
		Description: "DESCRIPTION",
		Visibility:  Public,
	}
	t.False(sut.IsValid(t.Report))
	t.Contains(t.Report.String(), "Has no ID")

	sut.Visibility = Partner
	t.False(sut.IsValid(t.Report))
	t.Contains(t.Report.String(), "Has no ID")

	// Private series won't need IDs
	sut.Visibility = Private
	t.True(sut.IsValid(t.Report))
}

func (t *ValidateTestSuite) TestSeriBookletWithoutID() {
	sut := CatalogSeri{
		Name:        "SERIES",
		ID:          "ID-123",
		Description: "SERIES DESCRIPTION",
		Visibility:  Public,
		Booklets: []OnlineResource{
			{URL: "https://booklet.pdf"},
		},
	}

	t.True(sut.IsValid(t.Report))
}

func (t *ValidateTestSuite) TestSeriBooklet() {
	sut := CatalogSeri{
		Name:        "SERIES",
		ID:          "ID-123",
		Description: "SERIES DESCRIPTION",
		Visibility:  Public,
		Booklets: []OnlineResource{
			{URL: "not a url", Name: "broken"},
		},
	}

	t.False(sut.IsValid(t.Report))
	t.Contains(t.Report.String(), "not contain a valid URL")
}

// +---------------------------------------------------------------------------
// | Catalog
// +---------------------------------------------------------------------------

func (t *ValidateTestSuite) TestValidateMessageSeries() {
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
	t.True(sut.IsMessageSeriesValid(t.Report))

	// given - add message without references
	sut.Messages = append(sut.Messages,
		CatalogMessage{
			Name: "MESSAGE",
		},
	)
	// then
	t.True(sut.IsMessageSeriesValid(t.Report))

	// given - add valid reference to message
	sut.Messages[0].Series = []SeriesReference{
		{
			Name: "SERIES-A",
		},
	}

	// then
	t.True(sut.IsMessageSeriesValid(t.Report))

	// given - change message to invalid reference
	sut.Messages[0].Series[0].Name = "NUN-SUCH"
	// then
	t.False(sut.IsMessageSeriesValid(t.Report))

	// given - change message to list of valid references
	sut.Messages[0].Series[0].Name = "SERIES-A"
	sut.Messages[0].Series = append(sut.Messages[0].Series, SeriesReference{
		Name: "SERIES-B",
	})
	// then
	t.True(sut.IsMessageSeriesValid(t.Report))

	// given - change one reference in list to invalid
	sut.Messages[0].Series[1].Name = "NUN-SUCH"
	// then
	t.False(sut.IsMessageSeriesValid(t.Report))
}

func (t *ValidateTestSuite) TestValidateMessageSeriesIndex_NoMessage() {
	sut := Catalog{
		Series: []CatalogSeri{
			{
				Name: "SERIES",
			},
		},
		Messages: []CatalogMessage{},
	}
	// then
	t.True(sut.IsMessageSeriesValid(t.Report))
}

func (t *ValidateTestSuite) TestValidateMessageSeriesIndex_SingleMessage() {
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
	t.True(sut.IsMessageSeriesIndexValid(t.Report))
}

func (t *ValidateTestSuite) TestValidateMessageSeriesIndex_MultipleMessage() {
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
	t.True(sut.IsMessageSeriesIndexValid(t.Report))
}

func (t *ValidateTestSuite) TestValidateMessageSeriesIndex_SingleMessageWithZero() {
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
	t.True(sut.IsMessageSeriesIndexValid(t.Report))
}

func (t *ValidateTestSuite) TestValidateMessageSeriesIndex_MultipleMessageWithZero() {
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
	t.True(sut.IsMessageSeriesIndexValid(t.Report))
}

func (t *ValidateTestSuite) TestValidateMessageSeriesIndex_DuplicateIndex() {
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
	t.False(sut.IsMessageSeriesIndexValid(t.Report))
}

func (t *ValidateTestSuite) TestValidateMessageSeriesIndex_IndexGap() {
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
	t.False(sut.IsMessageSeriesIndexValid(t.Report))
}

func (t *ValidateTestSuite) TestValidateMessageSeriesIndex_Index1Missing() {
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
	t.False(sut.IsMessageSeriesIndexValid(t.Report))
}

func (t *ValidateTestSuite) TestValidateSeriesNames() {
	sut := Catalog{
		Series: []CatalogSeri{
			{Name: "SERIES-A"},
			{Name: "SERIES-B"},
		},
	}

	// then
	t.True(sut.IsSeriesNamesValid(t.Report))
}

func (t *ValidateTestSuite) TestValidateSeriesNames_Duplicates() {
	sut := Catalog{
		Series: []CatalogSeri{
			{Name: "SERIES-A"},
			{Name: "SERIES-B"},
			{Name: "SERIES-A"},
		},
	}

	// then
	t.False(sut.IsSeriesNamesValid(t.Report))
}

// func (t *ValidateTestSuite) TestValidateMessageNames() {
// 	sut := Catalog{
// 		Messages: []CatalogMessage{
// 			{Name: "MESSAGE-A"},
// 			{Name: "MESSAGE-B"},
// 		},
// 	}

// 	// then
// 	t.True(sut.IsMessageNamesValid(t.Report))
// }

// func (t *ValidateTestSuite) TestValidateMessageNames_Duplicates() {
// 	sut := Catalog{
// 		Messages: []CatalogMessage{
// 			{Name: "MESSAGE-A"},
// 			{Name: "MESSAGE-B"},
// 			{Name: "MESSAGE-A"},
// 		},
// 	}

// 	// then
// 	t.False(sut.IsMessageNamesValid(t.Report))
// }

// func (t *ValidateTestSuite) TestValidateSeriesMessageNames_Duplicate() {
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
// 	t.False(sut.IsSeriesAndMessageNamesValid(t.Report))
// }

// func (t *ValidateTestSuite) TestValidateSeriesMessageNames_MessageInSeriesNotChecked() {
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
// 	t.True(sut.IsSeriesAndMessageNamesValid(t.Report))
// }
