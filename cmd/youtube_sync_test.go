package cmd

import (
	"bufio"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/ytpkg"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withInput feeds a test's prompts from a string, as the AudioTestSuite's withStdin
// does
func withInput(t *testing.T, input string, fn func()) {
	original := os.Stdin
	defer func() { os.Stdin = original }()

	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	os.Stdin = reader
	resetStdin()
	defer resetStdin()
	go func() {
		writer.WriteString(input)
		writer.Close()
	}()

	fn()
}

// withYes sets --yes for one test
func withYes(t *testing.T, yes bool) {
	viper.Set("yes", yes)
	t.Cleanup(func() { viper.Set("yes", false) })
}

// +---------------------------------------------------------------------------
// | The video-ID index
// +---------------------------------------------------------------------------

func row(name, url string) catalog.CatalogMessage {
	msg := catalog.CatalogMessage{Name: name}
	if url != "" {
		msg.Video = &catalog.OnlineResource{URL: url}
	}
	return msg
}

func TestIndexByVideoID(t *testing.T) {
	cat := &catalog.Catalog{Messages: []catalog.CatalogMessage{
		row("Plain", "https://youtu.be/01LiNfRzMo8"),
		row("Live link", "https://youtube.com/live/3oXvpnL75Lg?feature=share"),
		row("Malformed", "https://youtu.bea4fdKX1SCq8"),
		row("Second service", "https://www.youtube.com/watch?v=01LiNfRzMo8"),
		row("Rumble", "https://rumble.com/v1abcde-some-message.html"),
		row("S3", "https://s3.us-west-2.amazonaws.com/wordoflife.mn.video/2020-01-05.mp4"),
		row("No video", ""),
		row("Cassette era", "-"),
	}}

	index := indexByVideoID(cat)

	assert.Len(t, index, 3)
	require.Len(t, index["01LiNfRzMo8"], 2, "two rows share one video")
	assert.Equal(t, "Plain", index["01LiNfRzMo8"][0].Name)
	assert.Equal(t, "Second service", index["01LiNfRzMo8"][1].Name)
	assert.Equal(t, "Live link", index["3oXvpnL75Lg"][0].Name)
	assert.Equal(t, "Malformed", index["a4fdKX1SCq8"][0].Name)
}

func TestNameFromTitle(t *testing.T) {
	assert.Equal(t, "Boundaries", nameFromTitle("CORE | Boundaries | Mary Peltz | October 4, 2026"))
	assert.Equal(t, "Pay Attention", nameFromTitle("Pay Attention | Mary Peltz | October 4, 2026"))
	assert.Equal(t, "An old title", nameFromTitle("An old title"))
	assert.Equal(t, "CORE", nameFromTitle("CORE"))
}

// rows sharing a video are offered with the one matching the live title first
func TestChooseRow_RanksByLiveTitle(t *testing.T) {
	run := &syncRun{
		out: os.Stdout,
		byVideo: map[string][]catalog.CatalogMessage{
			"01LiNfRzMo8": {{Name: "Power of Prayer"}, {Name: "Boundaries"}},
		},
	}
	live := &ytpkg.Message{ID: "01LiNfRzMo8", Title: "CORE | Boundaries | Mary Peltz | October 4, 2026"}

	withInput(t, "\n", func() {
		chosen := run.chooseRow("01LiNfRzMo8", live)
		require.NotNil(t, chosen)
		assert.Equal(t, "Boundaries", chosen.Name)
	})
}

func TestChooseRow_NotInSheet(t *testing.T) {
	run := &syncRun{out: os.Stdout, byVideo: map[string][]catalog.CatalogMessage{}}
	assert.Nil(t, run.chooseRow("01LiNfRzMo8", &ytpkg.Message{Title: "T"}))
}

// +---------------------------------------------------------------------------
// | Approval defaults (Q13, Q19)
// +---------------------------------------------------------------------------

func TestConfirm_EnterTakesTheDefault(t *testing.T) {
	withYes(t, false)
	withInput(t, "\n", func() { assert.True(t, confirm("Apply?", true, true)) })
	withInput(t, "\n", func() { assert.False(t, confirm("Apply?", false, false)) })
}

func TestConfirm_Answers(t *testing.T) {
	withYes(t, false)
	withInput(t, "y\n", func() { assert.True(t, confirm("Apply?", false, false)) })
	withInput(t, "YES\n", func() { assert.True(t, confirm("Apply?", false, false)) })
	withInput(t, "n\n", func() { assert.False(t, confirm("Apply?", true, true)) })
	withInput(t, "maybe\ny\n", func() { assert.True(t, confirm("Apply?", false, false)) })
}

// with nobody at the keyboard nothing is applied, even where Enter would say yes
func TestConfirm_ClosedInputIsNo(t *testing.T) {
	withYes(t, false)
	withInput(t, "", func() { assert.False(t, confirm("Apply?", true, true)) })
}

func TestConfirm_YesFlag(t *testing.T) {
	withYes(t, true)

	// honoured: no input is read
	withInput(t, "", func() { assert.True(t, confirm("Apply?", true, true)) })

	// not honoured: the question is still asked, and closed input is no
	withInput(t, "", func() { assert.False(t, confirm("Create playlist?", false, false)) })
	withInput(t, "y\n", func() { assert.True(t, confirm("Create playlist?", false, false)) })
}

func TestApprove_Defaults(t *testing.T) {
	run := &syncRun{}
	plain := []ytpkg.Change{{Field: ytpkg.FieldTitle, Now: "a", New: "b"}}
	destructive := []ytpkg.Change{{Field: ytpkg.FieldDescription, Now: "a", New: "b", Destructive: true}}

	withYes(t, false)
	withInput(t, "\n", func() { assert.True(t, run.approve(plain), "Enter is yes") })
	withInput(t, "\n", func() { assert.False(t, run.approve(destructive), "Enter is no") })

	withYes(t, true)
	withInput(t, "", func() { assert.True(t, run.approve(plain), "--yes answers") })
	withInput(t, "", func() { assert.False(t, run.approve(destructive), "--yes does not answer") })
}

// +---------------------------------------------------------------------------
// | Playlist membership
// +---------------------------------------------------------------------------

func TestPlaylistStep_Change(t *testing.T) {
	ref := ytpkg.PlaylistRef{Title: "CORE | Healthy Limits", Position: 3}

	assert.Equal(t, `add to "CORE | Healthy Limits" (track 3)`,
		playlistStep{ref: ref, playlistID: "PL1"}.change().Summary)

	assert.Equal(t, `create "CORE | Healthy Limits" and add (track 3)`,
		playlistStep{ref: ref}.change().Summary)

	assert.Equal(t, `create "CORE | Healthy Limits" and add (track 3). `+
		`Similar playlists already exist: "CORE: Healthy Limits"`,
		playlistStep{ref: ref, near: []*ytpkg.Series{{Title: "CORE: Healthy Limits"}}}.change().Summary)

	assert.Contains(t, playlistStep{ref: ref, duplicates: 2}.change().Summary,
		"2 playlists are titled")

	assert.Equal(t, `add to "Ask the Pastor"`,
		playlistStep{ref: ytpkg.PlaylistRef{Title: "Ask the Pastor"}, playlistID: "PL2"}.change().Summary)
}

func TestNewPlaylistFor(t *testing.T) {
	cat := &catalog.Catalog{Series: []catalog.CatalogSeri{{
		Name:        "Healthy Limits",
		Description: "About limits.",
		Visibility:  catalog.Partner,
		Booklets:    []catalog.OnlineResource{{URL: "https://x/booklet.pdf", Name: "Booklet"}},
		Messages:    []catalog.CatalogMessage{{Ministry: catalog.CenterOfRelationshipExperience}},
	}}}
	run := &syncRun{cat: cat}
	msg := &catalog.CatalogMessage{
		Ministry:   catalog.CenterOfRelationshipExperience,
		Visibility: catalog.Public,
		Series:     []catalog.SeriesReference{{Name: "Healthy Limits", Index: 2}},
	}

	series, err := run.newPlaylistFor(msg, ytpkg.PlaylistRef{Title: "CORE | Healthy Limits", Position: 2})
	require.NoError(t, err)
	assert.Equal(t, "CORE | Healthy Limits", series.Title)
	assert.Equal(t, "About limits.\n\nBooklet\nhttps://x/booklet.pdf", series.Description)
	assert.Equal(t, ytpkg.PrivacyUnlisted, series.Privacy, "the series' visibility, not the message's")

	// no series behind the playlist: the message's privacy, no description
	atp := &catalog.CatalogMessage{Ministry: catalog.AskThePastor, Visibility: catalog.Public}
	series, err = run.newPlaylistFor(atp, ytpkg.PlaylistRef{Title: "Ask the Pastor"})
	require.NoError(t, err)
	assert.Equal(t, &ytpkg.Series{Title: "Ask the Pastor", Privacy: ytpkg.PrivacyPublic}, series)
}

// a new playlist for a series with no Series row is not described from a stand-alone
// series that happens to share its name
func TestNewPlaylistFor_StandAloneNameCollision(t *testing.T) {
	cat := &catalog.Catalog{Messages: []catalog.CatalogMessage{
		{Name: "Voting", Ministry: catalog.WordOfLife, Visibility: catalog.Public, Description: "Not this."},
		{
			Name: "The Importance of Voting", Ministry: catalog.WordOfLife, Visibility: catalog.Partner,
			Series: []catalog.SeriesReference{{Name: "Voting", Index: 1}},
		},
	}}
	require.NoError(t, cat.Initialize())
	run := &syncRun{cat: cat}

	series, err := run.newPlaylistFor(&cat.Messages[1], ytpkg.PlaylistRef{Title: "Voting", Position: 1})
	require.NoError(t, err)
	assert.Equal(t, &ytpkg.Series{Title: "Voting", Privacy: ytpkg.PrivacyUnlisted}, series)
}

func TestPrintManualReminder(t *testing.T) {
	var buf bytes.Buffer
	printManualReminder(&buf, &ytpkg.Series{ID: "PLnew1", Title: "CORE | Healthy Limits"}, "Then re-run sync.")

	out := buf.String()
	assert.Contains(t, out, `Set Video ordering to "Manual" in Studio`)
	assert.Contains(t, out, "│ Playlist: CORE | Healthy Limits ")
	assert.Contains(t, out, "\nhttps://studio.youtube.com/playlist/PLnew1/edit\n")
}

func TestPrintSummary(t *testing.T) {
	var buf bytes.Buffer
	run := &syncRun{out: &buf}

	run.printSummary()
	assert.Empty(t, buf.String(), "nothing to do, nothing printed")

	run.created = []*ytpkg.Series{{ID: "PL1", Title: "One"}, {ID: "PL2", Title: "Two"}}
	run.printSummary()
	out := buf.String()
	assert.Contains(t, out, "To do in Studio")
	assert.Contains(t, out, "\nhttps://studio.youtube.com/playlist/PL1/edit\n")
	assert.Contains(t, out, "\nhttps://studio.youtube.com/playlist/PL2/edit\n")
}

func TestDropPlaylistSnapshot(t *testing.T) {
	viper.Set("scratch-dir", t.TempDir())
	t.Cleanup(func() { viper.Set("scratch-dir", "") })
	run := &syncRun{channel: ytpkg.ChannelWOL}

	path := playlistSnapshotPath(ytpkg.ChannelWOL)
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0o644))

	run.dropPlaylistSnapshot()
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err))

	run.dropPlaylistSnapshot() // already gone is fine
}

