//go:build !windows

package microsoft

import "os"

// readShared reads a whole file. Only Windows needs special care to read a workbook
// Excel has open; see open_windows.go.
func readShared(path string) ([]byte, error) {
	return os.ReadFile(path)
}
