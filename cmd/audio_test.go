package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/WordOfLifeMN/online/catalog"
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
			Type: catalog.Message,
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

// Each case runs with stdin closed. Were the prompt still being reached, the read
// would fail and the result would fall back to the default rather than the expected
// name - so these tests prove the question is skipped, not merely answered.

func (t *AudioTestSuite) TestSpeaker_SpreadsheetIsAuthoritative() {
	msg := &catalog.CatalogMessage{Speakers: []string{"Pastor Mary Peltz"}}

	t.withStdin("", func() {
		t.Equal("Pastor Mary Peltz", resolveSpeaker("2026-10-04 Message.mp4", msg))
	})
}

// the row outranks an initial in the file name, so the summary cannot introduce one
// person while the title - built from the same row - credits another
func (t *AudioTestSuite) TestSpeaker_SpreadsheetBeatsFileNameInitial() {
	msg := &catalog.CatalogMessage{Speakers: []string{"Pastor Mary Peltz"}}

	t.withStdin("", func() {
		t.Equal("Pastor Mary Peltz", resolveSpeaker("2026-10-04-v Message.mp4", msg))
	})
}

func (t *AudioTestSuite) TestSpeaker_MultipleSpeakersFromSheet() {
	msg := &catalog.CatalogMessage{
		Speakers: []string{"Pastor Vern Peltz", "Pastor Mary Peltz"},
	}

	t.withStdin("", func() {
		t.Equal("Pastor Vern Peltz, Pastor Mary Peltz",
			resolveSpeaker("2026-10-04 Message.mp4", msg))
	})
}

// --speaker is the operator overriding everything, including the sheet
func (t *AudioTestSuite) TestSpeaker_FlagOverridesSpreadsheet() {
	viper.Set("speaker", "Guest Preacher")
	defer viper.Set("speaker", "")

	msg := &catalog.CatalogMessage{Speakers: []string{"Pastor Vern Peltz"}}

	t.withStdin("", func() {
		t.Equal("Guest Preacher", resolveSpeaker("2026-10-04-v Message.mp4", msg))
	})
}

// with no row - the lookup failed, or the sheet was unreadable - the file name initial
// is still worth having
func (t *AudioTestSuite) TestSpeaker_FallsBackToFileNameInitial() {
	t.withStdin("", func() {
		t.Equal("Pastor Vern Peltz", resolveSpeaker("2026-10-04-v Message.mp4", nil))
	})
}

// a row that names nobody is not an answer, so it falls through rather than
// resolving to an empty speaker
func (t *AudioTestSuite) TestSpeaker_EmptyRowFallsThrough() {
	msg := &catalog.CatalogMessage{}

	t.withStdin("", func() {
		t.Equal("Pastor Mary Peltz", resolveSpeaker("2026-10-04-m Message.mp4", msg))
	})
}

// nothing to go on at all: ask, and honour the answer
func (t *AudioTestSuite) TestSpeaker_PromptsWhenNothingKnown() {
	t.withStdin("m\n", func() {
		t.Equal("Pastor Mary Peltz", resolveSpeaker("2026-10-04 Message.mp4", nil))
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
