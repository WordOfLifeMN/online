package box

import (
	"io"
	"os"

	"golang.org/x/term"
)

// yellow is the colour for warnings. It is the plain ANSI yellow rather than a bright
// or 256-colour one, so it reads on both light and dark terminal themes - the
// terminal's own palette decides exactly which yellow it is.
const yellow = "\x1b[33m"

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
	if s == "" || !ColorEnabled(w) {
		return s
	}
	return yellow + s + reset
}
