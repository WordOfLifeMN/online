package cmd

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestMessageTypeTestSuite(t *testing.T) {
	suite.Run(t, new(MessageTypeTestSuite))
}

type MessageTypeTestSuite struct {
	suite.Suite
}

// +---------------------------------------------------------------------------
// | Title and date inference
// +---------------------------------------------------------------------------

// The file name is <date>[<discriminator>][-<obsolete markup>] <title>.<ext>. Only the
// date and the title are read; everything between them is tolerated and discarded,
// whatever it once meant.
func (t *MessageTypeTestSuite) TestTitleInference() {
	cases := map[string]string{
		"2026-03-08 Walking in Faith.mp4":     "Walking in Faith",
		"2026-10-04 Importance of Voting.mp4": "Importance of Voting",

		// obsolete speaker initials, leading and trailing
		"2026-03-08-v Walking in Faith.mp4": "Walking in Faith",
		"2026-03-08 Walking in Faith-v.mp4": "Walking in Faith",

		// obsolete prayer markers, with and without a speaker initial
		"2026-03-08-vp Opening Prayer.mp4": "Opening Prayer",
		"2026-03-08p Opening Prayer.mp4":   "Opening Prayer",

		// a discriminator letter keeps two files of one date apart on disk, and may be
		// followed by obsolete markup of any length
		"2026-10-11b-mp Learning at Moriah.mp4": "Learning at Moriah",
		"2026-10-11b Learning at Moriah.mp4":    "Learning at Moriah",
		"2026-10-11-mp Learning at Moriah.mp4":  "Learning at Moriah",

		// a title opening with a short word keeps it - the markers are consumed before
		// the gap, so there is nothing left to eat into the title
		"2026-03-08-v A New Day.mp4": "A New Day",
		"2026-03-08 A New Day.mp4":   "A New Day",

		// a title ending in a short word keeps it, because the trailing initial
		// convention requires the hyphen
		"2026-03-08 The Great I Am.mp4": "The Great I Am",

		// a full path is fine, and so is a date with nothing after it
		`C:\Videos\2026-10-04 The Works of God.mp4`: "The Works of God",
		"2026-03-08.mp4": "",
	}

	for name, expected := range cases {
		t.Equal(expected, getTitleFromFileName(name), "file %q", name)
	}
}

func (t *MessageTypeTestSuite) TestDateInference() {
	date, err := getDateFromFileName("2026-03-08-v Walking in Faith.mp4")
	t.NoError(err)
	t.Equal("2026-03-08", date.String())
}

func (t *MessageTypeTestSuite) TestDateInference_NoDate() {
	_, err := getDateFromFileName("Walking in Faith.mp4")
	t.Error(err)
	t.Contains(err.Error(), "2025-03-09")
}

// +---------------------------------------------------------------------------
// | Model output sanitising
// +---------------------------------------------------------------------------

// Both of these were produced by claude-opus-5 on a real message and would have
// reached the YouTube description unaltered.
func (t *MessageTypeTestSuite) TestSanitize_LiteralUnicodeEscape() {
	// the model escaped the backslash in its own JSON, so correct parsing leaves the
	// seven characters — rather than an em dash
	in := `seven keys—leading yourself, and staying teachable—plus the model`
	t.Equal("seven keys—leading yourself, and staying teachable—plus the model",
		sanitizeModelText(in))
}

func (t *MessageTypeTestSuite) TestSanitize_EmbeddedNewlines() {
	// a summary that breaks its own lines scrambles the description layout, which
	// uses blank lines to separate the summary from the resource links
	in := "He walked through seven keys\ninjecting humor along the way\nlike leading yourself."
	t.Equal("He walked through seven keys injecting humor along the way like leading yourself.",
		sanitizeModelText(in))
}

func (t *MessageTypeTestSuite) TestSanitize_LeavesGoodTextAlone() {
	// real em dashes, apostrophes and emoji must survive untouched
	in := "Real leadership is humble service—modeling what you want to see. 🙌"
	t.Equal(in, sanitizeModelText(in))
}

func (t *MessageTypeTestSuite) TestSanitize_CollapsesAndTrims() {
	t.Equal("one two three", sanitizeModelText("  one   two \t three\r\n "))
	t.Equal("", sanitizeModelText("   \n\t  "))
}

func (t *MessageTypeTestSuite) TestSanitize_MalformedEscapeIsLeftAsIs() {
	// not a valid \uXXXX - must not be mangled further
	t.Equal(`a \u20 b \uZZZZ c`, sanitizeModelText(`a \u20 b \uZZZZ c`))
}
