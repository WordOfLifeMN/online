package box

import (
	"io"
	"os"

	"golang.org/x/term"
)

// The colours are the plain ANSI ones rather than bright or 256-colour ones, so they
// read on both light and dark terminal themes - the terminal's own palette decides
// exactly which yellow it is.
const (
	// yellow is the colour for warnings
	yellow = "\x1b[33m"
	green  = "\x1b[32m"
	red    = "\x1b[31m"
)

// ColorEnabled reports whether colour sequences written to w will be drawn as colour.
// Colour is only for a person at a terminal: output that is piped, redirected to a
// file, or written to a test's buffer gets plain text, with no stray escape codes in
// it.
//
// NO_COLOR, set to anything, turns colour off everywhere (https://no-color.org).
//
// The window Git Bash opens (mintty) is not a Windows console and is not detected as
// a terminal, so it gets no colour. Windows Terminal and the VSCode terminal do.
func ColorEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	f, ok := w.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return false
	}

	return enableColor(f)
}

// Yellow returns s in the warning colour when w draws colour, and s unchanged when it
// does not. For a warning written outside a box:
//
//	fmt.Println(box.Yellow(os.Stdout, "WARNING: ..."))
func Yellow(w io.Writer, s string) string {
	return paintFor(w, yellow, s)
}

// Green returns s in green when w draws colour, and s unchanged when it does not
func Green(w io.Writer, s string) string {
	return paintFor(w, green, s)
}

// Red returns s in red when w draws colour, and s unchanged when it does not
func Red(w io.Writer, s string) string {
	return paintFor(w, red, s)
}

// paintFor wraps s in color when w draws colour. An empty s has nothing to colour.
func paintFor(w io.Writer, color, s string) string {
	if s == "" || !ColorEnabled(w) {
		return s
	}
	return color + s + reset
}
