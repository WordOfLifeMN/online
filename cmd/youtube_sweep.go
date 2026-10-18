package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/util/box"
	"github.com/WordOfLifeMN/online/ytclient"
	"github.com/WordOfLifeMN/online/ytpkg"
)

// moveQuota is what one playlistItems.update costs
const moveQuota = 50

// sweepPlaylists brings every playlist this run touched, and every playlist named on
// the command line, into line with the sheet: first its description and privacy, then
// its order. Each is shown and approved like a video. An error on one playlist is
// reported and the sweep moves on to the next.
func (r *syncRun) sweepPlaylists() error {
	var ids []string
	seen := map[string]bool{}
	for _, id := range append(append([]string{}, r.touchedOrder...), r.named...) {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}

	var errs []error
	for _, id := range ids {
		if err := r.sweepPlaylist(id); err != nil {
			fmt.Fprintf(r.out, "ERROR     playlist %s  %s\n\n", id, err)
			errs = append(errs, fmt.Errorf("playlist %s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

// sweepPlaylist syncs one playlist's fields and order
func (r *syncRun) sweepPlaylist(id string) error {
	live, err := r.yt.GetPlaylist(id)
	if err != nil {
		return err
	}
	if live.ChannelID != r.yt.ChannelID {
		fmt.Fprintf(r.out, "skipped   playlist %q: belongs to channel %s, not the signed-in %s\n",
			live.Title, live.ChannelID, r.yt.ChannelID)
		return nil
	}

	if err := r.syncPlaylistFields(live); err != nil {
		return err
	}
	return r.reorderPlaylist(live)
}

// syncPlaylistFields offers to make a playlist's description and privacy match its
// series. The title is never synced: it is how the playlist was found.
func (r *syncRun) syncPlaylistFields(live *ytpkg.Series) error {
	seri := r.seriesForPlaylist(live.Title)
	if seri == nil {
		// Ask the Pastor, or a series with no Series row: nothing to compare against
		return nil
	}

	want, err := ytpkg.NewSeriesForLive(seri, live)
	if err != nil {
		return err
	}
	changes := ytpkg.DiffSeries(live, want)
	if len(changes) == 0 {
		return nil
	}

	b := box.New(r.out)
	b.Header(fmt.Sprintf("Playlist %s · %s", live.ID, live.Title))
	b.Labeled("Sheet   : ", fmt.Sprintf("series %q", seri.Name))
	ytpkg.PrintChanges(b, changes)
	b.Footer()

	if !r.approve(changes) {
		fmt.Fprintf(r.out, "Skipped the playlist's fields.\n\n")
		return nil
	}
	if err := r.yt.UpdatePlaylist(want); err != nil {
		return err
	}
	r.dropPlaylistSnapshot()
	fmt.Fprintf(r.out, "  Updated the playlist.\n\n")
	return nil
}

// reorderPlaylist offers to put a playlist in track order, with as few moves as can do
// it. A playlist not manually ordered in Studio refuses, and is reported with the link
// to fix it rather than as an error.
func (r *syncRun) reorderPlaylist(live *ytpkg.Series) error {
	tracks := r.tracksFor(live.Title)
	moves := ytpkg.PlanReorder(live.Items, tracks)
	if len(moves) == 0 {
		fmt.Fprintf(r.out, "in order  playlist %q\n", live.Title)
		return nil
	}

	b := box.New(r.out)
	b.Header(fmt.Sprintf("Playlist %s · %s", live.ID, live.Title))
	printProposedOrder(b, live.Items, tracks, moves)
	b.Footer()

	if !confirm("Reorder the playlist?", true, true) {
		fmt.Fprintf(r.out, "Skipped the reorder.\n\n")
		return nil
	}

	for index, move := range moves {
		err := r.yt.MoveItem(live.ID, move)
		if errors.Is(err, ytclient.ErrManualSortRequired) {
			r.needsManual = append(r.needsManual, live)
			if index > 0 {
				fmt.Fprintf(r.out, "  Moved %d of %d before YouTube refused.\n", index, len(moves))
			}
			printManualReminder(r.out, live, "Then re-run sync to put it in order.")
			fmt.Fprintln(r.out)
			return nil
		}
		if err != nil {
			return err
		}
	}

	r.dropPlaylistSnapshot()
	fmt.Fprintf(r.out, "  Reordered the playlist: %d moves.\n\n", len(moves))
	return nil
}

// printProposedOrder lists a playlist in its proposed order, each item with its track
// and, for those that move, where it is now
func printProposedOrder(b *box.Box, items []ytpkg.PlaylistItem, tracks map[string]int, moves []ytpkg.Move) {
	was := map[string]int{}
	for _, item := range items {
		was[item.ItemID] = item.Position
	}
	moving := map[string]bool{}
	for _, move := range moves {
		moving[move.ItemID] = true
	}

	b.Section("Proposed order")
	for index, item := range ytpkg.DesiredOrder(items, tracks) {
		notes := []string{}
		if track := tracks[item.VideoID]; track > 0 {
			notes = append(notes, fmt.Sprintf("track %d", track))
		} else {
			notes = append(notes, "not numbered")
		}
		if moving[item.ItemID] {
			notes = append(notes, fmt.Sprintf("moves from %d", was[item.ItemID]+1))
		}
		b.Labeled(fmt.Sprintf("%3d. ", index+1), fmt.Sprintf("%s  (%s)", item.Title, strings.Join(notes, ", ")))
	}
	b.Blank()
	b.Line(fmt.Sprintf("%d moves, about %d quota units.", len(moves), len(moves)*moveQuota))
}

// seriesForPlaylist finds the series in the sheet whose playlist has a title. Returns
// nil for Ask the Pastor, and for a series with no Series row.
func (r *syncRun) seriesForPlaylist(title string) *catalog.CatalogSeri {
	for index := range r.cat.Series {
		seri := &r.cat.Series[index]
		if ytpkg.FindSeries(r.cat, seri.Name) != seri {
			// a stand-alone series, or a second series of the same name
			continue
		}
		if ytpkg.PlaylistTitle(seri.GetMinistry(), seri.Name) == title {
			return seri
		}
	}
	return nil
}

// tracksFor maps each video the sheet numbers in a playlist to its track. The sheet's
// rows say which video is which by their Video column; this run's own videos are added
// on top, which covers a video just uploaded whose link is not in the sheet yet.
func (r *syncRun) tracksFor(title string) map[string]int {
	tracks := map[string]int{}
	for index := range r.cat.Messages {
		msg := &r.cat.Messages[index]
		if msg.Video == nil {
			continue
		}
		id, ok := ytpkg.ParseVideoRef(msg.Video.URL)
		if !ok {
			continue
		}
		for _, ref := range ytpkg.PlaylistsForMessage(msg, r.cat) {
			if ref.Title == title && ref.Position > 0 {
				tracks[id] = ref.Position
			}
		}
	}

	for id, track := range r.trackOf[title] {
		tracks[id] = track
	}
	return tracks
}
