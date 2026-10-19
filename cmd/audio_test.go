package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/util"
	"github.com/WordOfLifeMN/online/util/box"
	"github.com/WordOfLifeMN/online/ytpkg"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
)

func TestAudioTestSuite(t *testing.T) {
	suite.Run(t, new(AudioTestSuite))
}

type AudioTestSuite struct {
	suite.Suite
}

// withStdin runs fn with os.Stdin replaced by the given input
func (t *AudioTestSuite) withStdin(input string, fn func()) {
	original := os.Stdin
	defer func() { os.Stdin = original }()

	reader, writer, err := os.Pipe()
	t.Require().NoError(err)

	os.Stdin = reader
	resetStdin()
	defer resetStdin()
	go func() {
		writer.WriteString(input)
		writer.Close()
	}()

	fn()
}

// newAmbiguity builds the real-world shape: a message and its Q&A on the same date,
// neither in a series, so both carry track 0 and nothing but asking can tell them apart
func (t *AudioTestSuite) newAmbiguity() *catalog.AmbiguousLookupError {
	return &catalog.AmbiguousLookupError{
		Lookup: catalog.MessageLookup{
			Date: catalog.MustParseDateOnly("2026-05-17"),
		},
		Candidates: []catalog.CatalogMessage{
			{Name: "Power of Prayer", Speakers: []string{"Pastor Vern Peltz"}},
			{Name: "Power of Prayer: Q&A", Speakers: []string{"Pastor Vern Peltz"}},
		},
	}
}

// +---------------------------------------------------------------------------
// | Speaker resolution
// +---------------------------------------------------------------------------

// Each case that expects a name without supplying input runs with stdin closed. Were
// the prompt still being reached, the read would fail and the result would be empty
// rather than the expected name - so these tests prove the question is skipped, not
// merely answered.

func (t *AudioTestSuite) TestSpeaker_SpreadsheetIsAuthoritative() {
	msg := &catalog.CatalogMessage{Speakers: []string{"Pastor Mary Peltz"}}

	t.withStdin("", func() {
		t.Equal("Pastor Mary Peltz", resolveSpeaker(msg))
	})
}

func (t *AudioTestSuite) TestSpeaker_MultipleSpeakersFromSheet() {
	msg := &catalog.CatalogMessage{
		Speakers: []string{"Pastor Vern Peltz", "Pastor Mary Peltz"},
	}

	t.withStdin("", func() {
		t.Equal("Pastor Vern Peltz, Pastor Mary Peltz", resolveSpeaker(msg))
	})
}

// --speaker is the operator overriding everything, including the sheet
func (t *AudioTestSuite) TestSpeaker_FlagOverridesSpreadsheet() {
	viper.Set("speaker", "Guest Preacher")
	defer viper.Set("speaker", "")

	msg := &catalog.CatalogMessage{Speakers: []string{"Pastor Vern Peltz"}}

	t.withStdin("", func() {
		t.Equal("Guest Preacher", resolveSpeaker(msg))
	})
}

// a row that names nobody is not an answer, so it falls through to the prompt rather
// than resolving to an empty speaker
func (t *AudioTestSuite) TestSpeaker_EmptyRowFallsThrough() {
	msg := &catalog.CatalogMessage{}

	t.withStdin("m\n", func() {
		t.Equal("Pastor Mary Peltz", resolveSpeaker(msg))
	})
}

// no row at all - the lookup failed, or the sheet was unreadable - so ask
func (t *AudioTestSuite) TestSpeaker_PromptsWhenNothingKnown() {
	t.withStdin("Guest Preacher\n", func() {
		t.Equal("Guest Preacher", resolveSpeaker(nil))
	})
}

// untitledVideo carries a date and nothing else, so the file name offers no opinion
// about which row is right. The prompt then behaves as it did before ranking existed:
// no default, and blank means skip.
const untitledVideo = "2026-05-17.mp4"

func (t *AudioTestSuite) TestChoice_PicksFirst() {
	ambiguous := t.newAmbiguity()
	t.withStdin("1\n", func() {
		chosen := promptUserForMessageChoice(ambiguous, untitledVideo)
		t.Require().NotNil(chosen)
		t.Equal("Power of Prayer", chosen.Name)
	})
}

