package util

import (
	"io"
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

// IsNonEmptyFile determines if `path` is a regular file with content in it.
//
// Use this rather than IsFile when deciding whether an earlier step's output can be
// reused. A zero-length file is what a tool leaves behind when it reports success but
// produces nothing, and treating that as done means the step is skipped forever.
func IsNonEmptyFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

// IsDirectory determines if a file represented by `path` is a directory or not
func IsDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// CopyFile copies src to dst by way of dst+".partial" and a rename, so that dst either
// does not exist or is complete - never cut short by a failure or an interrupted run.
//
// That matters wherever the existence of dst is taken to mean the work is done: a
// truncated file there would be reused, and the work never redone.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	partial := dst + ".partial"
	out, err := os.Create(partial)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(partial, dst)
	}
	if err != nil {
		os.Remove(partial)
		return err
	}

	return nil
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
