package catalog

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type CatalogSeriTestSuite struct {
	suite.Suite
	VariableThatShouldStartAtFive int
}

// Runs the test suite as a test
func TestCatalogSeriTestSuite(t *testing.T) {
	suite.Run(t, new(CatalogSeriTestSuite))
}

/******************************************************************************
 * Access
 *****************************************************************************/

func (t *CatalogSeriTestSuite) TestSeriesID_NoMessage() {
	// given
	sut := CatalogSeri{
		Name:     "SERIES",
		messages: []CatalogMessage{},
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
