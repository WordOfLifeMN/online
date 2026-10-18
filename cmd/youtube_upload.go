package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/util/box"
	"github.com/WordOfLifeMN/online/ytclient"
	"github.com/WordOfLifeMN/online/ytpkg"
)

// +---------------------------------------------------------------------------
// | Step list
// +---------------------------------------------------------------------------

// stepLabelWidth lines up the results of the steps, dots filling the gap
const stepLabelWidth = 34

// stepPrinter shows a fixed list of steps as they run, one line each:
//
//	1/6  Upload video (1.2 GB) ......... done  4m12s
//	2/6  Set thumbnail ................. done
//	3/6  Create playlist ............... skipped (exists)
//
// A step's line is started when it begins and finished with its result, so a slow
// step shows what is being waited on. The upload's percentage updates in place. A
// failed step shows FAILED and its reason, and every step after it "not run".
//
// It is also an io.Writer, for messages that arrive while a step is running - a retry
// - which are put on a line of their own before the step's line is picked up again.
type stepPrinter struct {
	w      io.Writer
	labels []string
	next   int  // the step running, or about to
	open   bool // a step's line has been started and not finished
	shown  int  // how much in-place text follows the step's prefix
}

func newStepPrinter(w io.Writer, labels ...string) *stepPrinter {
	return &stepPrinter{w: w, labels: labels}
}

// prefix renders a step's number, label and dots
func (p *stepPrinter) prefix(step int) string {
	label := p.labels[step]
	dots := max(3, stepLabelWidth-utf8.RuneCountInString(label)-1)
	return fmt.Sprintf("  %d/%d  %s %s ", step+1, len(p.labels), label, strings.Repeat(".", dots))
}

// start begins the next step's line
func (p *stepPrinter) start() {
	fmt.Fprint(p.w, p.prefix(p.next))
	p.open, p.shown = true, 0
}

// progress shows a percentage on the running step's line, in place
func (p *stepPrinter) progress(percent int) {
	text := fmt.Sprintf("%3d%%", percent)
	fmt.Fprintf(p.w, "\r%s%s", p.prefix(p.next), text)
	p.shown = len(text)
}

// finish ends the running step's line with its result
func (p *stepPrinter) finish(result string) {
	pad := strings.Repeat(" ", max(0, p.shown-len(result)))
	fmt.Fprintf(p.w, "\r%s%s%s\n", p.prefix(p.next), result, pad)
	p.next++
	p.open, p.shown = false, 0
}

// fail ends the running step as failed, and marks every later step as not run
func (p *stepPrinter) fail(err error) {
	p.finish("FAILED: " + err.Error())
	for ; p.next < len(p.labels); p.next++ {
		fmt.Fprintf(p.w, "%snot run\n", p.prefix(p.next))
	}
}

// Write puts a message on its own line, picking the running step's line up again
// after it
func (p *stepPrinter) Write(b []byte) (int, error) {
	if !p.open {
		return p.w.Write(b)
	}
	fmt.Fprintln(p.w)
	n, err := p.w.Write(b)
	fmt.Fprint(p.w, p.prefix(p.next))
	p.shown = 0
	return n, err
}

// +---------------------------------------------------------------------------
// | Whether to upload
// +---------------------------------------------------------------------------

// uploadAction is what to do with a message that has a package
type uploadAction int

const (
	actionUpload uploadAction = iota
	actionSync                // it is already on YouTube: sync it instead
)

// uploadDecision is whether to upload, and if not, which video is already there and
// how it was found
type uploadDecision struct {
	action  uploadAction
	videoID string
	reason  string
}

// decideUpload works out whether a message still needs uploading. It must not be
// uploaded twice, so three things are checked, cheapest and surest first, each only
// if the one before found nothing:
//
//  1. the sheet's Video column already links a YouTube video
//  2. the local upload record has this file (recorded)
//  3. one of the channel's most recent uploads has the same title (recent)
//
// Anything found is synced instead of uploaded.
func decideUpload(
	msg *catalog.CatalogMessage,
	recorded func() (string, error),
	recent func() (string, error),
) (uploadDecision, error) {
	if msg.Video != nil {
		if id, ok := ytpkg.ParseVideoRef(msg.Video.URL); ok {
			return uploadDecision{actionSync, id, "the spreadsheet already links it"}, nil
		}
	}

	id, err := recorded()
	if err != nil {
		return uploadDecision{}, err
	}
	if id != "" {
		return uploadDecision{actionSync, id, "this file was uploaded before"}, nil
	}

	id, err = recent()
	if err != nil {
		return uploadDecision{}, err
	}
	if id != "" {
		return uploadDecision{actionSync, id, "a recent upload has the same title"}, nil
	}

	return uploadDecision{action: actionUpload}, nil
}

