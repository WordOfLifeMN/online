package ytclient

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/WordOfLifeMN/online/util"
	"github.com/WordOfLifeMN/online/ytpkg"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// maxPerCall is the most IDs or results the API returns in one call
const maxPerCall = 50

// scanWorkers is how many playlists are read at once when scanning the channel. The
// scan is bound by round trips, not quota, so a few at a time turns 20 seconds into
// three without costing a unit more.
const scanWorkers = 8

// Client reads (and, later, writes) one channel's videos and playlists
type Client struct {
	ctx context.Context
	svc *youtube.Service

	// ChannelID is the channel the token was granted for, read from YouTube when the
	// client is made. Callers compare it with the channel they expected.
	ChannelID string

	// Progress receives the lines that explain a pause, such as waiting to retry a
	// write. Discarded unless the caller sets it.
	Progress io.Writer

	// the channel's own playlists and their items, fetched once per run or loaded
	// from a snapshot. items is filled by several goroutines during a scan.
	playlists []*ytpkg.Series
	mu        sync.Mutex
	items     map[string][]ytpkg.PlaylistItem

	// created holds the playlists this run made, and uploaded the videos. They are
	// known to be this channel's, and reading one back straight after making it can
	// fail: YouTube's reads lag its writes.
	created  map[string]bool
	uploaded map[string]bool
}

// wasUploaded reports whether this run uploaded a video
func (c *Client) wasUploaded(videoID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.uploaded[videoID]
}

// New makes a client and reads which channel it is authorised for
func New(ctx context.Context, httpClient *http.Client, opts ...option.ClientOption) (*Client, error) {
	opts = append([]option.ClientOption{option.WithHTTPClient(httpClient)}, opts...)
	svc, err := youtube.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("cannot create the YouTube client: %w", err)
	}

	resp, err := svc.Channels.List([]string{"id"}).Mine(true).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("cannot read the signed-in YouTube channel: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("the signed-in YouTube account has no channel")
	}

	return &Client{
		ctx:       ctx,
		svc:       svc,
		ChannelID: resp.Items[0].Id,
		Progress:  io.Discard,
		items:     map[string][]ytpkg.PlaylistItem{},
		created:   map[string]bool{},
		uploaded:  map[string]bool{},
	}, nil
}

// GetVideos reads videos by ID, in batches of 50, and returns them in the order asked
// for. A video that does not exist, or that this account cannot see, is simply absent
// from the result.
func (c *Client) GetVideos(ids ...string) ([]*ytpkg.Message, error) {
	found := map[string]*ytpkg.Message{}

	for start := 0; start < len(ids); start += maxPerCall {
		batch := ids[start:min(start+maxPerCall, len(ids))]
		resp, err := c.svc.Videos.List([]string{"snippet", "status"}).
			Id(batch...).
			MaxResults(maxPerCall).
			Context(c.ctx).
			Do()
		if err != nil {
			return nil, fmt.Errorf("cannot read videos: %w", err)
		}
		for _, v := range resp.Items {
			found[v.Id] = messageFromVideo(v)
		}
	}

	var out []*ytpkg.Message
	seen := map[string]bool{}
	for _, id := range ids {
		if m, ok := found[id]; ok && !seen[id] {
			seen[id] = true
			out = append(out, m)
		}
	}
	return out, nil
}

// GetPlaylist reads a playlist and all its items
func (c *Client) GetPlaylist(id string) (*ytpkg.Series, error) {
	resp, err := c.svc.Playlists.List([]string{"snippet", "status"}).Id(id).Context(c.ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("cannot read playlist %s: %w", id, err)
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("playlist %s was not found", id)
	}

	series := seriesFromPlaylist(resp.Items[0])
	series.Items, err = c.playlistItems(id)
	if err != nil {
		return nil, err
	}
	return series, nil
}

// FindPlaylists looks a title up among the channel's own playlists. The sheet stores no
// playlist IDs, so the title is the key.
//
// exact holds the playlists with exactly that title. near holds the rest whose titles
// match once case and punctuation are ignored, or where one contains the other: a
// playlist that was renamed slightly, which should be seen before a new one is made.
func (c *Client) FindPlaylists(title string) (exact []*ytpkg.Series, near []*ytpkg.Series, err error) {
	playlists, err := c.ownPlaylists()
	if err != nil {
		return nil, nil, err
	}

	want := util.NormalizeForMatch(title)
	for _, p := range playlists {
		if p.Title == title {
			exact = append(exact, p)
			continue
		}

		have := util.NormalizeForMatch(p.Title)
		if want == "" || have == "" {
			continue
		}
		if have == want || strings.Contains(have, want) || strings.Contains(want, have) {
			near = append(near, p)
		}
	}

	return exact, near, nil
}

// PlaylistsContaining lists the channel's own playlists that hold a video. Each
// position counts from 1, like a track, so it can be read against the sheet.
//
// Unless a snapshot is in use, this reads the items of every playlist on the channel,
// once per run, at a quota unit for each page of 50.
func (c *Client) PlaylistsContaining(videoID string) ([]ytpkg.PlaylistRef, error) {
	playlists, err := c.scanPlaylists()
	if err != nil {
		return nil, err
	}

	var refs []ytpkg.PlaylistRef
	for _, p := range playlists {
		for _, item := range c.cachedItems(p.ID) {
			if item.VideoID == videoID {
				refs = append(refs, ytpkg.PlaylistRef{Title: p.Title, ID: p.ID, Position: item.Position + 1})
			}
		}
	}
	return refs, nil
}

