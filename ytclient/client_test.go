package ytclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/WordOfLifeMN/online/ytpkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

const testChannel = "UCZF_jDrfP6OltpviYmj-mkA"

// fakeYouTube serves the read endpoints from fixed data and records every request
type fakeYouTube struct {
	videos    map[string]*youtube.Video
	playlists []*youtube.Playlist
	items     map[string][]*youtube.PlaylistItem

	mu       sync.Mutex
	requests []*http.Request
	writes   []write

	// failItemInserts makes that many playlist insertions fail as a new playlist does,
	// with 409 aborted. With applyFailedInserts, each failed one is applied anyway.
	failItemInserts    int
	applyFailedInserts bool
	failStatus         int // a status other than 409 to fail with

	// moveRefusal, when set, is the reason every playlist item move is refused with
	moveRefusal string

	// lagVideoReads makes that many reads of videos find nothing, as YouTube does
	// straight after an upload
	lagVideoReads int

	// refuseUpload, when set, is the reason an upload is refused with
	refuseUpload string
}

// write is one request that changes something, with its body
type write struct {
	method string
	path   string
	body   []byte
}

func newFakeYouTube() *fakeYouTube {
	return &fakeYouTube{
		videos: map[string]*youtube.Video{},
		items:  map[string][]*youtube.PlaylistItem{},
	}
}

// count reports how many requests went to an endpoint
func (f *fakeYouTube) count(endpoint string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.requests {
		if strings.HasSuffix(r.URL.Path, "/"+endpoint) {
			n++
		}
	}
	return n
}

func (f *fakeYouTube) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, r)
	f.mu.Unlock()

	if r.Method != http.MethodGet {
		f.serveWrite(w, r)
		return
	}

	q := r.URL.Query()
	var body any

	switch {
	case strings.HasSuffix(r.URL.Path, "/channels"):
		body = youtube.ChannelListResponse{Items: []*youtube.Channel{{Id: testChannel}}}

	case strings.HasSuffix(r.URL.Path, "/videos"):
		// the library repeats id= for each ID; the API also accepts a comma list
		var ids []string
		for _, value := range q["id"] {
			ids = append(ids, strings.Split(value, ",")...)
		}
		if len(ids) > 50 {
			http.Error(w, "too many ids", http.StatusBadRequest)
			return
		}
		resp := youtube.VideoListResponse{}
		f.mu.Lock()
		lagging := f.lagVideoReads > 0
		if lagging {
			f.lagVideoReads--
		}
		f.mu.Unlock()
		if lagging {
			ids = nil
		}
		for _, id := range ids {
			if v, ok := f.videos[id]; ok {
				resp.Items = append(resp.Items, v)
			}
		}
		body = resp

	case strings.HasSuffix(r.URL.Path, "/playlists"):
		var all []*youtube.Playlist
		if id := q.Get("id"); id != "" {
			for _, p := range f.playlists {
				if p.Id == id {
					all = append(all, p)
				}
			}
		} else {
			all = f.playlists
		}
		page, next := paginate(len(all), q)
		body = youtube.PlaylistListResponse{Items: all[page[0]:page[1]], NextPageToken: next}

	case strings.HasSuffix(r.URL.Path, "/playlistItems"):
		all := f.items[q.Get("playlistId")]
		page, next := paginate(len(all), q)
		body = youtube.PlaylistItemListResponse{Items: all[page[0]:page[1]], NextPageToken: next}

	default:
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(body)
}

// paginate serves maxResults at a time, with the next start index as the page token
func paginate(total int, q map[string][]string) ([2]int, string) {
	size := 5
	if values := q["maxResults"]; len(values) > 0 {
		if n, err := strconv.Atoi(values[0]); err == nil {
			size = n
		}
	}
	start := 0
	if values := q["pageToken"]; len(values) > 0 {
		start, _ = strconv.Atoi(values[0])
	}
	end := min(start+size, total)
	next := ""
	if end < total {
		next = strconv.Itoa(end)
	}
	return [2]int{start, end}, next
}