func (t *AudioTestSuite) TestChoice_PicksSecond() {
	ambiguous := t.newAmbiguity()
	t.withStdin("2\n", func() {
		chosen := promptUserForMessageChoice(ambiguous, untitledVideo)
		t.Require().NotNil(chosen)
		t.Equal("Power of Prayer: Q&A", chosen.Name)
	})
}

func (t *AudioTestSuite) TestChoice_BlankSkipsWithoutADefault() {
	ambiguous := t.newAmbiguity()
	t.withStdin("\n", func() {
		t.Nil(promptUserForMessageChoice(ambiguous, untitledVideo))
	})
}

func (t *AudioTestSuite) TestChoice_RejectsOutOfRangeThenAccepts() {
	ambiguous := t.newAmbiguity()
	t.withStdin("9\n0\nnonsense\n2\n", func() {
		chosen := promptUserForMessageChoice(ambiguous, untitledVideo)
		t.Require().NotNil(chosen)
		t.Equal("Power of Prayer: Q&A", chosen.Name)
	})
}

// a closed stdin must end the prompt rather than spin forever
func (t *AudioTestSuite) TestChoice_ClosedStdinSkips() {
	ambiguous := t.newAmbiguity()
	t.withStdin("", func() {
		t.Nil(promptUserForMessageChoice(ambiguous, untitledVideo))
	})
}

// +---------------------------------------------------------------------------
// | Thumbnails
// +---------------------------------------------------------------------------

// withThumbDir creates a fallback thumbnail directory holding the named files and
// points 'thumb-dir' at it
func (t *AudioTestSuite) withThumbDir(names ...string) string {
	dir := t.T().TempDir()
	viper.Set("thumb-dir", dir)
	t.T().Cleanup(func() { viper.Set("thumb-dir", "") })

	for _, name := range names {
		t.Require().NoError(os.WriteFile(filepath.Join(dir, name), []byte("image"), 0666))
	}
	return dir
}

// withVideoDir creates a message directory holding the named files and returns the
// path of the video inside it
func (t *AudioTestSuite) withVideoDir(video string, others ...string) string {
	dir := t.T().TempDir()
	for _, name := range append([]string{video}, others...) {
		t.Require().NoError(os.WriteFile(filepath.Join(dir, name), []byte("x"), 0666))
	}
	return filepath.Join(dir, video)
}

func (t *AudioTestSuite) TestThumb_NamedForTheVideo() {
	video := t.withVideoDir("2026-10-04 Message.mp4", "2026-10-04 Message.jpg")

	t.Equal(filepath.Join(filepath.Dir(video), "2026-10-04 Message.jpg"),
		getThumbnailPath(video, catalog.Message))
}

func (t *AudioTestSuite) TestThumb_NamedForTheVideoAcceptsPngAndJpeg() {
	video := t.withVideoDir("2026-10-04 Message.mp4", "2026-10-04 Message.png")
	t.Equal(filepath.Join(filepath.Dir(video), "2026-10-04 Message.png"),
		getThumbnailPath(video, catalog.Message))
}

// one image dropped into a series folder covers every message in that series
func (t *AudioTestSuite) TestThumb_SeriesThumbnailInTheFolder() {
	video := t.withVideoDir("2026-10-04 Message.mp4", "Offense series thumb.png")

	t.Equal(filepath.Join(filepath.Dir(video), "Offense series thumb.png"),
		getThumbnailPath(video, catalog.Message))
}

// every message in the folder resolves to the same series thumbnail
func (t *AudioTestSuite) TestThumb_SeriesThumbnailCoversEveryMessage() {
	dir := t.T().TempDir()
	for _, name := range []string{
		"2026-10-04 Part One.mp4", "2026-10-11 Part Two.mp4", "Offense thumb.jpg",
	} {
		t.Require().NoError(os.WriteFile(filepath.Join(dir, name), []byte("x"), 0666))
	}

	expected := filepath.Join(dir, "Offense thumb.jpg")
	t.Equal(expected,
		getThumbnailPath(filepath.Join(dir, "2026-10-04 Part One.mp4"), catalog.Message))
	t.Equal(expected,
		getThumbnailPath(filepath.Join(dir, "2026-10-11 Part Two.mp4"), catalog.Message))
}

