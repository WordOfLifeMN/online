package ytpkg

import (
	"bytes"
	"strings"
	"testing"

	"github.com/WordOfLifeMN/online/util/box"
	"github.com/stretchr/testify/suite"
)

func TestPrintTestSuite(t *testing.T) {
	suite.Run(t, new(PrintTestSuite))
}

type PrintTestSuite struct {
	suite.Suite
}

// printMessage renders a package inside a box and returns its lines
func printMessage(m *Message) []string {
	var buf bytes.Buffer
	b := box.New(&buf)
	b.Header("")
	m.Print(b)
	b.Footer()
	return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
}

// Every value pasted into YouTube is a bare line of its own, exactly as it is in the
// package, so that selecting the line copies the value and nothing else
func (t *PrintTestSuite) TestPrint_CopyLinesAreExact() {
	pkg := Message{
		Channel:       ChannelWOL,
		Title:         "Pay Attention | Mary Peltz | October 4, 2026",
		Alternative:   "Pay Attention to God's Breadcrumbs | Mary Peltz | October 4, 2026",
		Description:   "A summary.\n\nSermon notes\nhttps://example.org/notes.pdf",
		Playlists:     []PlaylistRef{{Title: "Prayer", Position: 3}},
		Privacy:       PrivacyPublic,
		ThumbnailPath: `C:\Users\Someone\Documents\WOLMessages\2026-q4 Prayer\2026-10-04-pm Pay Attention.jpg`,
	}

	out := printMessage(&pkg)

	for _, value := range []string{
		pkg.Title,
		pkg.Alternative,
		"A summary.",
		"",
		"Sermon notes",
		"https://example.org/notes.pdf",
		"Prayer",
		pkg.ThumbnailPath,
	} {
		t.Contains(out, value)
	}
}

func (t *PrintTestSuite) TestPrint_SectionOrder() {
	pkg := Message{
		Channel:       ChannelWOL,
		Title:         "Title",
		Alternative:   "Alternative",
		Description:   "Description",
		Playlists:     []PlaylistRef{{Title: "Prayer", Position: 3}},
		Privacy:       PrivacyPublic,
		ThumbnailPath: "thumb.jpg",
	}

	out := strings.Join(printMessage(&pkg), "\n")

	positions := []int{}
	for _, marker := range []string{
		"├─ UPLOAD PACKET ",
		"│ Channel : WOL ",
		"│ Privacy : public ",
		"│ Playlist: Prayer (position 3) ",
		"│ Thumb   : ",
		"\nthumb.jpg\n",
		"├─ Titles ",
		"│ Title ",
		"\nTitle\n",
		"│ Alternative Title ",
		"\nAlternative\n",
		"├─ Description ",
		"\nDescription\n",
		"├─ Playlist ",
		"\nPrayer\n",
	} {
		index := strings.Index(out, marker)
		t.GreaterOrEqual(index, 0, "missing %q", marker)
		positions = append(positions, index)
	}
	t.IsIncreasing(positions)
}

// a message in several series gets a labeled line and a copy line for each playlist
func (t *PrintTestSuite) TestPrint_OneLinePerPlaylist() {
	pkg := Message{
		Title:       "T",
		Description: "D",
		Playlists: []PlaylistRef{
			{Title: "Prayer", Position: 3},
			{Title: "Women of Faith", Position: 7},
		},
	}

	out := strings.Join(printMessage(&pkg), "\n")
	t.Contains(out, "│ Playlist: Prayer (position 3) ")
	t.Contains(out, "│ Playlist: Women of Faith (position 7) ")
	t.Contains(out, "\nPrayer\nWomen of Faith\n")
	t.Less(strings.Index(out, "Prayer (position 3)"), strings.Index(out, "Women of Faith (position 7)"))
}

func (t *PrintTestSuite) TestPrint_PositionZeroIsOmitted() {
	pkg := Message{Playlists: []PlaylistRef{{Title: "Ask the Pastor"}}, Title: "T", Description: "D"}

	out := strings.Join(printMessage(&pkg), "\n")
	t.Contains(out, "│ Playlist: Ask the Pastor ")
	t.NotContains(out, "position")
}