func newTestClient(t *testing.T, f *fakeYouTube) *Client {
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	c, err := New(context.Background(), srv.Client(), option.WithEndpoint(srv.URL+"/"))
	require.NoError(t, err)
	return c
}

func video(id, title string) *youtube.Video {
	return &youtube.Video{
		Id: id,
		Snippet: &youtube.VideoSnippet{
			ChannelId:            testChannel,
			Title:                title,
			Description:          "About " + title,
			CategoryId:           "29",
			Tags:                 []string{"faith"},
			DefaultLanguage:      "en",
			DefaultAudioLanguage: "en-US",
			PublishedAt:          "2026-10-04T15:04:05Z",
			Thumbnails: &youtube.ThumbnailDetails{
				High:   &youtube.Thumbnail{Url: "https://i.ytimg.com/vi/" + id + "/hqdefault.jpg"},
				Maxres: &youtube.Thumbnail{Url: "https://i.ytimg.com/vi/" + id + "/maxresdefault.jpg"},
			},
		},
		Status: &youtube.VideoStatus{
			PrivacyStatus: "unlisted",
			License:       "youtube",
			Embeddable:    true,
			MadeForKids:   false,
		},
	}
}

func playlist(id, title string) *youtube.Playlist {
	return &youtube.Playlist{
		Id:      id,
		Snippet: &youtube.PlaylistSnippet{ChannelId: testChannel, Title: title, Description: "D"},
		Status:  &youtube.PlaylistStatus{PrivacyStatus: "public"},
	}
}

func item(id, videoID, title string, position int64) *youtube.PlaylistItem {
	return &youtube.PlaylistItem{
		Id: id,
		Snippet: &youtube.PlaylistItemSnippet{
			Title:      title,
			Position:   position,
			ResourceId: &youtube.ResourceId{Kind: "youtube#video", VideoId: videoID},
		},
		ContentDetails: &youtube.PlaylistItemContentDetails{VideoId: videoID},
		Status:         &youtube.PlaylistItemStatus{PrivacyStatus: "public"},
	}
}

func TestNew_ReadsChannel(t *testing.T) {
	c := newTestClient(t, newFakeYouTube())
	assert.Equal(t, testChannel, c.ChannelID)
}

func TestGetVideos_Converts(t *testing.T) {
	f := newFakeYouTube()
	f.videos["01LiNfRzMo8"] = video("01LiNfRzMo8", "CORE | Boundaries | Mary Peltz | October 4, 2026")
	c := newTestClient(t, f)

	videos, err := c.GetVideos("01LiNfRzMo8")
	require.NoError(t, err)
	require.Len(t, videos, 1)

	m := videos[0]
	assert.Equal(t, "01LiNfRzMo8", m.ID)
	assert.Equal(t, testChannel, m.ChannelID)
	assert.Equal(t, "CORE | Boundaries | Mary Peltz | October 4, 2026", m.Title)
	assert.Equal(t, "About CORE | Boundaries | Mary Peltz | October 4, 2026", m.Description)
	assert.Equal(t, ytpkg.PrivacyUnlisted, m.Privacy)
	assert.Equal(t, "29", m.CategoryID)
	assert.Equal(t, []string{"faith"}, m.Tags)
	assert.Equal(t, "en", m.DefaultLanguage)
	assert.Equal(t, "en-US", m.DefaultAudioLanguage)
	assert.False(t, m.MadeForKids)
	assert.Equal(t, "youtube", m.License)
	assert.True(t, m.Embeddable)
	assert.Equal(t, 2026, m.PublishedAt.Year())
	assert.Equal(t, "https://i.ytimg.com/vi/01LiNfRzMo8/maxresdefault.jpg", m.ThumbnailURL)
}