// a message with its own artwork uses it, while the rest of the series folder still
// falls back to the series thumbnail
func (t *AudioTestSuite) TestThumb_OwnArtworkOverridesSeriesThumbnail() {
	dir := t.T().TempDir()
	for _, name := range []string{
		"2026-10-04 Part One.mp4", "2026-10-04 Part One.jpg",
		"2026-10-11 Part Two.mp4", "Offense thumb.jpg",
	} {
		t.Require().NoError(os.WriteFile(filepath.Join(dir, name), []byte("x"), 0666))
	}

	t.Equal(filepath.Join(dir, "2026-10-04 Part One.jpg"),
		getThumbnailPath(filepath.Join(dir, "2026-10-04 Part One.mp4"), catalog.Message))
	t.Equal(filepath.Join(dir, "Offense thumb.jpg"),
		getThumbnailPath(filepath.Join(dir, "2026-10-11 Part Two.mp4"), catalog.Message))
}

func (t *AudioTestSuite) TestThumb_ThumbMatchIsCaseInsensitive() {
	video := t.withVideoDir("2026-10-04 Message.mp4", "Sunday THUMB.JPG")

	t.Equal(filepath.Join(filepath.Dir(video), "Sunday THUMB.JPG"),
		getThumbnailPath(video, catalog.Message))
}

// a name match beats a loose "thumb" match, however the directory is ordered
func (t *AudioTestSuite) TestThumb_NameMatchWinsOverThumbMatch() {
	video := t.withVideoDir(
		"2026-10-04 Message.mp4", "2026-10-04 Message.jpg", "aaa-thumb.jpg")

	t.Equal(filepath.Join(filepath.Dir(video), "2026-10-04 Message.jpg"),
		getThumbnailPath(video, catalog.Message))
}

// several loose matches must resolve to the same file every run
func (t *AudioTestSuite) TestThumb_MultipleThumbsPickDeterministically() {
	video := t.withVideoDir("2026-10-04 Message.mp4", "b-thumb.jpg", "a-thumb.jpg")

	t.Equal(filepath.Join(filepath.Dir(video), "a-thumb.jpg"),
		getThumbnailPath(video, catalog.Message))
}

// a non-image with "thumb" in the name is not artwork
func (t *AudioTestSuite) TestThumb_IgnoresNonImages() {
	t.withThumbDir()
	video := t.withVideoDir("2026-10-04 Message.mp4", "thumb-notes.txt")

	t.Equal("", getThumbnailPath(video, catalog.Message))
}

func (t *AudioTestSuite) TestThumb_FallsBackToTypeArtwork() {
	dir := t.withThumbDir("WOL Thumbnail - Message.jpg", "WOL Thumbnail - Prayer.jpg")
	video := t.withVideoDir("2026-10-04 Message.mp4")

	t.Equal(filepath.Join(dir, "WOL Thumbnail - Prayer.jpg"),
		getThumbnailPath(video, catalog.Prayer))
	t.Equal(filepath.Join(dir, "WOL Thumbnail - Message.jpg"),
		getThumbnailPath(video, catalog.Message))
}

// a type with no artwork of its own is still a message as far as the cover image is
// concerned
func (t *AudioTestSuite) TestThumb_UnknownTypeFallsBackToMessageArtwork() {
	dir := t.withThumbDir("WOL Thumbnail - Message.jpg")
	video := t.withVideoDir("2026-10-04 Training.mp4")

	t.Equal(filepath.Join(dir, "WOL Thumbnail - Message.jpg"),
		getThumbnailPath(video, catalog.Training))
}

// local artwork beats the generic fallback
func (t *AudioTestSuite) TestThumb_LocalBeatsFallback() {
	t.withThumbDir("WOL Thumbnail - Message.jpg")
	video := t.withVideoDir("2026-10-04 Message.mp4", "2026-10-04 Message.jpg")

	t.Equal(filepath.Join(filepath.Dir(video), "2026-10-04 Message.jpg"),
		getThumbnailPath(video, catalog.Message))
}

// an empty fallback directory is not an error, it just means no thumbnail
func (t *AudioTestSuite) TestThumb_NothingAnywhere() {
	t.withThumbDir()
	video := t.withVideoDir("2026-10-04 Message.mp4")

	t.Equal("", getThumbnailPath(video, catalog.Message))
}