func (t *PrintTestSuite) TestPrint_NoPlaylist() {
	pkg := Message{Title: "T", Description: "D"}

	out := strings.Join(printMessage(&pkg), "\n")
	t.NotContains(out, "Playlist")
}

func (t *PrintTestSuite) TestPrint_NoAlternativeOrThumbnail() {
	pkg := Message{Title: "T", Description: "D"}

	out := strings.Join(printMessage(&pkg), "\n")
	t.NotContains(out, "Alternative")
	t.NotContains(out, "Thumb")
}

// +---------------------------------------------------------------------------
// | Live video and playlist
// +---------------------------------------------------------------------------

func render(print func(b *box.Box)) string {
	var buf bytes.Buffer
	b := box.New(&buf)
	b.Header("")
	print(b)
	b.Footer()
	return buf.String()
}

func (t *PrintTestSuite) TestPrintLive() {
	pkg := Message{
		ID:                   "01LiNfRzMo8",
		ChannelID:            "UCZF_jDrfP6OltpviYmj-mkA",
		Title:                "CORE | Boundaries | Mary Peltz | October 4, 2026",
		Description:          "About boundaries.\n\nNotes\nhttps://x/notes.pdf",
		Privacy:              PrivacyUnlisted,
		CategoryID:           "29",
		Tags:                 []string{"faith", "core"},
		DefaultLanguage:      "en",
		DefaultAudioLanguage: "en",
		License:              "youtube",
		Embeddable:           true,
		ThumbnailURL:         "https://i.ytimg.com/vi/01LiNfRzMo8/maxresdefault.jpg",
		Playlists:            []PlaylistRef{{Title: "CORE | Healthy Limits", ID: "PL1", Position: 2}},
	}

	out := render(pkg.PrintLive)

	for _, marker := range []string{
		"├─ VIDEO ",
		"│ ID        : 01LiNfRzMo8 ",
		"│ Privacy   : unlisted ",
		"│ For kids  : no ",
		"│ Embeddable: yes ",
		"│ Tags      : faith, core ",
		"\nhttps://youtu.be/01LiNfRzMo8\n",
		"\nhttps://i.ytimg.com/vi/01LiNfRzMo8/maxresdefault.jpg\n",
		"├─ Title ",
		"\nCORE | Boundaries | Mary Peltz | October 4, 2026\n",
		"├─ Description ",
		"\nAbout boundaries.\n\nNotes\nhttps://x/notes.pdf\n",
		"├─ Playlists ",
		"│ PL1  CORE | Healthy Limits (position 2) ",
	} {
		t.Contains(out, marker)
	}
}

func (t *PrintTestSuite) TestPrintLive_NoPlaylists() {
	pkg := Message{ID: "01LiNfRzMo8", Title: "T"}
	t.Contains(render(pkg.PrintLive), "Not in any of this channel's playlists")
}

func (t *PrintTestSuite) TestSeriesPrint() {
	s := Series{
		ID:          "PL1",
		Title:       "CORE | Healthy Limits",
		Description: "About limits.",
		Privacy:     PrivacyPublic,
		Items: []PlaylistItem{
			{VideoID: "01LiNfRzMo8", Title: "CORE | Boundaries", Privacy: PrivacyPublic, Position: 0},
			{VideoID: "zQ3xYk1abcd", Title: "CORE | Saying No", Privacy: PrivacyPrivate, Position: 1},
		},
	}

	out := render(s.Print)

	for _, marker := range []string{
		"├─ PLAYLIST ",
		"│ ID        : PL1 ",
		"│ Items     : 2 ",
		"\nhttps://www.youtube.com/playlist?list=PL1\n",
		"\nCORE | Healthy Limits\n",
		"\nAbout limits.\n",
		"├─ Items ",
		"│   1  01LiNfRzMo8  public    CORE | Boundaries ",
		"│   2  zQ3xYk1abcd  private   CORE | Saying No ",
	} {
		t.Contains(out, marker)
	}
}
