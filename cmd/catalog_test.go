package cmd

import (
	"bytes"
	"testing"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/stretchr/testify/suite"
)

type CatalogCmdTestSuite struct {
	suite.Suite
}

// Runs the test suite as a test
func TestCatalogCmdTestSuite(t *testing.T) {
	suite.Run(t, new(CatalogCmdTestSuite))
}

// +---------------------------------------------------------------------------
// | Seri Output
// +---------------------------------------------------------------------------

func (t *CatalogCmdTestSuite) TestSeriTemplate() {
	sut := catalogCmdStruct{}

	seri := catalog.CatalogSeri{
		Name: "SERIES",
		Messages: []catalog.CatalogMessage{
			{
				Name: "MESSAGE-A",
				Date: catalog.MustParseDateOnly("2021-09-10"),
			},
			{
				Name: "MESSAGE-B",
				Date: catalog.MustParseDateOnly("2021-09-15"),
			},
		},
	}
	buf := new(bytes.Buffer)

	err := sut.printCatalogSeri(&seri, buf)
	t.NoError(err)
	t.T().Logf("Results of printing:\n%s", buf.String())
}
