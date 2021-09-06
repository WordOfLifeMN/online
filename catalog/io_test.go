package catalog

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type CatalogIOTestSuite struct {
	suite.Suite
}

func (s *CatalogIOTestSuite) TestJSONRead() {
	c, err := NewCatalogFromJSON("../testdata/minimal-catalog.json")
	s.Nil(err)

	s.NotNil(c)
}

func TestCatalogIOTestSuite(t *testing.T) {
	suite.Run(t, new(CatalogIOTestSuite))
}
