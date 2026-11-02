package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/util"
	"github.com/WordOfLifeMN/online/util/box"
	"github.com/WordOfLifeMN/online/ytclient"
	"github.com/WordOfLifeMN/online/ytpkg"
	"github.com/spf13/cobra"
)

var youtubeSyncCmd = &cobra.Command{
	Use:   "sync <video, playlist, file or folder>...",
	Short: "Make videos on YouTube match the spreadsheet",
	Long: `Compare videos on YouTube with what the spreadsheet says they should be, and
offer to make YouTube match.

Each reference is a video or a playlist, as for 'youtube get', or a video file or a
folder. A playlist means every video in it. A file means the YouTube video in its
spreadsheet row, found by the date in its name as 'audio' finds it, choosing between
rows when several share the date. A folder means every .mp4 file in it and in the
folders below it. A file with no row, or whose row has no YouTube video, is warned
about and skipped. Each video is shown with its differences and confirmed in turn.

With no references given, they are asked for. Paste or type any number, separated by
spaces, tabs or new lines - a column of links copied from the spreadsheet works -
or drag in files and folders, and finish with an empty line.

Only the title, description, privacy, category, languages, made-for-kids and
playlist membership are compared. Everything else on YouTube is left as it is. A
video is added to the playlists its series call for, and never removed from one.

A change that would remove description text that is not in the spreadsheet is
marked with a WARNING, defaults to No, and is asked about even with --yes. So is
creating a playlist.

The spreadsheet is read through a cache that is kept for an hour. Use --refresh
after editing it.`,
	Example: `  online youtube sync 01LiNfRzMo8
  online youtube sync https://www.youtube.com/playlist?list=PL...
  online youtube sync "D:\Videos\2026-10-04 Pay Attention.mp4"
  online youtube sync D:\Videos\Watchmen
  online youtube sync --thumbnail cover.jpg 01LiNfRzMo8
  online youtube sync        (asks for them, so a column can be pasted from the sheet)`,
	Args: cobra.ArbitraryArgs,
	RunE: youtubeSync,
}

func init() {
	youtubeCmd.AddCommand(youtubeSyncCmd)

	youtubeSyncCmd.Flags().String("thumbnail", "",
		"Image to upload as the thumbnail. Only with a single video")
}

// syncRun is the state of one sync, shared by every video it covers. The audio command
// will share one too, so its end-of-run playlist sweep covers every upload.
type syncRun struct {
	cat     *catalog.Catalog
	yt      *ytclient.Client
	channel ytpkg.Channel
	out     io.Writer

	// thumbnail is an image to upload with the (single) video being synced
	thumbnail string

	// the spreadsheet rows, by the video ID in their Video column
	byVideo map[string][]catalog.CatalogMessage

	// fileRows are the rows already matched to a file given to sync, by video ID. The
	// file chose its row, so it is used rather than asking again.
	fileRows map[string]*catalog.CatalogMessage

	// playlists this run added a video to or created, by ID, and those named on the
	// command line: the end-of-run sweep covers both
	touched      map[string]*ytpkg.Series
	touchedOrder []string
	named        []string

	// created lists the playlists this run made, and needsManual those YouTube would
	// not reorder. Each needs its Video ordering set to Manual in Studio, which the API
	// cannot do.
	created     []*ytpkg.Series
	needsManual []*ytpkg.Series

	// trackOf is each playlist's sheet-numbered videos: title, then video ID, to track
	trackOf map[string]map[string]int
}

func newSyncRun(cat *catalog.Catalog, yt *ytclient.Client, channel ytpkg.Channel, out io.Writer) *syncRun {
	return &syncRun{
		cat:     cat,
		yt:      yt,
		channel: channel,
		out:     out,
		byVideo: indexByVideoID(cat),
		touched: map[string]*ytpkg.Series{},
		trackOf: map[string]map[string]int{},
	}
}