// recentUploadCount is how many of the channel's latest uploads are checked for a
// title that matches, as the last guard against a duplicate
const recentUploadCount = 50

// +---------------------------------------------------------------------------
// | Publishing
// +---------------------------------------------------------------------------

// publishAll offers to publish every message that has a package, after the audio
// command has shown them. They share one sync run, so that the playlist sweep at the
// end covers every playlist any of them touched.
//
// Nothing here is fatal to the audio command: its output has already been shown.
func publishAll(ctx context.Context, cat *catalog.Catalog, infos []*MessageInfo) error {
	channel := ytpkg.ChannelWOL

	var toPublish []*MessageInfo
	for _, info := range infos {
		if info.Packet == nil {
			continue
		}
		if ch := ytpkg.ChannelForMinistry(info.Message.Ministry); ch != channel {
			fmt.Printf("Not publishing %s: %s\n", filepath.Base(info.VideoPath),
				(&notAuthenticatedError{channel: ch}).Error())
			continue
		}
		toPublish = append(toPublish, info)
	}
	if len(toPublish) == 0 {
		return nil
	}

	client, err := newYouTubeClient(ctx, channel)
	if err != nil {
		fmt.Printf("Not publishing to YouTube: %s\n", err)
		return nil
	}

	recordPath := uploadRecordPath()
	record, err := loadUploadRecord(recordPath)
	if err != nil {
		return err
	}

	run := newSyncRun(cat, client, channel, os.Stdout)

	var errs []error
	for _, info := range toPublish {
		if err := publishMessage(run, info, record, recordPath); err != nil {
			fmt.Printf("ERROR     %s  %s\n\n", filepath.Base(info.VideoPath), err)
			errs = append(errs, fmt.Errorf("%s: %w", filepath.Base(info.VideoPath), err))
		}
	}

	errs = append(errs, run.sweepPlaylists())
	run.printSummary()
	return errors.Join(errs...)
}

// publishMessage uploads one message, or syncs it if it is already on YouTube.
//
// The upload is six steps, each shown as it runs:
//
//  1. upload, as private, with the house defaults; recorded locally at once
//  2. set the thumbnail
//  3. create any playlist the operator agreed to
//  4. add the video to its playlists
//  5. put those playlists in track order
//  6. set the target privacy
//
// A failure stops the rest, and the video stays private: nothing half-finished is ever
// public. Run again, the upload record turns the second attempt into a sync.
func publishMessage(run *syncRun, info *MessageInfo, record *uploadRecord, recordPath string) error {
	pkg, msg := info.Packet, info.Message
	fmt.Fprintf(run.out, "\n── Publish %s\n", filepath.Base(info.VideoPath))

	decision, err := decideUpload(msg,
		func() (string, error) { return recordedUpload(run, record, recordPath, info.VideoPath) },
		func() (string, error) { return recentUploadTitled(run, pkg.Title) })
	if err != nil {
		return err
	}
	if decision.action == actionSync {
		fmt.Fprintf(run.out, "Already on YouTube (%s): %s\nSyncing it instead of uploading.\n",
			decision.reason, ytpkg.VideoURL(decision.videoID))
		if !strings.Contains(decision.reason, "spreadsheet") {
			printPasteLines(run.out, msg, decision.videoID, "")
		}
		return run.syncByID(decision.videoID, msg)
	}

	// what joining the playlists will take; the video does not exist yet, so it has
	// no ID to look for in them
	steps, err := run.planPlaylists("", pkg)
	if err != nil {
		return err
	}

	printProposal(run, info, steps)
	if !confirm("Upload to YouTube?", true, true) {
		fmt.Fprintf(run.out, "Not uploaded.\n")
		return nil
	}

	// a new playlist is asked about now, separately, so the steps can run unattended
	create := map[string]bool{}
	for _, step := range steps {
		if step.playlistID != "" || step.duplicates > 1 {
			continue
		}
		if len(step.near) > 0 {
			fmt.Fprintf(run.out, "No playlist is titled %q. Similar ones: %s\n",
				step.ref.Title, quoteTitles(step.near))
		}
		create[step.ref.Title] = confirm(fmt.Sprintf("Create a new playlist %q?", step.ref.Title), false, false)
	}

	return runPublishSteps(run, info, steps, create, record, recordPath)
}

// recordedUpload finds the video the upload record has for a file. A recorded video
// that YouTube no longer has - deleted after a test, say - is forgotten, so it does
// not block a fresh upload.
func recordedUpload(run *syncRun, record *uploadRecord, recordPath, videoPath string) (string, error) {
	id, err := record.find(videoPath)
	if err != nil || id == "" {
		return "", err
	}

	found, err := run.yt.GetVideos(id)
	if err != nil {
		return "", err
	}
	if len(found) == 0 {
		fmt.Fprintf(run.out, "This file was uploaded before as %s, which is no longer on YouTube.\n", id)
		return "", record.forget(recordPath, id)
	}
	return id, nil
}

