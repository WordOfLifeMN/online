package util

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestOSTestSuite(t *testing.T) {
	suite.Run(t, new(OSTestSuite))
}

type OSTestSuite struct {
	suite.Suite
}

func (t *OSTestSuite) TestFileExists_FileExists() {
	testPath := "/tmp/testfile.txt"

	f, err := os.Create(testPath)
	t.NoError(err)
	f.Close()
	defer os.Remove(testPath)

	t.True(DoesPathExist(testPath))
}

func (t *OSTestSuite) TestFileExists_DirExists() {
	testPath := "/tmp/testdir"

	err := os.Mkdir(testPath, os.FileMode(0777))
	defer os.Remove(testPath)
	if t.NoError(err) {
		t.True(DoesPathExist(testPath))
	}
}

func (t *OSTestSuite) TestFileExists_FileMissing() {
	testPath := "/tmp/testfile.txt"
	os.Remove(testPath)
	t.False(DoesPathExist(testPath))
}

func (t *OSTestSuite) TestIsFile_File() {
	testPath := "/tmp/testfile.txt"

	f, err := os.Create(testPath)
	t.NoError(err)
	f.Close()
	defer os.Remove(testPath)

	t.True(IsFile(testPath))
}

func (t *OSTestSuite) TestIsFile_Dir() {
	testPath := "/tmp/testdir"

	err := os.Mkdir(testPath, os.FileMode(0777))
	defer os.Remove(testPath)
	if t.NoError(err) {
		t.False(IsFile(testPath))
	}
}

func (t *OSTestSuite) TestIsFile_Missing() {
	testPath := "/tmp/testfile.txt"

	os.Remove(testPath)
	t.False(IsFile(testPath))
}

func (t *OSTestSuite) TestIsDir_Dir() {
	testPath := "/tmp/testdir"

	err := os.Mkdir(testPath, os.FileMode(0777))
	defer os.Remove(testPath)
	if t.NoError(err) {
		t.True(IsDirectory(testPath))
	}
}

func (t *OSTestSuite) TestIsDir_File() {
	testPath := "/tmp/testfile.txt"

	f, err := os.Create(testPath)
	t.NoError(err)
	f.Close()
	defer os.Remove(testPath)

	t.False(IsDirectory(testPath))
}

func (t *OSTestSuite) TestIsDir_Missing() {
	testPath := "/tmp/testdir"

	os.Remove(testPath)
	t.False(IsDirectory(testPath))
}
