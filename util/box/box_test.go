package box

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/suite"
)

func TestBoxTestSuite(t *testing.T) {
	suite.Run(t, new(BoxTestSuite))
}

type BoxTestSuite struct {
	suite.Suite
}

// lines splits output into lines, dropping the final newline
func lines(buf *bytes.Buffer) []string {
	return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
}

// +---------------------------------------------------------------------------
// | Width
// +---------------------------------------------------------------------------

// The point of a closed box is that the right border lines up. Every drawn line must
// be exactly the box width, counted in runes, including lines whose text holds
// multi-byte characters - a byte count would come up short on every curly quote.
func (t *BoxTestSuite) TestEveryDrawnLineIsFullWidth() {
	var buf bytes.Buffer
	b := New(&buf)

	b.Header("Message #1")
	b.Line("short")
	b.Line("“curly quotes” and • bullets and an ellipsis…")
	b.Labeled("• ", strings.Repeat("“quoted” ", 30))
	b.Blank()
	b.Section("UPLOAD PACKET")
	b.Section("")
	b.Linef("Total: %d", 3)
	b.Header(strings.Repeat("long title ", 20))
	b.Footer()

	for _, line := range lines(&buf) {
		t.Equal(DefaultWidth, utf8.RuneCountInString(line), "line %q", line)
	}
}

func (t *BoxTestSuite) TestNarrowWidth() {
	var buf bytes.Buffer
	b := NewWidth(&buf, 20)

	b.Header("Title")
	b.Line("the quick brown fox jumps over the lazy dog")
	b.Footer()

	for _, line := range lines(&buf) {
		t.Equal(20, utf8.RuneCountInString(line), "line %q", line)
	}
}

// A buffer is never measured, so captured output does not depend on the terminal the
// tests happen to run in
func (t *BoxTestSuite) TestWidth_BufferGetsDefault() {
	t.T().Setenv("COLUMNS", "100")

	t.Equal(DefaultWidth, Width(&bytes.Buffer{}))
}

// notATerminal returns a file that cannot report a terminal size, so Width falls
// through to COLUMNS - the position mintty puts it in
func (t *BoxTestSuite) notATerminal() *os.File {
	f, err := os.CreateTemp(t.T().TempDir(), "out")
	t.Require().NoError(err)
	t.T().Cleanup(func() { f.Close() })
	return f
}

func (t *BoxTestSuite) TestWidth_Columns() {
	f := t.notATerminal()

	// one column spare, for consoles that wrap a line filling the last column
	t.T().Setenv("COLUMNS", "100")
	t.Equal(99, Width(f))

	t.T().Setenv("COLUMNS", " 90 ")
	t.Equal(89, Width(f), "surrounding space is tolerated")
}

// The spare column is taken before clamping, so the bounds are the box widths
// themselves rather than terminal widths
func (t *BoxTestSuite) TestWidth_ColumnsAtTheBounds() {
	f := t.notATerminal()

	t.T().Setenv("COLUMNS", "41")
	t.Equal(MinWidth, Width(f))

	t.T().Setenv("COLUMNS", "257")
	t.Equal(MaxWidth, Width(f))

	t.T().Setenv("COLUMNS", "258")
	t.Equal(MaxWidth, Width(f))
}

func (t *BoxTestSuite) TestWidth_ColumnsIsClamped() {
	f := t.notATerminal()

	t.T().Setenv("COLUMNS", "20")
	t.Equal(MinWidth, Width(f))

	t.T().Setenv("COLUMNS", "1000")
	t.Equal(MaxWidth, Width(f))
}

func (t *BoxTestSuite) TestWidth_UnreadableColumnsGetsDefault() {
	f := t.notATerminal()

	for _, value := range []string{"", "wide", "0", "-80"} {
		t.T().Setenv("COLUMNS", value)
		t.Equal(DefaultWidth, Width(f), "COLUMNS=%q", value)
	}
}

// +---------------------------------------------------------------------------
// | Rules
// +---------------------------------------------------------------------------

func (t *BoxTestSuite) TestTitlesSitInsideTheRule() {
	var buf bytes.Buffer
	b := NewWidth(&buf, 20)

	b.Header("Message #1")
	b.Section("Titles")
	b.Footer()

	t.Equal([]string{
		"╭─ Message #1 ─────╮",
		"├─ Titles ─────────┤",
		"╰──────────────────╯",
	}, lines(&buf))
}

