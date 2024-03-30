package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/util"
	"github.com/stretchr/testify/suite"
)

func TestCatalogCmdTestSuite(t *testing.T) {
	suite.Run(t, new(CatalogCmdTestSuite))
}

type CatalogCmdTestSuite struct {
	suite.Suite
}

// Runs the test suite as a test
// +---------------------------------------------------------------------------
// | Prepare the output directory
// +---------------------------------------------------------------------------

func (t *CatalogCmdTestSuite) TestOutputDirectoryPrep_NewDir() {
	// given
	testDir := "/tmp/testdir"
	sut := catalogCmdStruct{
		OutputDir: testDir,
	}

	// when
	t.NoError(sut.initializeOutputDir())
	defer os.RemoveAll(testDir)

	// then
	t.True(util.IsDirectory(testDir))
	t.True(util.IsFile(filepath.Join(testDir, "is.online.catalog.dir")))

	t.NoError(os.RemoveAll(testDir))
}

func (t *CatalogCmdTestSuite) TestOutputDirectoryPrep_DirIsOkToDelete() {
	// given
	testDir := "/tmp/testdir"
	sut := catalogCmdStruct{
		OutputDir: testDir,
	}
	t.NoError(os.MkdirAll(testDir, os.FileMode(0777)))
	defer os.RemoveAll(testDir)
	f, err := os.Create(filepath.Join(testDir, "is.online.catalog.dir"))
	t.NoError(err)
	f.Close()

	f, err = os.Create(filepath.Join(testDir, "actual-file.txt"))
	t.NoError(err)
	f.Close()

	// when
	if t.NoError(sut.initializeOutputDir()) {

		// then
		t.True(util.IsDirectory(testDir))
		t.True(util.IsFile(filepath.Join(testDir, "is.online.catalog.dir")))
		t.False(util.IsFile(filepath.Join(testDir, "actual-file.txt")))
	}

	t.NoError(os.RemoveAll(testDir))
}

func (t *CatalogCmdTestSuite) TestOutputDirectoryPrep_DirShouldNotBeDeleted() {
	// given
	testDir := "/tmp/testdir"
	sut := catalogCmdStruct{
		OutputDir: testDir,
	}
	t.NoError(os.MkdirAll(testDir, os.FileMode(0777)))
	defer os.RemoveAll(testDir)
	f, err := os.Create(filepath.Join(testDir, "actual-file.txt"))
	t.NoError(err)
	f.Close()

	// when
	err = sut.initializeOutputDir()

	// then
	t.NotNil(err)
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
				Name:     "MESSAGE-A",
				Date:     catalog.MustParseDateOnly("2021-09-10"),
				Ministry: catalog.WordOfLife,
			},
			{
				Name:     "MESSAGE-B",
				Date:     catalog.MustParseDateOnly("2021-09-15"),
				Ministry: catalog.WordOfLife,
			},
		},
	}
	buf := new(bytes.Buffer)

	err := sut.printCatalogSeri(&seri, buf)
	t.NoError(err)
	t.T().Logf("Results of printing:\n%s", buf.String())
	t.Contains(buf.String(), "SERIES")
	t.Contains(buf.String(), "MESSAGE-A")
	t.Contains(buf.String(), "MESSAGE-B")
	// header and footer
	t.Contains(buf.String(), `content="WORD OF LIFE MINISTRIES"`)
	t.Contains(buf.String(), `&copy; Word of Life Ministries 2012`)
}

func (t *CatalogCmdTestSuite) TestSeriPage() {
	testDir := "/tmp/onlinetest"
	sut := catalogCmdStruct{
		OutputDir: testDir,
	}

	seri := catalog.CatalogSeri{
		Name:       "SERIES",
		Visibility: catalog.Public,
		View:       catalog.Public,
		Messages: []catalog.CatalogMessage{
			{
				Name:     "MESSAGE-A",
				Date:     catalog.MustParseDateOnly("2021-09-10"),
				Ministry: catalog.WordOfLife,
			},
			{
				Name:     "MESSAGE-B",
				Date:     catalog.MustParseDateOnly("2021-09-15"),
				Ministry: catalog.WordOfLife,
			},
		},
	}

	err := sut.initializeOutputDir()
	if t.NoError(err) {
		err := sut.copyStaticFilesToOutputDir([]catalog.Ministry{catalog.WordOfLife})
		t.NoError(err)
		err = sut.createCatalogSeriPage(&seri)
		//	defer os.RemoveAll(testDir)
		t.NoError(err)
	}
}
