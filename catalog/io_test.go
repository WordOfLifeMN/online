package catalog

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type CatalogIOTestSuite struct {
	suite.Suite
}

// unmarshal a test file
func (s *CatalogIOTestSuite) TestJSONRead() {
	sut, err := NewCatalogFromJSON("../testdata/minimal-catalog.json")
	s.NoError(err)

	// catalog
	s.NotNil(sut)

	// series
	s.NotNil(sut.Series)
	s.Len(sut.Series, 1)
	s.Equal("SERIES-ID", sut.Series[0].ID)
	s.Equal("SERIES-NAME", sut.Series[0].Name)
	s.Equal("SERIES-DESCRIPTION", sut.Series[0].Description)
	s.Equal(Public, sut.Series[0].Visibility)

	// messages
	s.NotNil(sut.Messages)
	s.Len(sut.Messages, 1)
	s.Equal(MustParseDateOnly("2021-01-01"), sut.Messages[0].Date)
	s.Equal("MESSAGE-NAME", sut.Messages[0].Name)
	s.Equal("MESSAGE-DESCRIPTION", sut.Messages[0].Description)
	s.Len(sut.Messages[0].Speakers, 2)
	s.Equal("SPEAKER-A", sut.Messages[0].Speakers[0])
	s.Equal("SPEAKER-B", sut.Messages[0].Speakers[1])
	s.Equal(WordOfLife, sut.Messages[0].Ministry)
	s.Equal(Message, sut.Messages[0].Type)
	s.Equal(Partner, sut.Messages[0].Visibility)
}

// marshal a test catalog
func (s *CatalogIOTestSuite) TestJSONWrite() {
	// given
	sut := Catalog{
		Series: []CatalogSeri{
			{
				ID:          "TEST-ID",
				Name:        "TEST-NAME",
				Description: "TEST-DESCRIPTION",
				Visibility:  "private",
				Booklets: []OnlineResource{
					{
						URL:  "URL://PATH",
						Name: "TEST-BOOKLET",
					},
				},
				Jacket:    "URL://JACKET",
				Thumbnail: "URL://THUMB",
				StartDate: MustParseDateOnly("2021-01-01"),
				StopDate:  MustParseDateOnly("2021-01-08"),
			},
		},
		Messages: []CatalogMessage{
			{
				Date:       MustParseDateOnly("2021-01-01"),
				Name:       "MSG-A",
				Speakers:   []string{"VERN", "MARY"},
				Visibility: "public",
				Audio:      &OnlineResource{URL: "URL://AUDIO"},
				Video:      &OnlineResource{URL: "URL://VIDEO"},
			},
			{
				Date:       MustParseDateOnly("2021-01-08"),
				Name:       "MSG-B",
				Speakers:   []string{"VERN"},
				Visibility: "public",
				Audio:      &OnlineResource{URL: "URL://AUDIO2"},
				Video:      &OnlineResource{URL: "URL://VIDEO2"},
			},
		},
	}

	// when
	NewJSONFileFromCatalog("/tmp/unittest.json", &sut)

	// then
	s.FileExists("/tmp/unittest.json")

	bytes, err := os.ReadFile("/tmp/unittest.json")
	s.NoError(err)

	expectedJson := `{
  "created": "0001-01-01T00:00:00Z",
  "series": [
    {
      "id": "TEST-ID",
      "name": "TEST-NAME",
      "start-date": "2021-01-01",
      "end-date": "2021-01-08",
      "description": "TEST-DESCRIPTION",
      "booklets": [
        {
          "url": "URL://PATH",
          "name": "TEST-BOOKLET"
        }
      ],
      "visibility": "private",
      "jacket": "URL://JACKET",
      "thumbnail": "URL://THUMB"
    }
  ],
  "messages": [
    {
      "date": "2021-01-01",
      "name": "MSG-A",
      "speakers": [
        "VERN",
        "MARY"
      ],
      "ministry": "",
      "type": "",
      "visibility": "public",
      "audio": {
        "url": "URL://AUDIO"
      },
      "video": {
        "url": "URL://VIDEO"
      }
    },
    {
      "date": "2021-01-08",
      "name": "MSG-B",
      "speakers": [
        "VERN"
      ],
      "ministry": "",
      "type": "",
      "visibility": "public",
      "audio": {
        "url": "URL://AUDIO2"
      },
      "video": {
        "url": "URL://VIDEO2"
      }
    }
  ]
}`
	s.Equal(expectedJson, string(bytes))
}

func TestCatalogIOTestSuite(t *testing.T) {
	suite.Run(t, new(CatalogIOTestSuite))
}
