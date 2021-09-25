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