// PlaylistSnapshot is every playlist on a channel with its items, as read at one time.
// It is what a scan produces, saved so that later runs can skip the scan.
type PlaylistSnapshot struct {
	ChannelID string          `json:"channelId"`
	Playlists []*ytpkg.Series `json:"playlists"`
}

// SnapshotPlaylists scans the channel's playlists, if this run has not already, and
// returns them with their items
func (c *Client) SnapshotPlaylists() (*PlaylistSnapshot, error) {
	playlists, err := c.scanPlaylists()
	if err != nil {
		return nil, err
	}

	snapshot := &PlaylistSnapshot{ChannelID: c.ChannelID}
	for _, p := range playlists {
		withItems := *p
		withItems.Items = c.cachedItems(p.ID)
		snapshot.Playlists = append(snapshot.Playlists, &withItems)
	}
	return snapshot, nil
}

// UseSnapshot loads a saved snapshot in place of a scan. A snapshot of another channel
// is refused: its playlists are not this channel's.
func (c *Client) UseSnapshot(snapshot *PlaylistSnapshot) error {
	if snapshot.ChannelID != c.ChannelID {
		return fmt.Errorf("the playlist snapshot is for channel %s, not %s",
			snapshot.ChannelID, c.ChannelID)
	}

	playlists := make([]*ytpkg.Series, 0, len(snapshot.Playlists))
	items := make(map[string][]ytpkg.PlaylistItem, len(snapshot.Playlists))
	for _, p := range snapshot.Playlists {
		withoutItems := *p
		withoutItems.Items = nil
		playlists = append(playlists, &withoutItems)
		items[p.ID] = append([]ytpkg.PlaylistItem{}, p.Items...)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.playlists = playlists
	c.items = items
	return nil
}

// scanPlaylists reads every playlist on the channel and all their items, several
// playlists at a time. Whatever this run already holds is not read again.
func (c *Client) scanPlaylists() ([]*ytpkg.Series, error) {
	playlists, err := c.ownPlaylists()
	if err != nil {
		return nil, err
	}

	var todo []string
	for _, p := range playlists {
		if c.cachedItems(p.ID) == nil {
			todo = append(todo, p.ID)
		}
	}
	if len(todo) == 0 {
		return playlists, nil
	}

	log.Printf("Scanning %d of the channel's %d playlists for their videos...",
		len(todo), len(playlists))
	start := time.Now()

	jobs := make(chan string)
	errs := make(chan error, len(todo))
	var wg sync.WaitGroup
	for range min(scanWorkers, len(todo)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				if _, err := c.playlistItems(id); err != nil {
					errs <- err
				}
			}
		}()
	}
	for _, id := range todo {
		jobs <- id
	}
	close(jobs)
	wg.Wait()
	close(errs)

	if err := <-errs; err != nil {
		return nil, err
	}

	videos := 0
	for _, p := range playlists {
		videos += len(c.cachedItems(p.ID))
	}
	log.Printf("Read %d playlists holding %d videos in %s",
		len(playlists), videos, time.Since(start).Round(100*time.Millisecond))

	return playlists, nil
}

// cachedItems returns the items this run holds for a playlist, or nil if it has not
// read them
func (c *Client) cachedItems(playlistID string) []ytpkg.PlaylistItem {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.items[playlistID]
}

// ownPlaylists reads every playlist on the channel, all pages, once per run
func (c *Client) ownPlaylists() ([]*ytpkg.Series, error) {
	if c.playlists != nil {
		return c.playlists, nil
	}

	playlists := []*ytpkg.Series{}
	call := c.svc.Playlists.List([]string{"snippet", "status"}).Mine(true).MaxResults(maxPerCall)
	err := call.Pages(c.ctx, func(resp *youtube.PlaylistListResponse) error {
		for _, p := range resp.Items {
			playlists = append(playlists, seriesFromPlaylist(p))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("cannot list the channel's playlists: %w", err)
	}

	c.playlists = playlists
	return playlists, nil
}

// playlistItems reads every item in a playlist, all pages, once per run
func (c *Client) playlistItems(playlistID string) ([]ytpkg.PlaylistItem, error) {
	if items := c.cachedItems(playlistID); items != nil {
		return items, nil
	}

	items := []ytpkg.PlaylistItem{}
	call := c.svc.PlaylistItems.List([]string{"snippet", "contentDetails", "status"}).
		PlaylistId(playlistID).
		MaxResults(maxPerCall)
	err := call.Pages(c.ctx, func(resp *youtube.PlaylistItemListResponse) error {
		for _, i := range resp.Items {
			items = append(items, itemFromPlaylistItem(i))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("cannot read the items of playlist %s: %w", playlistID, err)
	}

	c.mu.Lock()
	c.items[playlistID] = items
	c.mu.Unlock()
	return items, nil
}
