package util

import (
	"os"
	"path/filepath"
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

// A zero-length file is what a tool leaves behind when it reports success but
// produces nothing. Treating that as usable output means the step that produced it is
// skipped on every future run, so IsNonEmptyFile must reject it where IsFile accepts.
func (t *OSTestSuite) TestIsNonEmptyFile() {
	dir := t.T().TempDir()

	empty := filepath.Join(dir, "empty.text")
	t.Require().NoError(os.WriteFile(empty, nil, 0666))

	full := filepath.Join(dir, "full.text")
	t.Require().NoError(os.WriteFile(full, []byte("transcribed words"), 0666))

	missing := filepath.Join(dir, "missing.text")

	// IsFile cannot tell the empty file from the full one
	t.True(IsFile(empty))
	t.True(IsFile(full))

	// IsNonEmptyFile can
	t.False(IsNonEmptyFile(empty), "a zero-length file is not usable output")
	t.True(IsNonEmptyFile(full))
	t.False(IsNonEmptyFile(missing))
	t.False(IsNonEmptyFile(dir), "a directory is not a usable file")
}

func (t *OSTestSuite) TestCopyFile_CopiesWithNothingLeftOver() {
	dir := t.T().TempDir()
	src := filepath.Join(dir, "src.text")
	dst := filepath.Join(dir, "dst.txt")
	t.Require().NoError(os.WriteFile(src, []byte("transcribed words"), 0666))

	t.Require().NoError(CopyFile(src, dst))

	content, err := os.ReadFile(dst)
	t.Require().NoError(err)
	t.Equal("transcribed words", string(content))
	t.False(DoesPathExist(dst+".partial"), "the partial file is renamed, not left behind")
	t.True(IsFile(src), "the source is copied, not moved")
}

// a copy that cannot be made must leave nothing behind at either name, because the
// presence of dst is what callers take to mean the work is done
func (t *OSTestSuite) TestCopyFile_MissingDestinationFolder() {
	dir := t.T().TempDir()
	src := filepath.Join(dir, "src.text")
	t.Require().NoError(os.WriteFile(src, []byte("transcribed words"), 0666))
	dst := filepath.Join(dir, "never-created", "dst.txt")

	t.Error(CopyFile(src, dst))
	t.False(DoesPathExist(dst))
	t.False(DoesPathExist(dst + ".partial"))
}

func (t *OSTestSuite) TestCopyFile_MissingSource() {
	dir := t.T().TempDir()
	dst := filepath.Join(dir, "dst.txt")

	t.Error(CopyFile(filepath.Join(dir, "missing.text"), dst))
	t.False(DoesPathExist(dst))
	t.False(DoesPathExist(dst + ".partial"))
}
