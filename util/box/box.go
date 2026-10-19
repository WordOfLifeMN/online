// Package box draws closed boxes of Unicode box-drawing characters on the console:
//
//	╭─ Title ─────────╮
//	│ bordered text   │
//	├─ Section ───────┤
//	a bare copy line
//	╰─────────────────╯
//
// Bordered lines are padded to the right edge and wrapped to fit inside it. Copy lines
// are the exception: they are written bare, with no border and no wrapping, so a value
// meant to be pasted somewhere else can be selected in one gesture without picking up
// a border character or a line break that is not part of it.
//
// Widths are counted in runes. That is right for the box-drawing characters and for
// ordinary punctuation such as curly quotes and bullets, but not for characters a
// terminal draws double width - emoji, CJK - which push the right border out of line.
// Keep those out of bordered text.
//
// Colour and style sequences (ESC [ ... m) take up no room on screen, so they are not
// counted, and bordered text may carry them. A bordered line holding one is reset
// before the padding, so the colour never bleeds into the border - which also means a
// colour does not carry on to the next line when text wraps. Colour a word or a short
// phrase, not a run of text that may wrap.
package box

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

// widths of a box, borders included
const (
	// DefaultWidth is used when the terminal's width cannot be measured
	DefaultWidth = 132

	// MinWidth and MaxWidth bound a measured width. Below the minimum a box is mostly
	// border; beyond the maximum a label sits so far from the right border that the
	// eye loses which line it belongs to.
	MinWidth = 40
	MaxWidth = 256
)

// sgr matches a terminal colour or style sequence - ESC [ params m - such as
// "\x1b[31m" for red, "\x1b[1;31m" for bold red, "\x1b[38;5;196m" for one of 256
// colours, or "\x1b[0m" and "\x1b[m" for a reset. Their lengths vary, so they are
// matched rather than counted.
var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// reset turns colour and style back off
const reset = "\x1b[0m"

// minWidth leaves room for the borders, their padding, and one character of text. It
// bounds an explicit width, which is not held to MinWidth.
const minWidth = 5

// Box draws one box to a writer, line by line. It holds no buffered state, so lines
// appear as soon as they are written.
//
// Errors from the writer are ignored, as fmt.Printf ignores them when writing to the
// console.
type Box struct {
	w     io.Writer
	width int
}

// New returns a box as wide as the terminal it writes to - see Width. The width is
// measured here, once, so resizing the window affects the next box rather than one
// that is half drawn.
func New(w io.Writer) *Box {
	return NewWidth(w, Width(w))
}

// NewWidth returns a box of the given full width, borders included. The width is not
// held to MinWidth and MaxWidth, which bound only a measured width.
func NewWidth(w io.Writer, width int) *Box {
	return &Box{w: w, width: max(width, minWidth)}
}

// Width is the box width for output to w, from the first of these that answers:
//
//  1. the width the terminal reports
//  2. the COLUMNS environment variable, which some terminals set where they cannot
//     be asked - mintty, the window Git Bash opens, is not a Windows console
//  3. DefaultWidth
//
// A measured width is held between MinWidth and MaxWidth - see fitTerminal.
//
// Only an *os.File is measured. Anything else - a test's bytes.Buffer, say - is not a
// terminal, and its output should not depend on the terminal the test ran in.
func Width(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return DefaultWidth
	}

	if columns, _, err := term.GetSize(int(f.Fd())); err == nil && columns > 0 {
		return fitTerminal(columns)
	}
	if columns, err := strconv.Atoi(strings.TrimSpace(os.Getenv("COLUMNS"))); err == nil && columns > 0 {
		return fitTerminal(columns)
	}
	return DefaultWidth
}

// fitTerminal turns a terminal's width in columns into a box width: one column
// narrower, held between MinWidth and MaxWidth.
//
// The spare column is for consoles that wrap as soon as a character lands in the last
// column. On those, a line that fills the window and then ends in a newline moves down
// two lines rather than one, and every line of the box comes out double spaced.
// Windows Terminal waits to see whether more text follows before wrapping, but not
// every console does, and a column is a small price for not finding out which.
func fitTerminal(columns int) int {
	return min(max(columns-1, MinWidth), MaxWidth)
}

// Header writes the top edge, ╭─ title ───╮, or a plain ╭────╮ when title is ""
func (b *Box) Header(title string) {
	b.writeln(b.rule("╭", title, "╮"))
}

// Section writes a divider, ├─ title ───┤, or a plain ├────┤ when title is ""
func (b *Box) Section(title string) {
	b.writeln(b.rule("├", title, "┤"))
}

// Footer writes the bottom edge, ╰────╯
func (b *Box) Footer() {
	b.writeln(b.rule("╰", "", "╯"))
}

// Blank writes an empty bordered line
func (b *Box) Blank() {
	b.bordered("")
}

// Line writes text as bordered lines, wrapped at spaces to fit inside the box. A "\n"
// in the text starts a new line.
func (b *Box) Line(text string) {
	for _, line := range wrap(text, b.inner()) {
		b.bordered(line)
	}
}