// thumb-dir is consulted by the fallback step alone, so leaving it unset simply means
// there is no fallback
func (t *AudioTestSuite) TestThumb_UnconfiguredThumbDir() {
	viper.Set("thumb-dir", "")
	video := t.withVideoDir("2026-10-04 Message.mp4")

	t.Equal("", getThumbnailPath(video, catalog.Message))
}

func (t *AudioTestSuite) TestThumbnailFileName() {
	t.Equal("WOL Thumbnail - Message.jpg", thumbnailFileName(catalog.Message))
	t.Equal("WOL Thumbnail - Prayer.jpg", thumbnailFileName(catalog.Prayer))

	// a hyphenated type capitalises each word, matching how the files are named
	t.Equal("WOL Thumbnail - Special-Event.jpg", thumbnailFileName(catalog.SpecialEvent))
}

// +---------------------------------------------------------------------------
// | Ranking candidates by the file name
// +---------------------------------------------------------------------------

// Enter takes the best match when the file name clearly points at one row
func (t *AudioTestSuite) TestChoice_EnterTakesBestMatch() {
	ambiguous := t.newAmbiguity()
	t.withStdin("\n", func() {
		chosen := promptUserForMessageChoice(ambiguous, "2026-05-17 Power of Prayer QA.mp4")
		t.Require().NotNil(chosen)
		t.Equal("Power of Prayer: Q&A", chosen.Name)
	})
}

// the numbers follow the ranked order, not the spreadsheet order
func (t *AudioTestSuite) TestChoice_NumbersFollowRankedOrder() {
	ambiguous := t.newAmbiguity()
	t.withStdin("1\n", func() {
		chosen := promptUserForMessageChoice(ambiguous, "2026-05-17 Power of Prayer QA.mp4")
		t.Require().NotNil(chosen)
		t.Equal("Power of Prayer: Q&A", chosen.Name)
	})
}

// a default must not swallow the ability to decline
func (t *AudioTestSuite) TestChoice_SkipKeyWithADefault() {
	ambiguous := t.newAmbiguity()
	t.withStdin("s\n", func() {
		t.Nil(promptUserForMessageChoice(ambiguous, "2026-05-17 Power of Prayer QA.mp4"))
	})
}

// the operator can still overrule the suggestion
func (t *AudioTestSuite) TestChoice_OverrulesBestMatch() {
	ambiguous := t.newAmbiguity()
	t.withStdin("2\n", func() {
		chosen := promptUserForMessageChoice(ambiguous, "2026-05-17 Power of Prayer QA.mp4")
		t.Require().NotNil(chosen)
		t.Equal("Power of Prayer", chosen.Name)
	})
}

func (t *AudioTestSuite) TestRank_OrdersByDistance() {
	candidates := []catalog.CatalogMessage{
		{Name: "The Works of God"},
		{Name: "The Importance of Voting"},
	}

	// the leading article differs, which edit distance tolerates and equality would not
	ranked, hasDefault := rankCandidatesByFileName(
		"2026-10-04 Importance of Voting.mp4", candidates)

	t.True(hasDefault)
	t.Equal("The Importance of Voting", ranked[0].Name)
	t.Equal("The Works of God", ranked[1].Name)
}

// a file name with no title offers no opinion, so the order is left alone and no
// default is suggested
func (t *AudioTestSuite) TestRank_NoTitleMeansNoOpinion() {
	candidates := []catalog.CatalogMessage{
		{Name: "The Works of God"},
		{Name: "The Importance of Voting"},
	}

	ranked, hasDefault := rankCandidatesByFileName(untitledVideo, candidates)

	t.False(hasDefault)
	t.Equal("The Works of God", ranked[0].Name)
	t.Equal("The Importance of Voting", ranked[1].Name)
}

// an equally good match for both is no match at all: offering a default here would
// invite a confirming keystroke it has not earned
func (t *AudioTestSuite) TestRank_TieOffersNoDefault() {
	candidates := []catalog.CatalogMessage{
		{Name: "Morning"},
		{Name: "Evening"},
	}

	ranked, hasDefault := rankCandidatesByFileName("2026-10-04 Zzzzzzz.mp4", candidates)

	t.False(hasDefault)
	t.Len(ranked, 2)
}