// recentUploadTitled finds one of the channel's latest uploads with exactly this title
func recentUploadTitled(run *syncRun, title string) (string, error) {
	recent, err := run.yt.RecentUploads(recentUploadCount)
	if err != nil {
		return "", err
	}
	for _, item := range recent {
		if item.Title == title {
			return item.VideoID, nil
		}
	}
	return "", nil
}

// printProposal shows what publishing will do, before it is approved
func printProposal(run *syncRun, info *MessageInfo, steps []playlistStep) {
	pkg := info.Packet

	b := box.New(run.out)
	b.Header("Publish · " + pkg.Title)
	b.Labeled("Video    : ", fmt.Sprintf("%s (%s)", filepath.Base(info.VideoPath), fileSize(info.VideoPath)))
	b.Labeled("Privacy  : ", fmt.Sprintf("private while publishing, then %s", pkg.Privacy))
	if pkg.ThumbnailPath != "" {
		b.Labeled("Thumbnail: ", filepath.Base(pkg.ThumbnailPath))
	} else {
		b.Labeled("Thumbnail: ", "none - YouTube will pick a frame")
	}
	for _, step := range steps {
		b.Labeled("Playlist : ", step.change().Summary)
		if step.playlistID != "" && step.ref.Position > 0 {
			if moves := estimateMoves(run, step); moves > 0 {
				b.Labeled("Reorder  : ", fmt.Sprintf("%d moves in %q, about %d quota units",
					moves, step.ref.Title, moves*moveQuota))
			}
		}
	}
	b.Line("An upload uses about 1,600 of the 10,000 quota units YouTube allows a day.")
	b.Footer()
}

// newVideoPlaceholder stands in for the video not yet uploaded when planning a reorder
const newVideoPlaceholder = "(new video)"

// estimateMoves works out how many moves will put an existing playlist in order once
// the new video is added at its end
func estimateMoves(run *syncRun, step playlistStep) int {
	series, err := run.yt.GetPlaylist(step.playlistID)
	if err != nil {
		return 0
	}
	items := append([]ytpkg.PlaylistItem{}, series.Items...)
	items = append(items, ytpkg.PlaylistItem{
		ItemID: newVideoPlaceholder, VideoID: newVideoPlaceholder, Position: len(items),
	})
	tracks := run.tracksFor(step.ref.Title)
	tracks[newVideoPlaceholder] = step.ref.Position
	return len(ytpkg.PlanReorder(items, tracks))
}

