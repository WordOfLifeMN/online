package util

import (
	"os"
)

// IsFileExists determines if a specific file or directory path exists or not
func DoesPathExist(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// IsFile determines if a file represented by `path` is a regular file or not
func IsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// IsDirectory determines if a file represented by `path` is a directory or not
func IsDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