// ranking must not lose or duplicate a candidate
func (t *AudioTestSuite) TestRank_PreservesEveryCandidate() {
	candidates := []catalog.CatalogMessage{
		{Name: "Alpha"}, {Name: "Beta"}, {Name: "Gamma"},
	}

	ranked, _ := rankCandidatesByFileName("2026-10-04 Beta.mp4", candidates)

	t.Len(ranked, 3)
	names := []string{ranked[0].Name, ranked[1].Name, ranked[2].Name}
	t.ElementsMatch([]string{"Alpha", "Beta", "Gamma"}, names)
	t.Equal("Beta", ranked[0].Name)
}

// two rows sharing a name must both survive - scores are held per candidate, not
// keyed by name
func (t *AudioTestSuite) TestRank_HandlesDuplicateNames() {
	candidates := []catalog.CatalogMessage{
		{Name: "Same Title", Speakers: []string{"Pastor Vern Peltz"}},
		{Name: "Same Title", Speakers: []string{"Pastor Mary Peltz"}},
	}

	ranked, hasDefault := rankCandidatesByFileName("2026-10-04 Same Title.mp4", candidates)

	t.Len(ranked, 2)
	t.False(hasDefault) // identical names cannot be told apart by name
}

// +---------------------------------------------------------------------------
// | Scratch directory sweep
// +---------------------------------------------------------------------------

// withScratchDir points the scratch directory at a temporary one and seeds it with
// files of a given age
func (t *AudioTestSuite) withScratchDir(files map[string]time.Duration) string {
	dir := t.T().TempDir()
	viper.Set("scratch-dir", dir)
	t.T().Cleanup(func() { viper.Set("scratch-dir", "") })

	for name, age := range files {
		path := filepath.Join(dir, name)
		t.Require().NoError(os.WriteFile(path, []byte("content"), 0666))
		when := time.Now().Add(-age)
		t.Require().NoError(os.Chtimes(path, when, when))
	}
	return dir
}

func (t *AudioTestSuite) remaining(dir string) []string {
	entries, err := os.ReadDir(dir)
	t.Require().NoError(err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func (t *AudioTestSuite) TestSweep_RemovesOldKeepsRecent() {
	dir := t.withScratchDir(map[string]time.Duration{
		"old service.mp3":    48 * time.Hour,
		"old service.text":   48 * time.Hour,
		"todays service.mp3": 5 * time.Minute, // what this run just produced
		"yesterday.mp3":      23 * time.Hour,  // inside the window, still worth reusing
	})

	cleanUpScratchDir()

	t.Equal([]string{"todays service.mp3", "yesterday.mp3"}, t.remaining(dir))
}

func (t *AudioTestSuite) TestSweep_IgnoresOtherFiles() {
	// only the extensions this application creates are ever deleted, however old
	dir := t.withScratchDir(map[string]time.Duration{
		"old.mp3":       48 * time.Hour,
		"notes.docx":    48 * time.Hour,
		"thumbnail.jpg": 72 * time.Hour,
		"README":        99 * time.Hour,
	})

	cleanUpScratchDir()

	t.Equal([]string{"README", "notes.docx", "thumbnail.jpg"}, t.remaining(dir))
}

func (t *AudioTestSuite) TestSweep_KeepIntermediatesDisablesIt() {
	dir := t.withScratchDir(map[string]time.Duration{"old.mp3": 99 * time.Hour})
	viper.Set("keep-intermediates", true)
	defer viper.Set("keep-intermediates", false)

	cleanUpScratchDir()

	t.Equal([]string{"old.mp3"}, t.remaining(dir))
}

func (t *AudioTestSuite) TestSweep_MissingDirectoryIsNotAnError() {
	viper.Set("scratch-dir", filepath.Join(t.T().TempDir(), "never-created"))
	defer viper.Set("scratch-dir", "")

	t.NotPanics(cleanUpScratchDir)
}

// +---------------------------------------------------------------------------
// | Printing the results
// +---------------------------------------------------------------------------

// printInfo renders one message's box and returns it
func printInfo(info *MessageInfo) string {
	var buf bytes.Buffer
	printMessageInfo(&buf, 0, info)
	return buf.String()
}

// ranFor is a stopwatch that ran for the given time
func ranFor(d time.Duration) util.StopWatch {
	start := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	return util.StopWatch{StartTime: start, StopTime: start.Add(d)}
}

// assertClosedBox checks the output is one box whose every bordered line is full width,
// followed by the blank line that separates it from the next
func (t *AudioTestSuite) assertClosedBox(out string) {
	t.True(strings.HasSuffix(out, "╯\n\n"), "box is closed and followed by a blank line")
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "│") || strings.HasPrefix(line, "╭") ||
			strings.HasPrefix(line, "├") || strings.HasPrefix(line, "╰") {
			t.Equal(box.DefaultWidth, utf8.RuneCountInString(line), "line %q", line)
		}
	}
}