func TestLargestThumbnail_FallsBack(t *testing.T) {
	assert.Equal(t, "high", largestThumbnail(&youtube.ThumbnailDetails{
		High:    &youtube.Thumbnail{Url: "high"},
		Default: &youtube.Thumbnail{Url: "default"},
	}))
	assert.Equal(t, "", largestThumbnail(nil))
}

// more than 50 IDs go in several calls, and the result keeps the order asked for
func TestGetVideos_BatchesOf50(t *testing.T) {
	f := newFakeYouTube()
	var ids []string
	for i := 0; i < 120; i++ {
		id := fmt.Sprintf("vid%08d", i)
		ids = append(ids, id)
		f.videos[id] = video(id, "T")
	}
	ids = append(ids, "missingXXXX")
	c := newTestClient(t, f)

	videos, err := c.GetVideos(ids...)
	require.NoError(t, err)
	assert.Len(t, videos, 120, "the missing video is absent")
	assert.Equal(t, 3, f.count("videos"))
	for i, v := range videos {
		assert.Equal(t, ids[i], v.ID)
	}
}

func TestGetPlaylist_AllPages(t *testing.T) {
	f := newFakeYouTube()
	f.playlists = []*youtube.Playlist{playlist("PL1", "Prayer")}
	for i := 0; i < 120; i++ {
		f.items["PL1"] = append(f.items["PL1"],
			item(fmt.Sprintf("item%d", i), fmt.Sprintf("vid%08d", i), "T", int64(i)))
	}
	c := newTestClient(t, f)

	series, err := c.GetPlaylist("PL1")
	require.NoError(t, err)
	assert.Equal(t, "Prayer", series.Title)
	assert.Equal(t, ytpkg.PrivacyPublic, series.Privacy)
	require.Len(t, series.Items, 120)
	assert.Equal(t, 3, f.count("playlistItems"))
	assert.Equal(t, ytpkg.PlaylistItem{
		ItemID: "item7", VideoID: "vid00000007", Title: "T", Privacy: ytpkg.PrivacyPublic, Position: 7,
	}, series.Items[7])
}

func TestGetPlaylist_NotFound(t *testing.T) {
	c := newTestClient(t, newFakeYouTube())
	_, err := c.GetPlaylist("PLnope")
	assert.ErrorContains(t, err, "not found")
}

func TestFindPlaylists(t *testing.T) {
	f := newFakeYouTube()
	for i := 0; i < 60; i++ {
		f.playlists = append(f.playlists, playlist(fmt.Sprintf("PLx%d", i), fmt.Sprintf("Filler %d", i)))
	}
	f.playlists = append(f.playlists,
		playlist("PL1", "CORE | Healthy Limits"),
		playlist("PL2", "CORE: Healthy Limits"),         // normalises equal
		playlist("PL3", "CORE | Healthy Limits (2025)"), // contains
		playlist("PL4", "Prayer"),
	)
	c := newTestClient(t, f)

	exact, near, err := c.FindPlaylists("CORE | Healthy Limits")
	require.NoError(t, err)
	require.Len(t, exact, 1)
	assert.Equal(t, "PL1", exact[0].ID)

	var nearIDs []string
	for _, p := range near {
		nearIDs = append(nearIDs, p.ID)
	}
	assert.Equal(t, []string{"PL2", "PL3"}, nearIDs)

	// the channel's playlists are listed once per run, across pages
	_, _, err = c.FindPlaylists("Prayer")
	require.NoError(t, err)
	assert.Equal(t, 2, f.count("playlists"), "64 playlists is two pages, read once")
}

func TestPlaylistsContaining(t *testing.T) {
	f := newFakeYouTube()
	f.playlists = []*youtube.Playlist{playlist("PL1", "Prayer"), playlist("PL2", "Women of Faith")}
	f.items["PL1"] = []*youtube.PlaylistItem{item("a", "other000000", "O", 0), item("b", "01LiNfRzMo8", "T", 1)}
	f.items["PL2"] = []*youtube.PlaylistItem{item("c", "01LiNfRzMo8", "T", 0)}
	c := newTestClient(t, f)

	refs, err := c.PlaylistsContaining("01LiNfRzMo8")
	require.NoError(t, err)
	assert.Equal(t, []ytpkg.PlaylistRef{
		{Title: "Prayer", ID: "PL1", Position: 2},
		{Title: "Women of Faith", ID: "PL2", Position: 1},
	}, refs)
}