// promptForRefs asks for the videos, playlists, files and folders to sync and returns
// every value entered, in order. Values are separated by any whitespace, so all the
// ways a selection comes out of a spreadsheet work: a column copies as one cell per
// line, a row as cells separated by tabs. See splitRefs for quoted values.
//
// Input ends at an empty line, or when stdin closes. A blank cell in a pasted column
// is also an empty line, so a selection with gaps in it ends at the first gap: copy a
// range without blank cells, or paste it a part at a time.
func promptForRefs(reader *bufio.Reader) []string {
	fmt.Println("Enter or paste the videos or playlists to sync - IDs or links - or drag in")
	fmt.Println("video files or folders, separated by spaces or one per line. Press Enter on an")
	fmt.Println("empty line to finish:")

	var refs []string
	for {
		line, err := reader.ReadString('\n')
		if strings.TrimSpace(line) == "" {
			return refs
		}
		refs = append(refs, splitRefs(line)...)
		if err != nil {
			// the last line, unterminated, before stdin closed
			return refs
		}
	}
}

// splitRefs splits a line of input into values at whitespace, except inside quotes,
// so a dragged file whose path has spaces in it stays one value. The quotes are
// dropped, as cleanDroppedPath drops them:
//
//	"C:\Videos\Pay Attention.mp4"      cmd.exe, or a cell a spreadsheet copied in quotes
//	'C:\Videos\God''s Word.mp4'        single quotes, with '' standing for '
//	& 'C:\Videos\Pay Attention.mp4'    PowerShell, whose "&" is dropped
//
// A quote left open runs to the end of the line.
func splitRefs(line string) []string {
	var values []string
	runes := []rune(line)

	for i := 0; i < len(runes); {
		switch r := runes[i]; {
		case unicode.IsSpace(r):
			i++

		case r == '&' && (i+1 == len(runes) || unicode.IsSpace(runes[i+1])):
			// PowerShell offers a dragged file as a command to run
			i++

		case r == '"' || r == '\'':
			var value strings.Builder
			for i++; i < len(runes); i++ {
				if runes[i] != r {
					value.WriteRune(runes[i])
					continue
				}
				if r == '\'' && i+1 < len(runes) && runes[i+1] == '\'' {
					// '' inside single quotes is one '
					value.WriteRune('\'')
					i++
					continue
				}
				i++
				break
			}
			if v := strings.TrimSpace(value.String()); v != "" {
				values = append(values, v)
			}

		default:
			start := i
			for i < len(runes) && !unicode.IsSpace(runes[i]) {
				i++
			}
			values = append(values, string(runes[start:i]))
		}
	}

	return values
}

// resolvePathRefs replaces each file or folder among args with the YouTube video its
// spreadsheet row links to, leaving every other value as it is for parseGetRefs. A
// folder means every .mp4 in it and in the folders below it.
//
// A file is found in the spreadsheet as 'audio' finds it - by the date in its name,
// choosing between rows when several share the date - and the row it was matched to is
// returned by video ID, so the sync uses that row rather than asking again. A file that
// cannot be matched to a row with a YouTube video is warned about and left out.
func resolvePathRefs(
	args []string,
	cat *catalog.Catalog,
) ([]string, map[string]*catalog.CatalogMessage) {
	var out []string
	rows := map[string]*catalog.CatalogMessage{}

	for _, arg := range args {
		// only a file or folder that is really there. Not DoesPathExist: that takes any
		// error but "not found" to mean found, and Windows answers a link - with its "?"
		// and second ":" - with "the syntax is incorrect", which would turn every link
		// into a file.
		path := util.NormalizePath(cleanDroppedPath(arg))
		if path == "" || !(util.IsFile(path) || util.IsDirectory(path)) {
			out = append(out, arg)
			continue
		}

		files := []string{path}
		if util.IsDirectory(path) {
			var err error
			if files, err = videosInTree(path); err != nil {
				fmt.Println(box.Yellow(os.Stdout, fmt.Sprintf(
					"WARNING: cannot read the folder %s, skipped: %s", path, err)))
				continue
			}
			if len(files) == 0 {
				fmt.Println(box.Yellow(os.Stdout, fmt.Sprintf(
					"WARNING: there are no .mp4 files in %s, skipped", path)))
				continue
			}
			fmt.Printf("Found %d video(s) in %s\n", len(files), path)
		}

		for _, file := range files {
			fmt.Printf("\n── %s\n", filepath.Base(file))

			id, msg, err := videoIDForFile(file, cat)
			if err != nil {
				fmt.Println(box.Yellow(os.Stdout, fmt.Sprintf(
					"WARNING: %s, skipped: %s", filepath.Base(file), err)))
				continue
			}

			fmt.Printf("   YouTube video: %s\n", id)
			if _, ok := rows[id]; !ok {
				rows[id] = msg
			}
			out = append(out, id)
		}
	}

	return out, rows
}