func (t *AudioTestSuite) TestPrintMessageInfo_Matched() {
	info := &MessageInfo{
		VideoPath:   "/videos/2026-10-04-pm Pay Attention.mp4",
		SpeakerName: "Pastor Mary Peltz",
		Title:       "Pay Attention to God's Breadcrumbs",
		Summary:     "A summary.",
		Packet: &ytpkg.Message{
			Channel:     ytpkg.ChannelWOL,
			Title:       "Pay Attention | Mary Peltz | October 4, 2026",
			Description: "A summary.",
			Privacy:     ytpkg.PrivacyPublic,
		},
	}

	out := printInfo(info)

	t.assertClosedBox(out)
	t.True(strings.HasPrefix(out, "╭─ Message #1 "))
	t.Contains(out, "│ Video file: 2026-10-04-pm Pay Attention.mp4 ")
	t.Contains(out, "│ Speaker   : Pastor Mary Peltz ")
	t.Contains(out, "├─ UPLOAD PACKET ")
	t.Contains(out, "\nPay Attention | Mary Peltz | October 4, 2026\n")
	t.NotContains(out, "No upload packet")
	t.NotContains(out, "Timeline", "the timeline is for --verbose only")
}

// A message that was processed but has no spreadsheet row still shows what was
// generated, so the work is not wasted
func (t *AudioTestSuite) TestPrintMessageInfo_Unmatched() {
	info := &MessageInfo{
		VideoPath:   "/videos/2026-10-11 Renamed Copy.mp4",
		SpeakerName: "Pastor Vern Peltz",
		Title:       "Learning at Moriah",
		Summary:     "A summary.\n\nSecond paragraph.",
		PacketError: errors.New("no spreadsheet row is dated 2026-10-11"),
	}

	out := printInfo(info)

	t.assertClosedBox(out)
	t.Contains(out, "├─ No upload packet ")
	t.Contains(out, "│ Reason: no spreadsheet row is dated 2026-10-11 ")
	t.Contains(out, "\nLearning at Moriah\n")
	t.Contains(out, "\nA summary.\n\nSecond paragraph.\n")
	t.Less(strings.Index(out, "├─ Titles "), strings.Index(out, "├─ Description "))
	t.NotContains(out, "Not processed")
}

func (t *AudioTestSuite) TestPrintMessageInfo_NotProcessed() {
	info := &MessageInfo{
		VideoPath:   "/videos/2026-10-11 Learning at Moriah.mp4",
		SpeakerName: "Pastor Vern Peltz",
	}

	out := printInfo(info)

	t.assertClosedBox(out)
	t.Contains(out, "├─ Not processed ")
	t.Contains(out, "│ Processing failed - see the error below. ")
	t.NotContains(out, "Reason")
}

// No row is known before processing starts, so PacketError is already set when
// processing fails. That message failed; it was not merely unmatched, and reporting it
// as unmatched would show empty Titles and Description sections.
func (t *AudioTestSuite) TestPrintMessageInfo_NotProcessedAndNoRow() {
	info := &MessageInfo{
		VideoPath:   "/videos/2026-10-11 Learning at Moriah.mp4",
		SpeakerName: "Pastor Vern Peltz",
		PacketError: errors.New("the spreadsheet was not available"),
	}

	out := printInfo(info)

	t.assertClosedBox(out)
	t.Contains(out, "├─ Not processed ")
	t.Contains(out, "│ Reason: the spreadsheet was not available ")
	t.NotContains(out, "No upload packet")
	t.NotContains(out, "Titles")
}

