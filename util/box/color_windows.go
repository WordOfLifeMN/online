//go:build windows

package box

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableColor switches a Windows console into the mode that draws colour sequences
// rather than printing them as text. Windows Terminal has it on already; the older
// console host, which a double-clicked .bat can still open, needs asking. Reports
// false when the console cannot be switched, so the caller falls back to plain text.
func enableColor(f *os.File) bool {
	handle := windows.Handle(f.Fd())

	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return false
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return true
	}

	return windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}