// videoIDForFile finds a file's spreadsheet row and the YouTube video it links to
func videoIDForFile(
	file string,
	cat *catalog.Catalog,
) (string, *catalog.CatalogMessage, error) {
	msg, _, err := findMessageForVideo(file, cat)
	if err != nil {
		return "", nil, err
	}

	if msg.Video == nil {
		return "", nil, fmt.Errorf("the spreadsheet row %q has no video", msg.Name)
	}
	id, ok := ytpkg.ParseVideoRef(msg.Video.URL)
	if !ok {
		return "", nil, fmt.Errorf("the spreadsheet row %q has no YouTube video, only %s",
			msg.Name, msg.Video.URL)
	}

	return id, msg, nil
}

// videosInTree returns every .mp4 file in dir and the folders below it, in name order
// folder by folder
func videosInTree(dir string) ([]string, error) {
	var videos []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() && strings.EqualFold(filepath.Ext(path), ".mp4") {
			videos = append(videos, path)
		}
		return nil
	})
	return videos, err
}

// indexByVideoID maps each YouTube video ID found in the sheet's Video column to the
// rows that carry it. Several rows can carry one ID - a message and its second service,
// or a copy-and-paste mistake - so each ID maps to a list.
//
// Every link form the sheet uses is understood, including the malformed youtu.beID.
// Rows with no video, or with a Rumble or S3 link, are not YouTube rows and are left
// out.
func indexByVideoID(cat *catalog.Catalog) map[string][]catalog.CatalogMessage {
	index := map[string][]catalog.CatalogMessage{}
	for _, msg := range cat.Messages {
		if msg.Video == nil {
			continue
		}
		if id, ok := ytpkg.ParseVideoRef(msg.Video.URL); ok {
			index[id] = append(index[id], msg)
		}
	}
	return index
}

func youtubeSync(cmd *cobra.Command, args []string) error {
	initLogging()

	if len(args) == 0 {
		args = promptForRefs(getStdin())
		if len(args) == 0 {
			fmt.Println("Nothing to sync")
			return nil
		}
		fmt.Printf("Read %d reference(s)\n", len(args))
	}

	thumbnail, _ := cmd.Flags().GetString("thumbnail")
	if thumbnail != "" {
		thumbnail = util.NormalizePath(cleanDroppedPath(thumbnail))
		if !util.IsFile(thumbnail) {
			return fmt.Errorf("cannot find the thumbnail %s", thumbnail)
		}
	}

	cat, err := readCatalogCached(cmd.Context())
	if err != nil {
		return err
	}
	if err := cat.Initialize(); err != nil {
		return fmt.Errorf("cannot initialize the catalog: %w", err)
	}

	// files and folders become the videos their rows link to. This asks its questions
	// before signing in, so they are all asked together at the start.
	args, fileRows := resolvePathRefs(args, cat)

	refs, refErr := parseGetRefs(args, false)
	if len(refs) == 0 {
		if refErr == nil {
			fmt.Println("Nothing to sync")
		}
		return refErr
	}

	channel := ytpkg.ChannelWOL
	client, err := newYouTubeClient(cmd.Context(), channel)
	if err != nil {
		return err
	}

	run := newSyncRun(cat, client, channel, os.Stdout)
	run.thumbnail = thumbnail
	run.fileRows = fileRows

	ids, err := run.expandRefs(refs)
	if err != nil {
		return err
	}
	if thumbnail != "" && len(ids) != 1 {
		return fmt.Errorf("--thumbnail needs exactly one video, but this would sync %d", len(ids))
	}

	live := map[string]*ytpkg.Message{}
	found, err := client.GetVideos(ids...)
	if err != nil {
		return err
	}
	for _, v := range found {
		live[v.ID] = v
	}

	errs := []error{refErr}
	for _, id := range ids {
		if err := run.syncVideo(id, live[id]); err != nil {
			fmt.Fprintf(run.out, "ERROR     %s  %s\n\n", id, err)
			errs = append(errs, fmt.Errorf("video %s: %w", id, err))
		}
	}

	errs = append(errs, run.sweepPlaylists())
	run.printSummary()

	return errors.Join(errs...)
}