func (t *AudioTestSuite) TestPrintMessageInfo_TimelineWhenVerbose() {
	viper.Set("verbose", true)
	defer viper.Set("verbose", false)

	info := &MessageInfo{
		VideoPath:      "/videos/2026-10-04-pm Pay Attention.mp4",
		Title:          "Pay Attention",
		Summary:        "A summary.",
		PacketError:    errors.New("no row"),
		ExtractTime:    ranFor(24 * time.Second),
		TranscribeTime: ranFor(2*time.Minute + 8*time.Second),
		SummaryTime:    ranFor(4 * time.Second),
	}

	out := printInfo(info)

	t.assertClosedBox(out)
	t.Contains(out, "├─ Timeline ")
	t.Contains(out, "│ Extract   :    24s ")
	t.Contains(out, "│ Transcribe:  2m08s ")
	t.Contains(out, "│ Summarize :     4s ")
	t.Contains(out, "│ Total     :  2m36s ")
}

// A step that did not run was either reused, when the message went on to be
// summarized, or never reached; the total counts only what ran
func (t *AudioTestSuite) TestPrintMessageInfo_TimelineReusedAndNotReached() {
	viper.Set("verbose", true)
	defer viper.Set("verbose", false)

	reused := printInfo(&MessageInfo{
		VideoPath:   "/videos/a.mp4",
		Title:       "Pay Attention",
		PacketError: errors.New("no row"),
		SummaryTime: ranFor(4 * time.Second),
	})
	t.Contains(reused, "│ Extract   : reused ")
	t.Contains(reused, "│ Transcribe: reused ")
	t.Contains(reused, "│ Total     :     4s ")

	stopped := printInfo(&MessageInfo{
		VideoPath:   "/videos/a.mp4",
		ExtractTime: ranFor(24 * time.Second),
	})
	t.Contains(stopped, "│ Extract   :    24s ")
	t.Contains(stopped, "│ Transcribe:      - ")
	t.Contains(stopped, "│ Summarize :      - ")
	t.Contains(stopped, "│ Total     :    24s ")
}

func (t *AudioTestSuite) TestFormatStepDuration() {
	t.Equal("24s", formatStepDuration(ranFor(24*time.Second), true))
	t.Equal("2m08s", formatStepDuration(ranFor(2*time.Minute+8*time.Second), true))
	t.Equal("1h02m03s", formatStepDuration(ranFor(time.Hour+2*time.Minute+3*time.Second), true))
	t.Equal("2s", formatStepDuration(ranFor(1600*time.Millisecond), true), "rounded to the second")
	t.Equal("reused", formatStepDuration(util.StopWatch{}, true))
	t.Equal("-", formatStepDuration(util.StopWatch{}, false))
}

// The banner is a closed box holding the same instructions it always has, with a blank
// line either side to set it off from the prompts
func (t *AudioTestSuite) TestPrintAudioBanner() {
	var buf bytes.Buffer
	printAudioBanner(&buf)
	out := buf.String()

	t.True(strings.HasPrefix(out, "\n╭─ Prepare messages for YouTube "))
	t.True(strings.HasSuffix(out, "╯\n\n"))
	for _, line := range strings.Split(strings.Trim(out, "\n"), "\n") {
		t.Equal(box.DefaultWidth, utf8.RuneCountInString(line), "line %q", line)
	}

	// the wording is unchanged, whatever width it wraps to
	var text []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "│") {
			text = append(text, strings.Trim(line, "│ "))
		}
	}
	words := strings.Join(strings.Fields(strings.Join(text, " ")), " ")
	t.Equal("Drag an edited video into this window and press Enter. You will be asked "+
		"about that video before being asked for the next one. "+
		"Press Enter on an empty line when there are no more videos.", words)
}

// +---------------------------------------------------------------------------
// | Dragged file paths
// +---------------------------------------------------------------------------

func (t *AudioTestSuite) TestCleanDroppedPath() {
	cases := map[string]string{
		`C:\Videos\Pay Attention.mp4`:   `C:\Videos\Pay Attention.mp4`,
		`"C:\Videos\Pay Attention.mp4"`: `C:\Videos\Pay Attention.mp4`,
		`'C:\Videos\Pay Attention.mp4'`: `C:\Videos\Pay Attention.mp4`,

		// PowerShell, as VSCode's terminal inserts a dragged file
		`& 'C:\Videos\Pay Attention.mp4'`:       `C:\Videos\Pay Attention.mp4`,
		"& 'C:\\Videos\\Pay Attention.mp4'\r\n": `C:\Videos\Pay Attention.mp4`,
		`&'C:\Videos\Pay Attention.mp4'`:        `C:\Videos\Pay Attention.mp4`,

		// PowerShell doubles an apostrophe inside single quotes
		`& 'C:\Videos\God''s Word.mp4'`: `C:\Videos\God's Word.mp4`,

		// an ampersand inside the path is left alone
		`C:\Videos\Q&A.mp4`:     `C:\Videos\Q&A.mp4`,
		`& 'C:\Videos\Q&A.mp4'`: `C:\Videos\Q&A.mp4`,
		"  \r\n":                "",
		"":                      "",
	}

	for in, expected := range cases {
		t.Equal(expected, cleanDroppedPath(in), "input %q", in)
	}
}