// +---------------------------------------------------------------------------
// | Playlist snapshot
// +---------------------------------------------------------------------------

// a fake channel with many playlists, some with more than a page of items, one empty
func manyPlaylists() *fakeYouTube {
	f := newFakeYouTube()
	for p := 0; p < 30; p++ {
		id := fmt.Sprintf("PL%02d", p)
		f.playlists = append(f.playlists, playlist(id, fmt.Sprintf("Series %d", p)))
		count := p * 4 // PL00 is empty; PL13 and up need a second page, PL26 and up a third
		for i := 0; i < count; i++ {
			f.items[id] = append(f.items[id],
				item(fmt.Sprintf("%s-%d", id, i), fmt.Sprintf("v%02d%08d", p, i), "T", int64(i)))
		}
	}
	f.items["PL07"] = append(f.items["PL07"], item("x", "01LiNfRzMo8", "Mine", 28))
	return f
}

func TestSnapshotPlaylists_IsComplete(t *testing.T) {
	f := manyPlaylists()
	c := newTestClient(t, f)

	snapshot, err := c.SnapshotPlaylists()
	require.NoError(t, err)
	assert.Equal(t, testChannel, snapshot.ChannelID)
	require.Len(t, snapshot.Playlists, 30)
	for p, series := range snapshot.Playlists {
		assert.Equal(t, fmt.Sprintf("PL%02d", p), series.ID, "the channel's order is kept")
		want := len(f.items[series.ID])
		assert.Len(t, series.Items, want, "playlist %s", series.ID)
	}

	// every playlist read exactly once: one page each, a second for 17, a third for 4
	assert.Equal(t, 30+17+4, f.count("playlistItems"))

	// and nothing read again for the lookup that follows
	refs, err := c.PlaylistsContaining("01LiNfRzMo8")
	require.NoError(t, err)
	assert.Equal(t, []ytpkg.PlaylistRef{{Title: "Series 7", ID: "PL07", Position: 29}}, refs)
	assert.Equal(t, 30+17+4, f.count("playlistItems"))
}

// a saved snapshot replaces the scan entirely, and survives a JSON round trip,
// including a playlist with no items
func TestUseSnapshot_MakesNoRequests(t *testing.T) {
	first := newTestClient(t, manyPlaylists())
	snapshot, err := first.SnapshotPlaylists()
	require.NoError(t, err)

	data, err := json.Marshal(snapshot)
	require.NoError(t, err)
	var loaded PlaylistSnapshot
	require.NoError(t, json.Unmarshal(data, &loaded))

	f := manyPlaylists()
	c := newTestClient(t, f)
	require.NoError(t, c.UseSnapshot(&loaded))

	refs, err := c.PlaylistsContaining("01LiNfRzMo8")
	require.NoError(t, err)
	assert.Equal(t, []ytpkg.PlaylistRef{{Title: "Series 7", ID: "PL07", Position: 29}}, refs)

	exact, _, err := c.FindPlaylists("Series 0")
	require.NoError(t, err)
	require.Len(t, exact, 1)
	assert.Empty(t, exact[0].Items, "the per-run list carries no items")

	assert.Equal(t, 0, f.count("playlists"))
	assert.Equal(t, 0, f.count("playlistItems"))
}

func TestUseSnapshot_RefusesAnotherChannel(t *testing.T) {
	c := newTestClient(t, newFakeYouTube())
	err := c.UseSnapshot(&PlaylistSnapshot{ChannelID: "UCother"})
	assert.ErrorContains(t, err, "UCother")
}