// expandRefs turns the command line into the video IDs to sync, in order and without
// repeats. A playlist contributes every video in it, and is remembered for the sweep.
func (r *syncRun) expandRefs(refs []getRef) ([]string, error) {
	var ids []string
	seen := map[string]bool{}
	add := func(id string) {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}

	for _, ref := range refs {
		if ref.videoID != "" {
			add(ref.videoID)
			continue
		}

		series, err := r.yt.GetPlaylist(ref.playlistID)
		if err != nil {
			return nil, err
		}
		r.named = append(r.named, series.ID)
		fmt.Fprintf(r.out, "Playlist %q: %d videos\n", series.Title, len(series.Items))
		for _, item := range series.Items {
			add(item.VideoID)
		}
	}

	return ids, nil
}

// syncVideo compares one video with its spreadsheet row, shows the differences, asks
// whether to apply them, and applies them. A video that is skipped - not in the sheet,
// on another channel, declined - is reported and is not an error.
func (r *syncRun) syncVideo(id string, live *ytpkg.Message) error {
	if live == nil {
		return fmt.Errorf("not found, or not visible to this account")
	}

	msg := r.fileRows[id]
	if msg == nil {
		msg = r.chooseRow(id, live)
	}
	if msg == nil {
		return nil
	}
	return r.syncVideoRow(id, live, msg)
}

// syncByID reads a video and syncs it against a given spreadsheet row. This is how
// 'audio' syncs a video it finds already uploaded, whose link may not be in the sheet
// yet, so the row cannot be found by it.
func (r *syncRun) syncByID(id string, msg *catalog.CatalogMessage) error {
	found, err := r.yt.GetVideos(id)
	if err != nil {
		return err
	}
	if len(found) == 0 {
		return fmt.Errorf("video %s was not found, or is not visible to this account", id)
	}
	return r.syncVideoRow(id, found[0], msg)
}

