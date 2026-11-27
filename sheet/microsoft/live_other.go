//go:build !windows

package microsoft

// readLive never finds Excel off Windows, so the file is always read
func readLive(path string) (*liveWorkbook, error) {
	return nil, errNotRunning
}
