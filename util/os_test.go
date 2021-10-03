package util

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileExists_FileExists(t *testing.T) {
	testPath := "/tmp/testfile.txt"

	_, err := os.Create(testPath)
	defer os.Remove(testPath)
	if assert.NoError(t, err) {
		assert.True(t, DoesPathExist(testPath))
	}
}

func TestFileExists_DirExists(t *testing.T) {
	testPath := "/tmp/testdir"

	err := os.Mkdir(testPath, os.FileMode(0777))
	defer os.Remove(testPath)
	if assert.NoError(t, err) {
		assert.True(t, DoesPathExist(testPath))
	}
}

func TestFileExists_FileMissing(t *testing.T) {
	testPath := "/tmp/testfile.txt"
	os.Remove(testPath)
	assert.False(t, DoesPathExist(testPath))
}

func TestIsFile_File(t *testing.T) {
	testPath := "/tmp/testfile.txt"

	_, err := os.Create(testPath)
	defer os.Remove(testPath)
	if assert.NoError(t, err) {
		assert.True(t, IsFile(testPath))
	}
}

func TestIsFile_Dir(t *testing.T) {
	testPath := "/tmp/testdir"

	err := os.Mkdir(testPath, os.FileMode(0777))
	defer os.Remove(testPath)
	if assert.NoError(t, err) {
		assert.False(t, IsFile(testPath))
	}
}

func TestIsFile_Missing(t *testing.T) {
	testPath := "/tmp/testfile.txt"

	os.Remove(testPath)
	assert.False(t, IsFile(testPath))
}

func TestIsDir_Dir(t *testing.T) {
	testPath := "/tmp/testdir"

	err := os.Mkdir(testPath, os.FileMode(0777))
	defer os.Remove(testPath)
	if assert.NoError(t, err) {
		assert.True(t, IsDirectory(testPath))
	}
}

func TestIsDir_File(t *testing.T) {
	testPath := "/tmp/testfile.txt"

	_, err := os.Create(testPath)
	defer os.Remove(testPath)
	if assert.NoError(t, err) {
		assert.False(t, IsDirectory(testPath))
	}
}

func TestIsDir_Missing(t *testing.T) {
	testPath := "/tmp/testdir"

	os.Remove(testPath)
	assert.False(t, IsDirectory(testPath))
}