// syncVideoRow is syncVideo once the row is known
func (r *syncRun) syncVideoRow(id string, live *ytpkg.Message, msg *catalog.CatalogMessage) error {
	if live.ChannelID != r.yt.ChannelID {
		r.skip(id, live.Title, fmt.Sprintf("belongs to channel %s, not the signed-in %s",
			live.ChannelID, r.yt.ChannelID))
		return nil
	}
	if ch := ytpkg.ChannelForMinistry(msg.Ministry); ch != r.channel {
		r.skip(id, live.Title, (&notAuthenticatedError{channel: ch}).Error())
		return nil
	}

	want, err := ytpkg.NewMessageForLive(msg, r.cat, live)
	if err != nil {
		return err
	}
	if err := ytpkg.Validate(want.Title, want.Description); err != nil {
		return err
	}

	changes := ytpkg.DiffMessage(live, want)
	updateVideo := len(changes) > 0

	steps, err := r.planPlaylists(id, want)
	if err != nil {
		return err
	}
	for _, step := range steps {
		changes = append(changes, step.change())
	}
	if r.thumbnail != "" {
		changes = append(changes, ytpkg.Change{Field: ytpkg.FieldThumbnail, Summary: "upload " + r.thumbnail})
	}

	if len(changes) == 0 {
		fmt.Fprintf(r.out, "in sync   %s  %s\n", id, live.Title)
		return nil
	}

	b := box.New(r.out)
	b.Header(fmt.Sprintf("%s · %s", id, msg.Name))
	b.Labeled("Sheet   : ", msg.DescribeForChoice())
	b.Labeled("YouTube : ", live.Title)
	ytpkg.PrintChanges(b, changes)
	b.Footer()

	if !r.approve(changes) {
		fmt.Fprintf(r.out, "Skipped %s.\n\n", id)
		return nil
	}

	if updateVideo {
		if err := r.yt.UpdateVideo(want); err != nil {
			return err
		}
		fmt.Fprintf(r.out, "  Updated the video.\n")
	}
	if r.thumbnail != "" {
		if err := r.yt.SetThumbnail(id, r.thumbnail); err != nil {
			return err
		}
		fmt.Fprintf(r.out, "  Set the thumbnail.\n")
	}
	for _, step := range steps {
		if err := r.ensurePlaylist(msg, id, step); err != nil {
			return err
		}
	}

	fmt.Fprintln(r.out)
	return nil
}

// approve asks whether to apply a video's changes. A destructive change defaults to No
// and is asked even with --yes; anything else defaults to Yes and --yes answers it.
func (r *syncRun) approve(changes []ytpkg.Change) bool {
	if ytpkg.HasDestructive(changes) {
		return confirm("Apply these changes, including the WARNING above?", false, false)
	}
	return confirm("Apply these changes?", true, true)
}

// chooseRow finds the spreadsheet row for a video, asking when several rows carry its
// ID. Returns nil, having said why, when there is no row to sync against.
func (r *syncRun) chooseRow(id string, live *ytpkg.Message) *catalog.CatalogMessage {
	rows := r.byVideo[id]
	switch len(rows) {
	case 0:
		r.skip(id, live.Title, "not in the spreadsheet")
		return nil
	case 1:
		return &rows[0]
	}

	chosen := promptUserForChoice(rows, nameFromTitle(live.Title),
		fmt.Sprintf("%d spreadsheet rows point at video %s, %q:", len(rows), id, live.Title))
	if chosen == nil {
		r.skip(id, live.Title, "no spreadsheet row was chosen")
	}
	return chosen
}

// nameFromTitle extracts the message name from a video title in the house format,
// "[CORE | ]Name | Date", or "[CORE | ]Name | Speaker | Date" as titles were before
// 2026-10-17, for ranking it against the sheet's names
func nameFromTitle(title string) string {
	parts := strings.Split(title, " | ")
	if len(parts) > 1 && parts[0] == "CORE" {
		parts = parts[1:]
	}
	return parts[0]
}

// +---------------------------------------------------------------------------
// | Playlist membership
// +---------------------------------------------------------------------------

// playlistStep is what joining one playlist needs
type playlistStep struct {
	ref ytpkg.PlaylistRef

	// playlistID is the existing playlist to add to, or "" when it must be created
	playlistID string

	// near are existing playlists with similar titles, shown before creating one so a
	// renamed playlist is not duplicated
	near []*ytpkg.Series

	// duplicates counts the playlists sharing the title, when there is more than one.
	// Nothing is done then: which one is meant is for the operator to settle in Studio.
	duplicates int
}