// runPublishSteps uploads and finishes the video, showing each step as it runs
func runPublishSteps(
	run *syncRun,
	info *MessageInfo,
	steps []playlistStep,
	create map[string]bool,
	record *uploadRecord,
	recordPath string,
) error {
	pkg, msg := info.Packet, info.Message

	p := newStepPrinter(run.out,
		fmt.Sprintf("Upload video (%s)", fileSize(info.VideoPath)),
		"Set thumbnail",
		"Create playlist",
		"Add to playlist",
		"Reorder playlist",
		fmt.Sprintf("Set privacy: %s", pkg.Privacy),
	)
	saved := run.yt.Progress
	run.yt.Progress = p
	defer func() { run.yt.Progress = saved }()

	fmt.Fprintf(run.out, "Publishing %q\n", pkg.Title)

	// 1. upload, private
	p.start()
	started := time.Now()
	lastPercent := -1
	id, err := run.yt.Upload(pkg, info.VideoPath, func(sent, total int64) {
		if total <= 0 {
			return
		}
		if percent := int(sent * 100 / total); percent != lastPercent {
			lastPercent = percent
			p.progress(percent)
		}
	})
	if err != nil {
		p.fail(err)
		return err
	}
	// recorded before anything else can fail, so a re-run cannot upload it again
	if err := record.add(recordPath, info.VideoPath, id, pkg.Title); err != nil {
		fmt.Fprintf(p, "  WARNING: could not record the upload: %s\n", err)
	}
	run.recordTracks(id, pkg)
	p.finish("done  " + formatDuration(time.Since(started)))

	// 2. thumbnail
	p.start()
	if pkg.ThumbnailPath == "" {
		p.finish("skipped (no thumbnail)")
	} else if err := run.yt.SetThumbnail(id, pkg.ThumbnailPath); err != nil {
		p.fail(err)
		return err
	} else {
		p.finish("done")
	}

	// 3. create playlists the operator agreed to
	p.start()
	var created, declined int
	var newPlaylists []*ytpkg.Series
	for index := range steps {
		step := &steps[index]
		if step.playlistID != "" || step.duplicates > 1 {
			continue
		}
		if !create[step.ref.Title] {
			declined++
			continue
		}
		series, err := run.newPlaylistFor(msg, step.ref)
		if err == nil {
			series.ID, err = run.yt.InsertPlaylist(series)
		}
		if err != nil {
			p.fail(err)
			return err
		}
		step.playlistID = series.ID
		run.created = append(run.created, series)
		newPlaylists = append(newPlaylists, series)
		run.dropPlaylistSnapshot()
		created++
	}
	switch {
	case created > 0:
		p.finish(fmt.Sprintf("done (%d)", created))
	case declined > 0:
		p.finish("skipped (declined)")
	default:
		p.finish("skipped (exists)")
	}

	// 4. add to playlists
	p.start()
	var added []playlistStep
	for _, step := range steps {
		if step.playlistID == "" {
			continue
		}
		if _, err := run.yt.AddToPlaylist(step.playlistID, id); err != nil {
			p.fail(err)
			return err
		}
		if _, ok := run.touched[step.playlistID]; !ok {
			run.touchedOrder = append(run.touchedOrder, step.playlistID)
		}
		run.touched[step.playlistID] = &ytpkg.Series{ID: step.playlistID, Title: step.ref.Title}
		run.dropPlaylistSnapshot()
		added = append(added, step)
	}
	if len(added) == 0 {
		p.finish("skipped (no playlist)")
	} else {
		p.finish(fmt.Sprintf("done (%d)", len(added)))
	}

	// 5. reorder. A playlist not manually ordered is reported below rather than
	// failing the publish: the video itself is fine, and the order can be fixed later.
	p.start()
	moves, needsManual, err := reorderAdded(run, added)
	if err != nil {
		p.fail(err)
		return err
	}
	switch {
	case len(needsManual) > 0:
		p.finish(`skipped: set Video ordering to "Manual" (see below)`)
	case moves > 0:
		p.finish(fmt.Sprintf("%d moves, done", moves))
	default:
		p.finish("in order")
	}

	// 6. target privacy, last
	p.start()
	want := *pkg
	want.ID = id
	if err := run.yt.UpdateVideo(&want); err != nil {
		p.fail(err)
		return err
	}
	p.finish("done")

	for _, series := range newPlaylists {
		printManualReminder(run.out, series, "")
	}
	for _, series := range needsManual {
		printManualReminder(run.out, series, "Then re-run sync on the playlist to put it in order.")
	}

	summary := info.Summary
	if summary == msg.Description {
		summary = "" // the sheet already has it
	}
	printPasteLines(run.out, msg, id, summary)
	return nil
}

// reorderAdded puts each playlist the new video was added to in track order, without
// asking: the operator approved the upload knowing the moves. Returns the moves made,
// and the playlists YouTube would not reorder because they are not manually ordered.
func reorderAdded(run *syncRun, added []playlistStep) (int, []*ytpkg.Series, error) {
	total := 0
	var needsManual []*ytpkg.Series

	for _, step := range added {
		series, err := run.yt.GetPlaylist(step.playlistID)
		if err != nil {
			return total, needsManual, err
		}

		for _, move := range ytpkg.PlanReorder(series.Items, run.tracksFor(series.Title)) {
			err := run.yt.MoveItem(series.ID, move)
			if errors.Is(err, ytclient.ErrManualSortRequired) {
				needsManual = append(needsManual, series)
				run.needsManual = append(run.needsManual, series)
				break
			}
			if err != nil {
				return total, needsManual, err
			}
			total++
		}
	}
	return total, needsManual, nil
}

// printPasteLines shows what to paste into the spreadsheet row now that the video is
// on YouTube: its link for the Video column, and the generated summary for the
// Description column when the row does not have it. The sheet is never written to.
//
// Until the summary is pasted, a later sync warns that it would remove the YouTube
// description. That is the intended reminder.
func printPasteLines(w io.Writer, msg *catalog.CatalogMessage, videoID, summary string) {
	b := box.New(w)
	b.Header("Paste into the spreadsheet")
	b.Labeled("Row        : ", msg.DescribeForChoice())
	b.Line("Video column:")
	b.Copy(ytpkg.VideoURL(videoID))
	if summary != "" {
		b.Line("Description column:")
		b.Copy(summary)
	}
	b.Footer()
}

// fileSize renders a file's size for reading: "1.2 GB", "340 MB"
func fileSize(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "size unknown"
	}
	size := float64(info.Size())
	switch {
	case size >= 1<<30:
		return fmt.Sprintf("%.1f GB", size/(1<<30))
	case size >= 1<<20:
		return fmt.Sprintf("%.0f MB", size/(1<<20))
	case size >= 1<<10:
		return fmt.Sprintf("%.0f KB", size/(1<<10))
	}
	return fmt.Sprintf("%d bytes", info.Size())
}