func (t *BoxTestSuite) TestEmptyTitleGivesPlainRule() {
	var buf bytes.Buffer
	b := NewWidth(&buf, 10)

	b.Header("")
	b.Section("")

	t.Equal([]string{"╭────────╮", "├────────┤"}, lines(&buf))
}

// A title is cut short rather than pushing the corner out, and keeps at least one rule
// character before the corner so the end of the title does not look like the end of
// the box
func (t *BoxTestSuite) TestLongTitleIsCutShort() {
	var buf bytes.Buffer
	b := NewWidth(&buf, 20)

	b.Section("Platform risk notes (advisory - not for upload)")

	t.Equal([]string{"├─ Platform risk… ─┤"}, lines(&buf))
}

// A title that exactly fills the room is left alone
func (t *BoxTestSuite) TestTitleThatJustFits() {
	var buf bytes.Buffer
	b := NewWidth(&buf, 20)

	b.Section("0123456789abcd")

	t.Equal([]string{"├─ 0123456789abcd ─┤"}, lines(&buf))
}

// +---------------------------------------------------------------------------
// | Line
// +---------------------------------------------------------------------------

func (t *BoxTestSuite) TestLineWrapsAtSpaces() {
	var buf bytes.Buffer
	b := NewWidth(&buf, 20) // 16 characters of text

	b.Line("the quick brown fox jumps over the lazy dog")

	t.Equal([]string{
		"│ the quick brown  │",
		"│ fox jumps over   │",
		"│ the lazy dog     │",
	}, lines(&buf))
}

func (t *BoxTestSuite) TestLineStartsNewLineAtNewline() {
	var buf bytes.Buffer
	b := NewWidth(&buf, 20)

	b.Line("first\n\nthird")

	t.Equal([]string{
		"│ first            │",
		"│                  │",
		"│ third            │",
	}, lines(&buf))
}

func (t *BoxTestSuite) TestLineHardBreaksAnOverlongWord() {
	var buf bytes.Buffer
	b := NewWidth(&buf, 20)

	b.Line("see C:\\Users\\WordofLifeMNMedia\\Documents now")

	t.Equal([]string{
		"│ see              │",
		"│ C:\\Users\\WordofL │",
		"│ ifeMNMedia\\Docum │",
		"│ ents now         │",
	}, lines(&buf))
}

// Spacing that lines things up must survive on a line that fits
func (t *BoxTestSuite) TestLineThatFitsKeepsItsSpacing() {
	var buf bytes.Buffer
	b := NewWidth(&buf, 20)

	b.Line("Thumb   :")
	b.Labeled("Total: ", "    4s")

	t.Equal([]string{
		"│ Thumb   :        │",
		"│ Total:     4s    │",
	}, lines(&buf))
}

func (t *BoxTestSuite) TestEmptyLineIsBlank() {
	var buf bytes.Buffer
	b := NewWidth(&buf, 10)

	b.Line("")
	b.Blank()

	t.Equal([]string{"│        │", "│        │"}, lines(&buf))
}

// +---------------------------------------------------------------------------
// | Labeled
// +---------------------------------------------------------------------------

func (t *BoxTestSuite) TestLabeledLinesUpContinuations() {
	var buf bytes.Buffer
	b := NewWidth(&buf, 24) // 20 characters of text, 12 beside the label

	b.Labeled("Reason  : ", "no spreadsheet row is dated 2026-10-11")

	t.Equal([]string{
		"│ Reason  : no         │",
		"│           spreadshee │",
		"│           t row is   │",
		"│           dated      │",
		"│           2026-10-11 │",
	}, lines(&buf))
}

func (t *BoxTestSuite) TestLabeledShortValue() {
	var buf bytes.Buffer
	b := NewWidth(&buf, 24)

	b.Labeled("Channel : ", "WOL")

	t.Equal([]string{"│ Channel : WOL        │"}, lines(&buf))
}

// +---------------------------------------------------------------------------
// | Copy
// +---------------------------------------------------------------------------

// A copy line is the value exactly, so selecting it picks up nothing that is not part
// of what gets pasted - no border, no padding, and none of the wrapping that would
// otherwise put line breaks into a long title
func (t *BoxTestSuite) TestCopyIsExact() {
	var buf bytes.Buffer
	b := NewWidth(&buf, 20)

	description := "A summary far longer than the box is wide, left unwrapped.\n\n" +
		"Sermon notes\nhttps://example.org/notes.pdf"

	b.Header("")
	b.Copy(description)
	b.Footer()

	t.Equal("╭──────────────────╮\n"+description+"\n╰──────────────────╯\n", buf.String())
}