// change describes the step as a line of the diff
func (s playlistStep) change() ytpkg.Change {
	var summary string
	switch {
	case s.duplicates > 1:
		summary = fmt.Sprintf("%d playlists are titled %q; rename all but one in Studio. "+
			"Not added to any of them", s.duplicates, s.ref.Title)
	case s.playlistID == "":
		summary = fmt.Sprintf("create %q and add%s", s.ref.Title, describeTrack(s.ref))
		if len(s.near) > 0 {
			summary += ". Similar playlists already exist: " + quoteTitles(s.near)
		}
	default:
		summary = fmt.Sprintf("add to %q%s", s.ref.Title, describeTrack(s.ref))
	}
	return ytpkg.Change{Field: ytpkg.FieldPlaylist, Summary: summary}
}

// planPlaylists works out what joining the video's playlists needs: nothing where it is
// already in one, an addition where the playlist exists, and a new playlist where none
// has the title.
//
// The channel's playlists are read live, once per run, never from the snapshot 'get'
// caches: what is written must not be decided from something an hour old.
//
// It also records each playlist's sheet track for the video, for the reorder.
func (r *syncRun) planPlaylists(id string, want *ytpkg.Message) ([]playlistStep, error) {
	var steps []playlistStep

	r.recordTracks(id, want)

	for _, ref := range want.Playlists {
		exact, near, err := r.yt.FindPlaylists(ref.Title)
		if err != nil {
			return nil, err
		}

		switch len(exact) {
		case 0:
			steps = append(steps, playlistStep{ref: ref, near: near})
		case 1:
			series, err := r.yt.GetPlaylist(exact[0].ID)
			if err != nil {
				return nil, err
			}
			if !containsVideo(series, id) {
				steps = append(steps, playlistStep{ref: ref, playlistID: series.ID})
			}
		default:
			steps = append(steps, playlistStep{ref: ref, duplicates: len(exact)})
		}
	}

	return steps, nil
}

// recordTracks remembers a video's track in each of its playlists, for the reorder. It
// matters most for a video just uploaded, whose link the sheet does not have yet.
func (r *syncRun) recordTracks(id string, want *ytpkg.Message) {
	if id == "" {
		return
	}
	for _, ref := range want.Playlists {
		if ref.Position > 0 {
			if r.trackOf[ref.Title] == nil {
				r.trackOf[ref.Title] = map[string]int{}
			}
			r.trackOf[ref.Title][id] = ref.Position
		}
	}
}

// ensurePlaylist carries out one approved step: creating the playlist first if it must
// be, which is asked separately, defaults to No, and is not answered by --yes.
func (r *syncRun) ensurePlaylist(msg *catalog.CatalogMessage, videoID string, step playlistStep) error {
	if step.duplicates > 1 {
		return nil
	}

	playlistID := step.playlistID
	series := &ytpkg.Series{ID: playlistID, Title: step.ref.Title}

	if playlistID == "" {
		if len(step.near) > 0 {
			fmt.Fprintf(r.out, "  No playlist is titled %q. Similar ones: %s\n",
				step.ref.Title, quoteTitles(step.near))
		}
		if !confirm(fmt.Sprintf("  Create a new playlist %q?", step.ref.Title), false, false) {
			fmt.Fprintf(r.out, "  Not added to %q.\n", step.ref.Title)
			return nil
		}

		var err error
		series, err = r.newPlaylistFor(msg, step.ref)
		if err != nil {
			return err
		}
		if playlistID, err = r.yt.InsertPlaylist(series); err != nil {
			return err
		}
		series.ID = playlistID
		r.created = append(r.created, series)
		r.dropPlaylistSnapshot()

		fmt.Fprintf(r.out, "  Created playlist %q.\n", series.Title)
		printManualReminder(r.out, series, "")
	}

	if _, err := r.yt.AddToPlaylist(playlistID, videoID); err != nil {
		return err
	}
	if _, ok := r.touched[playlistID]; !ok {
		r.touchedOrder = append(r.touchedOrder, playlistID)
	}
	r.touched[playlistID] = series
	r.dropPlaylistSnapshot()
	fmt.Fprintf(r.out, "  Added to %q.\n", step.ref.Title)
	return nil
}

