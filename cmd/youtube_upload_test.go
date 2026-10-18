package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// +---------------------------------------------------------------------------
// | Step list (Q23)
// +---------------------------------------------------------------------------

var publishLabels = []string{
	"Upload video (1.2 GB)",
	"Set thumbnail",
	"Create playlist",
	"Add to playlist",
	"Reorder playlist",
	"Set privacy: public",
}

// lines splits output into lines, each as it finally reads on a terminal: a carriage
// return starts the line over
func lines(out string) []string {
	var result []string
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if index := strings.LastIndex(line, "\r"); index >= 0 {
			line = line[index+1:]
		}
		result = append(result, strings.TrimRight(line, " "))
	}
	return result
}

func TestStepPrinter_AllDone(t *testing.T) {
	var buf bytes.Buffer
	p := newStepPrinter(&buf, publishLabels...)

	p.start()
	for _, percent := range []int{0, 37, 100} {
		p.progress(percent)
	}
	p.finish("done  4m12s")
	p.start()
	p.finish("done")
	p.start()
	p.finish("skipped (exists)")
	p.start()
	p.finish("done (1)")
	p.start()
	p.finish("3 moves, done")
	p.start()
	p.finish("done")

	assert.Equal(t, []string{
		"  1/6  Upload video (1.2 GB) ............ done  4m12s",
		"  2/6  Set thumbnail .................... done",
		"  3/6  Create playlist .................. skipped (exists)",
		"  4/6  Add to playlist .................. done (1)",
		"  5/6  Reorder playlist ................. 3 moves, done",
		"  6/6  Set privacy: public .............. done",
	}, lines(buf.String()))
}

// the percentage is written in place, on the upload's own line
func TestStepPrinter_ProgressInPlace(t *testing.T) {
	var buf bytes.Buffer
	p := newStepPrinter(&buf, publishLabels...)

	p.start()
	p.progress(5)
	p.progress(100)
	p.finish("done")

	out := buf.String()
	assert.Contains(t, out, "\r  1/6  Upload video (1.2 GB) ............   5%")
	assert.Contains(t, out, "\r  1/6  Upload video (1.2 GB) ............ 100%")
	assert.Equal(t, 1, strings.Count(out, "\n"), "one line, rewritten")
	assert.Equal(t, []string{"  1/6  Upload video (1.2 GB) ............ done"}, lines(out))
}

func TestStepPrinter_FailureStopsTheRest(t *testing.T) {
	var buf bytes.Buffer
	p := newStepPrinter(&buf, publishLabels...)

	p.start()
	p.finish("done  12s")
	p.start()
	p.fail(errors.New("the thumbnail is too large"))

	assert.Equal(t, []string{
		"  1/6  Upload video (1.2 GB) ............ done  12s",
		"  2/6  Set thumbnail .................... FAILED: the thumbnail is too large",
		"  3/6  Create playlist .................. not run",
		"  4/6  Add to playlist .................. not run",
		"  5/6  Reorder playlist ................. not run",
		"  6/6  Set privacy: public .............. not run",
	}, lines(buf.String()))
}

// a retry message mid-step gets a line of its own, and the step carries on after it
func TestStepPrinter_InterruptedByAMessage(t *testing.T) {
	var buf bytes.Buffer
	p := newStepPrinter(&buf, "Add to playlist")

	p.start()
	fmt.Fprintf(p, "  YouTube is not ready to add to the playlist yet, retrying in 1s...\n")
	p.finish("done (1)")

	assert.Equal(t, []string{
		"  1/1  Add to playlist ..................",
		"  YouTube is not ready to add to the playlist yet, retrying in 1s...",
		"  1/1  Add to playlist .................. done (1)",
	}, lines(buf.String()))
}

// no emoji: the console the operator uses draws them badly
func TestStepPrinter_PlainText(t *testing.T) {
	var buf bytes.Buffer
	p := newStepPrinter(&buf, publishLabels...)
	for range publishLabels {
		p.start()
		p.finish("done")
	}
	for _, r := range buf.String() {
		assert.Less(t, r, rune(0x2000), "unexpected character %q", r)
	}
}

// +---------------------------------------------------------------------------
// | The duplicate guard (Q17, Q24)
// +---------------------------------------------------------------------------

