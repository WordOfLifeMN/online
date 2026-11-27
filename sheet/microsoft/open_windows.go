//go:build windows

package microsoft

import (
	"errors"
	"io"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// readShared reads a whole file while another program, such as Excel, has it open.
//
// Excel holds an open workbook with DELETE access, because it saves by writing a new
// file and renaming it over the old one. Windows then refuses any other open that
// doesn't also share DELETE, and Go's os.Open doesn't (golang/go#32088). Opening with
// every share flag succeeds, and still asks only to read.
//
// The file is read in full and closed at once, so the handle is held for milliseconds
// and never stands in the way of Excel's or OneDrive's next save.
//
// A sharing violation can still happen at the moment OneDrive or Excel replaces the
// file, so it is retried for a couple of seconds before being reported.
func readShared(path string) ([]byte, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	deadline := time.Now().Add(sharingRetryFor)
	for {
		handle, err := windows.CreateFile(name,
			windows.GENERIC_READ, // read only, never write
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
			nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
		if err == nil {
			f := os.NewFile(uintptr(handle), path)
			defer f.Close()
			return io.ReadAll(f)
		}

		busy := errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION)
		if !busy || time.Now().After(deadline) {
			return nil, &os.PathError{Op: "open", Path: path, Err: err}
		}
		time.Sleep(sharingRetryEvery)
	}
}

// sharingRetryFor and sharingRetryEvery set how long a busy file is retried, and how often
var (
	sharingRetryFor   = 2 * time.Second
	sharingRetryEvery = 100 * time.Millisecond
)