// newPlaylistFor describes the playlist to create for one of a message's playlist
// references: the series' description and booklets, and its privacy. The title is the
// reference's, so that the playlist is found by it next time.
func (r *syncRun) newPlaylistFor(msg *catalog.CatalogMessage, ref ytpkg.PlaylistRef) (*ytpkg.Series, error) {
	for _, seriesRef := range msg.Series {
		seri := ytpkg.FindSeries(r.cat, seriesRef.Name)
		if seri == nil || ytpkg.PlaylistTitle(msg.Ministry, seri.Name) != ref.Title {
			continue
		}
		series, err := ytpkg.NewSeriesFromCatalog(seri)
		if err != nil {
			return nil, err
		}
		series.Title = ref.Title
		return series, nil
	}

	// Ask the Pastor, or a series with no Series row in the sheet: no description, and
	// the message's own privacy
	privacy, err := ytpkg.PrivacyForView(msg.Visibility)
	if err != nil {
		return nil, err
	}
	return &ytpkg.Series{Title: ref.Title, Privacy: privacy}, nil
}

// studioPlaylistURL is where a playlist's settings are edited in YouTube Studio
func studioPlaylistURL(id string) string {
	return fmt.Sprintf("https://studio.youtube.com/playlist/%s/edit", id)
}

// printManualReminder asks for a playlist's Video ordering to be set to Manual, which
// the API can neither read nor set. Without it, the playlist cannot be put in track
// order. then, when set, says what to do once it is.
func printManualReminder(w io.Writer, series *ytpkg.Series, then string) {
	b := box.New(w)
	b.Header(`Set Video ordering to "Manual" in Studio`)
	b.Labeled("Playlist: ", series.Title)
	b.Line("Open the link, then Video ordering, then Manual:")
	b.Copy(studioPlaylistURL(series.ID))
	if then != "" {
		b.Line(then)
	}
	b.Footer()
}

// printSummary closes a run by listing what still needs doing by hand: every playlist
// this run created, or could not reorder, once each
func (r *syncRun) printSummary() {
	var playlists []*ytpkg.Series
	seen := map[string]bool{}
	for _, series := range append(append([]*ytpkg.Series{}, r.created...), r.needsManual...) {
		if !seen[series.ID] {
			seen[series.ID] = true
			playlists = append(playlists, series)
		}
	}
	if len(playlists) == 0 {
		return
	}

	b := box.New(r.out)
	b.Header("To do in Studio")
	b.Line(`Set Video ordering to "Manual" for each of these playlists, then re-run sync ` +
		`for any that could not be put in order:`)
	for _, series := range playlists {
		b.Labeled("Playlist: ", series.Title)
		b.Copy(studioPlaylistURL(series.ID))
	}
	b.Footer()
}

// dropPlaylistSnapshot deletes the cached playlist snapshot 'get' uses, once this run
// has changed a playlist, so that 'get' does not show the old membership for up to an
// hour
func (r *syncRun) dropPlaylistSnapshot() {
	path := playlistSnapshotPath(r.channel)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("WARNING: could not delete %s: %s", path, err)
	}
}

// describeTrack renders a playlist position for a change line
func describeTrack(ref ytpkg.PlaylistRef) string {
	if ref.Position > 0 {
		return fmt.Sprintf(" (track %d)", ref.Position)
	}
	return ""
}

// quoteTitles lists playlists by title
func quoteTitles(playlists []*ytpkg.Series) string {
	titles := make([]string, len(playlists))
	for index, p := range playlists {
		titles[index] = fmt.Sprintf("%q", p.Title)
	}
	return strings.Join(titles, ", ")
}

// containsVideo reports whether a playlist holds a video
func containsVideo(series *ytpkg.Series, videoID string) bool {
	for _, item := range series.Items {
		if item.VideoID == videoID {
			return true
		}
	}
	return false
}

// skip reports a video that is not being synced, and why
func (r *syncRun) skip(id, title, reason string) {
	fmt.Fprintf(r.out, "skipped   %s  %s: %s\n", id, title, reason)
}
