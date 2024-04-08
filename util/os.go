package util

import (
	"os"
	"strings"
)

// DoesPathExist determines if a specific file or directory path exists or not
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

// NormalizePath handles any path updates necessary for standard path resolution:
func NormalizePath(path string) string {
	// replace initial ~ with home directory
	if strings.HasPrefix(path, "~") {
		homeDir, _ := os.UserHomeDir()
		path = strings.Replace(path, "~", homeDir, 1)
	}

	return path
}