func refsFrom(input string) []string {
	return promptForRefs(bufio.NewReader(strings.NewReader(input)))
}

// every way a selection leaves a spreadsheet: a column is a cell per line, with
// Windows or Unix line endings; a row is cells separated by tabs; typed values are
// separated by spaces
func TestPromptForRefs_SpreadsheetPastes(t *testing.T) {
	want := []string{"https://youtu.be/01LiNfRzMo8", "https://youtu.be/16UKsDSvn3s", "2YsVzfbrtm8"}

	assert.Equal(t, want, refsFrom("https://youtu.be/01LiNfRzMo8\nhttps://youtu.be/16UKsDSvn3s\n2YsVzfbrtm8\n\n"))
	assert.Equal(t, want, refsFrom("https://youtu.be/01LiNfRzMo8\r\nhttps://youtu.be/16UKsDSvn3s\r\n2YsVzfbrtm8\r\n\r\n"))
	assert.Equal(t, want, refsFrom("https://youtu.be/01LiNfRzMo8\thttps://youtu.be/16UKsDSvn3s\t2YsVzfbrtm8\n\n"))
	assert.Equal(t, want, refsFrom("  https://youtu.be/01LiNfRzMo8   https://youtu.be/16UKsDSvn3s\n2YsVzfbrtm8\n\n"))

	// a cell holding a line break is copied in quotes
	assert.Equal(t, want, refsFrom("\"https://youtu.be/01LiNfRzMo8\"\nhttps://youtu.be/16UKsDSvn3s\n'2YsVzfbrtm8'\n\n"))
}

// input ends at an empty line - what follows is left for later prompts - or when
// stdin closes, with or without a final line break
func TestPromptForRefs_Ending(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, refsFrom("a\nb\n\nc\n"))
	assert.Equal(t, []string{"a", "b"}, refsFrom("a\nb"))
	assert.Empty(t, refsFrom("\n"))
	assert.Empty(t, refsFrom(""))
	assert.Empty(t, refsFrom("   \t \n"), "a line of only whitespace is empty")
}