// the prompt accepts a dragged PowerShell path
func (t *AudioTestSuite) TestPromptAcceptsPowerShellDrop() {
	dir := t.T().TempDir()
	video := filepath.Join(dir, "God's Word.mp4")
	t.Require().NoError(os.WriteFile(video, []byte("v"), 0o644))

	dropped := "& '" + strings.ReplaceAll(video, "'", "''") + "'\n"
	t.withStdin(dropped, func() {
		t.Equal(video, getInputVideo(""))
	})
}

// +---------------------------------------------------------------------------
// | Match confidence
// +---------------------------------------------------------------------------

func (t *AudioTestSuite) TestConfidence_SameTitleIsCertain() {
	t.Equal(1.0, matchConfidence("Pay Attention", "Pay Attention"))

	// case, punctuation and articles are house style, not a difference
	t.Equal(1.0, matchConfidence("Importance of Voting", "The Importance of Voting"))
	t.Equal(1.0, matchConfidence("power of prayer q&a", "Power of Prayer: Q&A"))
}

func (t *AudioTestSuite) TestConfidence_Scale() {
	// one character in nineteen: "power of prayer qa" against "power of prayer q a"
	t.InDelta(18.0/19.0, matchConfidence("Power of Prayer QA", "Power of Prayer: Q&A"), 0.001)

	// unrelated titles fall to the bottom of the scale
	t.Less(matchConfidence("Pay Attention", "The Importance of Voting"), confidenceMedium)

	// nothing in common at all
	t.Equal(0.0, matchConfidence("zzzz", "aaaa"))
	t.Equal(0.0, matchConfidence("", ""))
	t.Equal(0.0, matchConfidence("The", "A"), "titles of nothing but articles have nothing to compare")
}

// the colour is chosen by the first threshold reached; to a test's buffer, which is
// not a terminal, the number comes out plain
func (t *AudioTestSuite) TestConfidence_Format() {
	t.Equal("1.00", formatConfidence(1))
	t.Equal("0.80", formatConfidence(0.8))
	t.Equal("0.41", formatConfidence(0.41))
	t.Equal("0.00", formatConfidence(0))
}

// every row is shown with its confidence, best first, and the old marker is gone
func (t *AudioTestSuite) TestChoice_ShowsConfidence() {
	ambiguous := t.newAmbiguity()
	out := captureStdout(t.T(), func() {
		t.withStdin("\n", func() {
			promptUserForMessageChoice(ambiguous, "2026-05-17 Power of Prayer QA.mp4")
		})
	})

	t.Contains(out, "  1. 0.95  Power of Prayer: Q&A")
	t.Contains(out, "  2. 0.83  Power of Prayer")
	t.NotContains(out, "best match")
}

// with no title in the file name there is nothing to be confident about
func (t *AudioTestSuite) TestChoice_NoConfidenceWithoutATitle() {
	ambiguous := t.newAmbiguity()
	out := captureStdout(t.T(), func() {
		t.withStdin("\n", func() {
			promptUserForMessageChoice(ambiguous, untitledVideo)
		})
	})

	t.Contains(out, "  1. Power of Prayer")
	t.NotRegexp(`\d\.\d\d  `, out)
}

// captureStdout runs fn with os.Stdout replaced by a pipe, and returns what it wrote.
// A pipe is not a terminal, so anything coloured comes out plain.
func captureStdout(t *testing.T, fn func()) string {
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	os.Stdout = writer
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		buf.ReadFrom(reader)
		done <- buf.String()
	}()

	defer func() { os.Stdout = original }()
	fn()
	writer.Close()
	return <-done
}