// guard records which checks were made, in order, and answers each from a script
type guard struct {
	calls    []string
	recorded string
	recent   string
	err      error
}

func (g *guard) checkRecord() (string, error) {
	g.calls = append(g.calls, "record")
	return g.recorded, g.err
}

func (g *guard) checkRecent() (string, error) {
	g.calls = append(g.calls, "recent")
	return g.recent, nil
}

func (g *guard) decide(msg *catalog.CatalogMessage) (uploadDecision, error) {
	return decideUpload(msg, g.checkRecord, g.checkRecent)
}

func TestDecideUpload_SheetLinkFirst(t *testing.T) {
	g := &guard{recorded: "recorded000", recent: "recent00000"}
	msg := &catalog.CatalogMessage{Video: &catalog.OnlineResource{URL: "https://youtu.be/-4AH6lVaW38"}}

	decision, err := g.decide(msg)
	require.NoError(t, err)
	assert.Equal(t, actionSync, decision.action)
	assert.Equal(t, "-4AH6lVaW38", decision.videoID)
	assert.Contains(t, decision.reason, "spreadsheet")
	assert.Empty(t, g.calls, "nothing else is consulted")
}

func TestDecideUpload_RecordSecond(t *testing.T) {
	g := &guard{recorded: "recorded000", recent: "recent00000"}

	decision, err := g.decide(&catalog.CatalogMessage{})
	require.NoError(t, err)
	assert.Equal(t, uploadDecision{actionSync, "recorded000", "this file was uploaded before"}, decision)
	assert.Equal(t, []string{"record"}, g.calls, "the recent uploads are not read")
}

func TestDecideUpload_RecentTitleThird(t *testing.T) {
	// a Rumble link in the Video column is not a YouTube upload
	g := &guard{recent: "recent00000"}
	msg := &catalog.CatalogMessage{Video: &catalog.OnlineResource{URL: "https://rumble.com/v1abc.html"}}

	decision, err := g.decide(msg)
	require.NoError(t, err)
	assert.Equal(t, uploadDecision{actionSync, "recent00000", "a recent upload has the same title"}, decision)
	assert.Equal(t, []string{"record", "recent"}, g.calls)
}

func TestDecideUpload_OtherwiseUpload(t *testing.T) {
	g := &guard{}

	decision, err := g.decide(&catalog.CatalogMessage{})
	require.NoError(t, err)
	assert.Equal(t, actionUpload, decision.action)
	assert.Equal(t, []string{"record", "recent"}, g.calls)
}

// a guard that cannot be checked stops the upload, rather than uploading blind
func TestDecideUpload_ErrorStops(t *testing.T) {
	g := &guard{err: errors.New("cannot read the upload record")}

	_, err := g.decide(&catalog.CatalogMessage{})
	assert.ErrorContains(t, err, "upload record")
	assert.Equal(t, []string{"record"}, g.calls)
}

// +---------------------------------------------------------------------------
// | Output
// +---------------------------------------------------------------------------

func TestPrintPasteLines(t *testing.T) {
	msg := &catalog.CatalogMessage{
		Name:     "Pay Attention",
		Speakers: []string{"Pastor Mary Peltz"},
		Series:   []catalog.SeriesReference{{Name: "Prayer", Index: 3}},
	}

	var buf bytes.Buffer
	printPasteLines(&buf, msg, "newvid00001", "A summary.")
	out := buf.String()
	assert.Contains(t, out, "│ Row        : Pay Attention  [Prayer, track 3]  (Pastor Mary Peltz) ")
	assert.Contains(t, out, "\nhttps://youtu.be/newvid00001\n")
	assert.Contains(t, out, "│ Description column: ")
	assert.Contains(t, out, "\nA summary.\n")

	buf.Reset()
	printPasteLines(&buf, msg, "newvid00001", "")
	assert.NotContains(t, buf.String(), "Description column")
}

func TestFileSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v.mp4")
	require.NoError(t, os.WriteFile(path, make([]byte, 3<<20), 0o644))
	assert.Equal(t, "3 MB", fileSize(path))
	assert.Equal(t, "size unknown", fileSize(filepath.Join(t.TempDir(), "missing.mp4")))
}