// Linef is Line with formatting
func (b *Box) Linef(format string, a ...any) {
	b.Line(fmt.Sprintf(format, a...))
}

// Labeled writes the label followed directly by the value, so the label carries its
// own separator: Labeled("Channel : ", "WOL"). When the value wraps, continuation lines
// are indented to line up under the start of the value.
func (b *Box) Labeled(label, value string) {
	indent := visibleWidth(label)
	if indent >= b.inner() {
		// no room left beside the label for the value, so give up on aligning it
		b.Line(label + value)
		return
	}

	for index, line := range wrap(value, b.inner()-indent) {
		if index == 0 {
			b.bordered(label + line)
		} else {
			b.bordered(strings.Repeat(" ", indent) + line)
		}
	}
}

// Copy writes text bare - no border, no wrapping, no padding - exactly as given,
// embedded newlines and blank lines included, followed by a newline
func (b *Box) Copy(text string) {
	b.writeln(text)
}

// inner is the width available for text between "│ " and " │"
func (b *Box) inner() int {
	return b.width - 4
}

// bordered writes one line of text that is known to fit, padded to the right border.
// Text holding a colour is reset before the padding, so that neither the padding nor
// the border is drawn in it.
func (b *Box) bordered(text string) {
	if sgr.MatchString(text) {
		text += reset
	}
	b.writeln("│ " + pad(text, b.inner()) + " │")
}

func (b *Box) writeln(line string) {
	fmt.Fprintln(b.w, line)
}

// rule draws a horizontal edge with an optional title inside it. A title too long for
// the edge is cut short with "…", leaving at least one rule character before the
// corner so the end of the title is not mistaken for the end of the box.
func (b *Box) rule(left, title, right string) string {
	span := b.width - 2 // between the two corners
	if title == "" {
		return left + strings.Repeat("─", span) + right
	}

	// "─ " before the title, " ─" after it at the least
	room := span - 4
	if visibleWidth(title) > room {
		// cut without its colours, which a cut could leave switched on along the rule
		title = string([]rune(sgr.ReplaceAllString(title, ""))[:max(room-1, 0)]) + "…"
	} else if sgr.MatchString(title) {
		title += reset
	}

	tail := span - 3 - visibleWidth(title)
	return left + "─ " + title + " " + strings.Repeat("─", tail) + right
}

// visibleWidth is the number of runes a terminal draws for s: its colour sequences
// take up no room, so they are not counted
func visibleWidth(s string) int {
	if !strings.Contains(s, "\x1b") {
		return utf8.RuneCountInString(s)
	}
	return utf8.RuneCountInString(sgr.ReplaceAllString(s, ""))
}

// splitVisible cuts s after n visible runes. A colour sequence is never cut in two:
// those up to and immediately after the cut stay with the head, so a reset that ends
// the head's colour stays with it.
func splitVisible(s string, n int) (string, string) {
	count, i := 0, 0
	for i < len(s) {
		if s[i] == '\x1b' {
			if loc := sgr.FindStringIndex(s[i:]); loc != nil && loc[0] == 0 {
				i += loc[1]
				continue
			}
		}
		if count == n {
			break
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
		count++
	}
	return s[:i], s[i:]
}

// pad fills s with spaces out to width visible runes
func pad(s string, width int) string {
	if n := visibleWidth(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}

// wrap breaks text into lines no wider than width visible runes. Each "\n" starts a
// new line, so an empty paragraph becomes an empty line.
//
// A paragraph that fits is left exactly as it is, so spacing used for alignment - a
// right-aligned column, "Thumb   :" lined up under "Channel :" - survives. One that
// does not fit breaks at spaces, and runs of spaces between its words collapse to one;
// a word wider than a whole line is broken at the edge, since there is no space to
// break it at.
func wrap(text string, width int) []string {
	var lines []string

	for _, paragraph := range strings.Split(text, "\n") {
		if visibleWidth(paragraph) <= width {
			lines = append(lines, paragraph)
			continue
		}

		// line is the text so far, and lineWidth how much of it is visible. They
		// differ by the colour sequences, which take up no room.
		var line string
		var lineWidth int

		for _, word := range strings.Fields(paragraph) {
			wordWidth := visibleWidth(word)

			// start a new line when this word will not fit after a space
			if lineWidth > 0 && lineWidth+1+wordWidth > width {
				lines = append(lines, line)
				line, lineWidth = "", 0
			}

			// a word wider than a whole line is broken wherever it hits the edge
			for wordWidth > width {
				if lineWidth > 0 {
					lines = append(lines, line)
				} else {
					// nothing visible yet, but keep any colour it switched on
					word = line + word
				}
				line, lineWidth = "", 0

				head, tail := splitVisible(word, width)
				lines = append(lines, head)
				word, wordWidth = tail, wordWidth-width
			}

			if lineWidth > 0 {
				line += " "
				lineWidth++
			}
			line += word
			lineWidth += wordWidth
		}

		lines = append(lines, line)
	}

	return lines
}
