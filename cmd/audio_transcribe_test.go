package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WordOfLifeMN/online/util"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
)

func TestTranscriptTestSuite(t *testing.T) {
	suite.Run(t, new(TranscriptTestSuite))
}

type TranscriptTestSuite struct {
	suite.Suite
}

// -----------------------------------------------------------------------------------
// | Header line
// -----------------------------------------------------------------------------------

func (t *TranscriptTestSuite) TestHeader_Format() {
	when := time.Date(2026, 10, 18, 21, 30, 0, 0, time.Local)
	t.Equal("# wolm-transcript app=faster-whisper model=small transcribed=2026-10-18",
		transcriptHeader("small", when))
}

func (t *TranscriptTestSuite) TestHeader_AddThenStripRoundTrips() {
	path := filepath.Join(t.T().TempDir(), "message.text")
	original := " Good morning, church.\n Turn with me to Genesis 22.\n"
	t.Require().NoError(os.WriteFile(path, []byte(original), 0666))

	t.Require().NoError(addTranscriptHeader(path, transcriptHeader("small", time.Now())))

	content, err := os.ReadFile(path)
	t.Require().NoError(err)
	t.Contains(string(content), transcriptHeaderMarker+" app=faster-whisper model=small ")
	t.Equal(original, stripTranscriptHeader(string(content)))
	t.False(util.DoesPathExist(path+".partial"), "the rewrite leaves nothing behind")
}

func (t *TranscriptTestSuite) TestStrip_NoHeaderIsUnchanged() {
	text := "Good morning, church.\n\nTurn with me to Genesis 22.\n"
	t.Equal(text, stripTranscriptHeader(text))
}

// only the marker is a header. a first line that merely looks like a comment is
// something someone said, or typed by hand, and is sent as it is.
func (t *TranscriptTestSuite) TestStrip_OrdinaryCommentLineIsKept() {
	for _, text := range []string{
		"# not a header\nGood morning.\n",
		"# wolm-transcripts are kept beside the video\nGood morning.\n",
	} {
		t.Equal(text, stripTranscriptHeader(text))
	}
}

func (t *TranscriptTestSuite) TestStrip_CRLF() {
	text := "# wolm-transcript app=faster-whisper model=small transcribed=2026-10-18\r\n\r\nGood morning.\r\n"
	t.Equal("Good morning.\r\n", stripTranscriptHeader(text))
}

func (t *TranscriptTestSuite) TestStrip_HeaderOnly() {
	t.Equal("", stripTranscriptHeader("# wolm-transcript app=faster-whisper model=small\n\n"))
	t.Equal("", stripTranscriptHeader("# wolm-transcript"))
}

// -----------------------------------------------------------------------------------
// | Finding and preserving transcripts
// -----------------------------------------------------------------------------------

// withVideoAndScratch creates a video folder holding an .mp4 and points the scratch
// directory at a separate temporary one. Returns the video path.
func (t *TranscriptTestSuite) withVideoAndScratch() string {
	viper.Set("scratch-dir", t.T().TempDir())
	t.T().Cleanup(func() { viper.Set("scratch-dir", "") })

	video := filepath.Join(t.T().TempDir(), "2026-10-12 Sun Pay Attention.mp4")
	t.Require().NoError(os.WriteFile(video, []byte("video"), 0666))
	return video
}

func (t *TranscriptTestSuite) write(path, content string) {
	t.Require().NoError(os.WriteFile(path, []byte(content), 0666))
}

func (t *TranscriptTestSuite) read(path string) string {
	content, err := os.ReadFile(path)
	t.Require().NoError(err)
	return string(content)
}

func (t *TranscriptTestSuite) TestPaths() {
	video := t.withVideoAndScratch()

	t.Equal(filepath.Join(filepath.Dir(video), "2026-10-12 Sun Pay Attention.txt"),
		getPreservedTranscriptPath(video))
	t.Equal(filepath.Join(getScratchDir(), "2026-10-12 Sun Pay Attention.text"),
		getScratchTranscriptPathFromVideoPath(video))
}

func (t *TranscriptTestSuite) TestFind_PreservedTranscript() {
	video := t.withVideoAndScratch()
	t.write(getPreservedTranscriptPath(video), "kept")

	t.Equal(getPreservedTranscriptPath(video), findExistingTranscript(video))
}

// deleting the .txt is how a transcript is regenerated, so a recent scratch copy must
// not stand in for it
func (t *TranscriptTestSuite) TestFind_IgnoresScratchTranscript() {
	video := t.withVideoAndScratch()
	t.Require().NoError(os.MkdirAll(getScratchDir(), 0777))
	t.write(getScratchTranscriptPathFromVideoPath(video), "scratch")

	t.Equal("", findExistingTranscript(video))
}

func (t *TranscriptTestSuite) TestFind_IgnoresEmptyTranscript() {
	video := t.withVideoAndScratch()
	t.write(getPreservedTranscriptPath(video), "")

	t.Equal("", findExistingTranscript(video))
}

func (t *TranscriptTestSuite) TestFind_Nothing() {
	video := t.withVideoAndScratch()
	t.Equal("", findExistingTranscript(video))
}

func (t *TranscriptTestSuite) TestPreserve_CopiesBesideTheVideo() {
	video := t.withVideoAndScratch()
	scratch := filepath.Join(t.T().TempDir(), "message.text")
	t.write(scratch, "transcribed words")

	got := preserveTranscript(scratch, video)

	t.Equal(getPreservedTranscriptPath(video), got)
	t.Equal("transcribed words", t.read(got))
	t.True(util.IsFile(scratch), "the scratch copy is left for the sweep")
}

// a folder that cannot be written to costs the preserved copy, not the run
func (t *TranscriptTestSuite) TestPreserve_FailedCopyFallsBackToScratch() {
	scratch := filepath.Join(t.T().TempDir(), "message.text")
	t.write(scratch, "transcribed words")
	video := filepath.Join(t.T().TempDir(), "never-created", "message.mp4")

	t.Equal(scratch, preserveTranscript(scratch, video))
	t.False(util.DoesPathExist(getPreservedTranscriptPath(video)))
}

func (t *TranscriptTestSuite) TestPreserve_NeverOverwrites() {
	video := t.withVideoAndScratch()
	t.write(getPreservedTranscriptPath(video), "kept")
	scratch := filepath.Join(t.T().TempDir(), "message.text")
	t.write(scratch, "new")

	t.Equal(scratch, preserveTranscript(scratch, video))
	t.Equal("kept", t.read(getPreservedTranscriptPath(video)))
}

// an empty .txt is not a transcript, so it is replaced rather than protected
func (t *TranscriptTestSuite) TestPreserve_ReplacesEmpty() {
	video := t.withVideoAndScratch()
	t.write(getPreservedTranscriptPath(video), "")
	scratch := filepath.Join(t.T().TempDir(), "message.text")
	t.write(scratch, "new")

	t.Equal(getPreservedTranscriptPath(video), preserveTranscript(scratch, video))
	t.Equal("new", t.read(getPreservedTranscriptPath(video)))
}
