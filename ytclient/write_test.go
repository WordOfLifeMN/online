package ytclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WordOfLifeMN/online/ytpkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/youtube/v3"
)

// serveWrite accepts the write endpoints, records each request, and answers as YouTube
// would
func (f *fakeYouTube) serveWrite(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	f.mu.Lock()
	f.writes = append(f.writes, write{method: r.Method, path: r.URL.Path, body: body})
	n := len(f.writes)
	f.mu.Unlock()

	var resp any
	switch {
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/videos"):
		if f.refuseUpload != "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, `{"error":{"code":403,"message":"Refused.",`+
				`"errors":[{"message":"Refused.","reason":%q}]}}`, f.refuseUpload)
			return
		}
		resp = youtube.Video{Id: fmt.Sprintf("uploaded%03d", n)}

	case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/videos"):
		var v youtube.Video
		json.Unmarshal(body, &v)
		resp = v

	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/playlistItems"):
		var item youtube.PlaylistItem
		json.Unmarshal(body, &item)
		item.Id = fmt.Sprintf("newitem%d", n)

		f.mu.Lock()
		fail := f.failItemInserts > 0
		if fail {
			f.failItemInserts--
			if f.applyFailedInserts {
				id := item.Snippet.PlaylistId
				f.items[id] = append(f.items[id], &item)
			}
		}
		status := f.failStatus
		f.mu.Unlock()

		if fail {
			w.Header().Set("Content-Type", "application/json")
			if status == 0 {
				// what YouTube answers while a new playlist settles
				w.WriteHeader(http.StatusConflict)
				fmt.Fprint(w, `{"error":{"code":409,"message":"The operation was aborted.",`+
					`"errors":[{"message":"","reason":"SERVICE_UNAVAILABLE"}]}}`)
				return
			}
			w.WriteHeader(status)
			fmt.Fprintf(w, `{"error":{"code":%d,"message":"Refused.",`+
				`"errors":[{"message":"Refused.","reason":"forbidden"}]}}`, status)
			return
		}
		resp = item

	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/playlists"):
		var p youtube.Playlist
		json.Unmarshal(body, &p)
		p.Id = fmt.Sprintf("PLnew%d", n)
		p.Snippet.ChannelId = testChannel
		resp = p

	case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/playlistItems"):
		if f.moveRefusal != "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":{"code":400,"message":"Refused: %s.",`+
				`"errors":[{"message":"Refused.","reason":%q}]}}`, f.moveRefusal, f.moveRefusal)
			return
		}
		var item youtube.PlaylistItem
		json.Unmarshal(body, &item)
		resp = item

	case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/playlists"):
		var p youtube.Playlist
		json.Unmarshal(body, &p)
		resp = p

	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/thumbnails/set"):
		resp = youtube.ThumbnailSetResponse{}

	default:
		http.Error(w, "unexpected write "+r.Method+" "+r.URL.Path, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// writesTo lists the recorded writes to an endpoint
func (f *fakeYouTube) writesTo(endpoint string) []write {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []write
	for _, w := range f.writes {
		if strings.HasSuffix(w.path, "/"+endpoint) {
			out = append(out, w)
		}
	}
	return out
}

// lastWriteURL is the full URL of the last write to an endpoint, query included
func (f *fakeYouTube) lastWriteURL(endpoint string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.requests) - 1; i >= 0; i-- {
		r := f.requests[i]
		if r.Method != http.MethodGet && strings.HasSuffix(r.URL.Path, "/"+endpoint) {
			return r.URL.String()
		}
	}
	return ""
}

// a live video carrying every unmanaged field, some of them false
func richVideo(id string) *youtube.Video {
	v := video(id, "Old title")
	v.Snippet.Tags = []string{"faith", "prayer"}
	v.Status.License = "creativeCommon"
	v.Status.Embeddable = false
	v.Status.PublicStatsViewable = false
	v.Status.PrivacyStatus = "private"
	v.Status.PublishAt = "2026-12-24T16:00:00Z"
	return v
}

func wantFor(id string) *ytpkg.Message {
	return &ytpkg.Message{
		ID:                   id,
		Title:                "New title | Mary Peltz | October 4, 2026",
		Description:          "New description.",
		Privacy:              ytpkg.PrivacyUnlisted,
		CategoryID:           "29",
		DefaultLanguage:      "en",
		DefaultAudioLanguage: "en",
		MadeForKids:          false,
	}
}

// decodeBody reads a recorded write as raw JSON, so that a field left out of the
// request shows as missing rather than as its zero value
func decodeBody(t *testing.T, w write) map[string]map[string]any {
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.body, &raw))

	body := map[string]map[string]any{}
	for _, part := range []string{"snippet", "status"} {
		var fields map[string]any
		require.NoError(t, json.Unmarshal(raw[part], &fields))
		body[part] = fields
	}
	return body
}

func TestUpdateVideo_PreservesUnmanagedFields(t *testing.T) {
	f := newFakeYouTube()
	f.videos["01LiNfRzMo8"] = richVideo("01LiNfRzMo8")
	c := newTestClient(t, f)

	require.NoError(t, c.UpdateVideo(wantFor("01LiNfRzMo8")))

	writes := f.writesTo("videos")
	require.Len(t, writes, 1)
	body := decodeBody(t, writes[0])
	snippet, status := body["snippet"], body["status"]

	// managed fields changed
	assert.Equal(t, "New title | Mary Peltz | October 4, 2026", snippet["title"])
	assert.Equal(t, "New description.", snippet["description"])
	assert.Equal(t, "29", snippet["categoryId"])
	assert.Equal(t, "en", snippet["defaultLanguage"])
	assert.Equal(t, "en", snippet["defaultAudioLanguage"])
	assert.Equal(t, "unlisted", status["privacyStatus"])
	assert.Equal(t, false, status["selfDeclaredMadeForKids"])

	// unmanaged fields sent back as they were, the false ones included
	assert.Equal(t, []any{"faith", "prayer"}, snippet["tags"])
	assert.Equal(t, "creativeCommon", status["license"])
	assert.Equal(t, false, status["embeddable"], "a false embeddable must be sent, or it resets")
	assert.Equal(t, false, status["publicStatsViewable"])

	// a schedule is only valid on a private video
	assert.NotContains(t, status, "publishAt")

	// and both parts were declared
	url := f.lastWriteURL("videos")
	assert.Contains(t, url, "part=snippet")
	assert.Contains(t, url, "part=status")
}

func TestUpdateVideo_KeepsScheduleWhilePrivate(t *testing.T) {
	f := newFakeYouTube()
	f.videos["01LiNfRzMo8"] = richVideo("01LiNfRzMo8")
	c := newTestClient(t, f)

	want := wantFor("01LiNfRzMo8")
	want.Privacy = ytpkg.PrivacyPrivate
	require.NoError(t, c.UpdateVideo(want))

	body := decodeBody(t, f.writesTo("videos")[0])
	assert.Equal(t, "2026-12-24T16:00:00Z", body["status"]["publishAt"])
}

func TestUpdateVideo_RefusesAnotherChannel(t *testing.T) {
	f := newFakeYouTube()
	other := richVideo("VXHmO7546ps")
	other.Snippet.ChannelId = "UCtbo"
	f.videos["VXHmO7546ps"] = other
	c := newTestClient(t, f)

	err := c.UpdateVideo(wantFor("VXHmO7546ps"))
	assert.True(t, errors.Is(err, ErrOtherChannel))
	assert.Empty(t, f.writes, "nothing is sent")

	err = c.SetThumbnail("VXHmO7546ps", "unused.jpg")
	assert.True(t, errors.Is(err, ErrOtherChannel))
	assert.Empty(t, f.writes)
}

func TestUpdateVideo_ValidatesFirst(t *testing.T) {
	f := newFakeYouTube()
	f.videos["01LiNfRzMo8"] = richVideo("01LiNfRzMo8")
	c := newTestClient(t, f)

	want := wantFor("01LiNfRzMo8")
	want.Description = "a <b>bold</b> claim"
	assert.Error(t, c.UpdateVideo(want))
	assert.Empty(t, f.writes)
}

func TestUpdateVideo_Missing(t *testing.T) {
	c := newTestClient(t, newFakeYouTube())
	assert.ErrorContains(t, c.UpdateVideo(wantFor("missingXXXX")), "not found")
}

func TestSetThumbnail(t *testing.T) {
	f := newFakeYouTube()
	f.videos["01LiNfRzMo8"] = richVideo("01LiNfRzMo8")
	c := newTestClient(t, f)

	path := filepath.Join(t.TempDir(), "thumb.jpg")
	require.NoError(t, os.WriteFile(path, []byte("\xff\xd8\xff not really a jpeg"), 0o644))

	require.NoError(t, c.SetThumbnail("01LiNfRzMo8", path))
	writes := f.writesTo("thumbnails/set")
	require.Len(t, writes, 1)
	assert.Contains(t, string(writes[0].body), "not really a jpeg")

	assert.ErrorContains(t, c.SetThumbnail("01LiNfRzMo8", filepath.Join(t.TempDir(), "nope.jpg")),
		"cannot read the thumbnail")
}

func TestAddToPlaylist(t *testing.T) {
	f := newFakeYouTube()
	f.playlists = []*youtube.Playlist{playlist("PL1", "Prayer")}
	f.items["PL1"] = []*youtube.PlaylistItem{item("a", "other000000", "O", 0)}
	c := newTestClient(t, f)

	// read first, so the run's view of the playlist has to be kept current
	_, err := c.GetPlaylist("PL1")
	require.NoError(t, err)

	itemID, err := c.AddToPlaylist("PL1", "01LiNfRzMo8")
	require.NoError(t, err)
	assert.NotEmpty(t, itemID)

	var body youtube.PlaylistItem
	require.NoError(t, json.Unmarshal(f.writesTo("playlistItems")[0].body, &body))
	assert.Equal(t, "PL1", body.Snippet.PlaylistId)
	assert.Equal(t, "01LiNfRzMo8", body.Snippet.ResourceId.VideoId)
	assert.Equal(t, "youtube#video", body.Snippet.ResourceId.Kind)

	refs, err := c.PlaylistsContaining("01LiNfRzMo8")
	require.NoError(t, err)
	assert.Equal(t, []ytpkg.PlaylistRef{{Title: "Prayer", ID: "PL1", Position: 2}}, refs,
		"the run sees the addition, at the end")
}

func TestAddToPlaylist_RefusesAnotherChannel(t *testing.T) {
	f := newFakeYouTube()
	theirs := playlist("PLtbo", "TBO")
	theirs.Snippet.ChannelId = "UCtbo"
	f.playlists = []*youtube.Playlist{theirs}
	c := newTestClient(t, f)

	_, err := c.AddToPlaylist("PLtbo", "01LiNfRzMo8")
	assert.True(t, errors.Is(err, ErrOtherChannel))
	assert.Empty(t, f.writes)
}

func TestInsertPlaylist(t *testing.T) {
	f := newFakeYouTube()
	f.playlists = []*youtube.Playlist{playlist("PL1", "Prayer")}
	c := newTestClient(t, f)

	id, err := c.InsertPlaylist(&ytpkg.Series{
		Title:       "CORE | Healthy Limits",
		Description: "About limits.",
		Privacy:     ytpkg.PrivacyUnlisted,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	var body youtube.Playlist
	require.NoError(t, json.Unmarshal(f.writesTo("playlists")[0].body, &body))
	assert.Equal(t, "CORE | Healthy Limits", body.Snippet.Title)
	assert.Equal(t, "About limits.", body.Snippet.Description)
	assert.Equal(t, "unlisted", body.Status.PrivacyStatus)

	// the run finds it from now on, and knows it is empty
	exact, _, err := c.FindPlaylists("CORE | Healthy Limits")
	require.NoError(t, err)
	require.Len(t, exact, 1)
	assert.Equal(t, id, exact[0].ID)
	refs, err := c.PlaylistsContaining("01LiNfRzMo8")
	require.NoError(t, err)
	assert.Empty(t, refs)
}

// YouTube's reads lag its writes, so a playlist read back straight after it is created
// may not be found. The fake never serves created playlists, which is that lag at its
// worst: adding to a playlist this run created must not depend on reading it back.
func TestAddToPlaylist_JustCreated(t *testing.T) {
	f := newFakeYouTube()
	c := newTestClient(t, f)

	id, err := c.InsertPlaylist(&ytpkg.Series{Title: "Voting", Privacy: ytpkg.PrivacyUnlisted})
	require.NoError(t, err)

	_, err = c.AddToPlaylist(id, "-4AH6lVaW38")
	require.NoError(t, err)
	require.Len(t, f.writesTo("playlistItems"), 1)

	// a playlist that already existed is still checked, and one that cannot be found
	// is still refused
	_, err = c.AddToPlaylist("PLmissing", "-4AH6lVaW38")
	assert.ErrorContains(t, err, "not found")
}

func TestInsertPlaylist_Validates(t *testing.T) {
	f := newFakeYouTube()
	c := newTestClient(t, f)

	_, err := c.InsertPlaylist(&ytpkg.Series{Title: ""})
	assert.Error(t, err)
	_, err = c.InsertPlaylist(&ytpkg.Series{Title: strings.Repeat("x", 151)})
	assert.ErrorContains(t, err, "151 characters")
	_, err = c.InsertPlaylist(&ytpkg.Series{Title: "T", Description: "<script>"})
	assert.Error(t, err)
	assert.Empty(t, f.writes)
}

// +---------------------------------------------------------------------------
// | Retrying a playlist that is not ready (Deviation 5)
// +---------------------------------------------------------------------------

// noWaiting makes retries immediate for one test, and returns what the client printed
func noWaiting(t *testing.T, c *Client) *strings.Builder {
	saved := retryDelays
	retryDelays = []time.Duration{0, 0, 0, 0, 0}
	t.Cleanup(func() { retryDelays = saved })

	var progress strings.Builder
	c.Progress = &progress
	return &progress
}

func newPlaylistFake() *fakeYouTube {
	f := newFakeYouTube()
	f.playlists = []*youtube.Playlist{playlist("PL1", "Voting")}
	f.items["PL1"] = nil
	return f
}

func TestAddToPlaylist_RetriesUntilReady(t *testing.T) {
	f := newPlaylistFake()
	f.failItemInserts = 2
	c := newTestClient(t, f)
	progress := noWaiting(t, c)

	itemID, err := c.AddToPlaylist("PL1", "-4AH6lVaW38")
	require.NoError(t, err)
	assert.NotEmpty(t, itemID)
	assert.Len(t, f.writesTo("playlistItems"), 3)
	assert.Equal(t, 2, strings.Count(progress.String(), "retrying in"))
}

func TestAddToPlaylist_GivesUp(t *testing.T) {
	f := newPlaylistFake()
	f.failItemInserts = 100
	c := newTestClient(t, f)
	noWaiting(t, c)

	_, err := c.AddToPlaylist("PL1", "-4AH6lVaW38")
	assert.ErrorContains(t, err, "still not ready after 6 attempts")
	assert.Len(t, f.writesTo("playlistItems"), 6)
}

// an attempt reported as aborted that went through after all is not repeated, so the
// video does not end up in the playlist twice
func TestAddToPlaylist_NoDuplicateWhenAFailureWasApplied(t *testing.T) {
	f := newPlaylistFake()
	f.failItemInserts = 1
	f.applyFailedInserts = true
	c := newTestClient(t, f)
	noWaiting(t, c)

	itemID, err := c.AddToPlaylist("PL1", "-4AH6lVaW38")
	require.NoError(t, err)
	assert.Equal(t, "newitem1", itemID, "the item that was applied")
	assert.Len(t, f.writesTo("playlistItems"), 1, "no second insert")
}

// only YouTube asking to try again is retried; a refusal fails at once
func TestAddToPlaylist_OtherErrorsFailAtOnce(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusBadRequest} {
		f := newPlaylistFake()
		f.failItemInserts = 100
		f.failStatus = status
		c := newTestClient(t, f)
		progress := noWaiting(t, c)

		_, err := c.AddToPlaylist("PL1", "-4AH6lVaW38")
		assert.ErrorContains(t, err, "Refused", "status %d", status)
		assert.Len(t, f.writesTo("playlistItems"), 1, "status %d", status)
		assert.Empty(t, progress.String())
	}
}

func TestRetryable(t *testing.T) {
	assert.True(t, retryable(&googleapi.Error{Code: 409, Message: "The operation was aborted."}))
	assert.True(t, retryable(&googleapi.Error{Code: 409, Errors: []googleapi.ErrorItem{{Reason: "SERVICE_UNAVAILABLE"}}}))
	assert.True(t, retryable(&googleapi.Error{Code: 503}))
	assert.True(t, retryable(&googleapi.Error{Code: 500}))
	assert.True(t, retryable(fmt.Errorf("wrapped: %w", &googleapi.Error{Code: 409, Message: "Aborted"})))
	assert.True(t, retryable(&googleapi.Error{Code: 403, Errors: []googleapi.ErrorItem{{Reason: "backendError"}}}))

	assert.False(t, retryable(&googleapi.Error{Code: 409, Message: "Conflict"}))
	assert.False(t, retryable(&googleapi.Error{Code: 403, Errors: []googleapi.ErrorItem{{Reason: "quotaExceeded"}}}))
	assert.False(t, retryable(&googleapi.Error{Code: 400}))
	assert.False(t, retryable(errors.New("network down")))
	assert.False(t, retryable(nil))
}

// +---------------------------------------------------------------------------
// | Playlist fields and order (Phase 5)
// +---------------------------------------------------------------------------

func TestMoveItem(t *testing.T) {
	f := newPlaylistFake()
	f.items["PL1"] = []*youtube.PlaylistItem{
		item("i-b", "b0000000000", "B", 0),
		item("i-a", "a0000000000", "A", 1),
	}
	c := newTestClient(t, f)
	_, err := c.GetPlaylist("PL1")
	require.NoError(t, err)

	move := ytpkg.Move{ItemID: "i-a", VideoID: "a0000000000", Title: "A", From: 1, To: 0}
	require.NoError(t, c.MoveItem("PL1", move))

	writes := f.writesTo("playlistItems")
	require.Len(t, writes, 1)
	assert.Equal(t, http.MethodPut, writes[0].method)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(writes[0].body, &raw))
	assert.JSONEq(t, `"i-a"`, string(raw["id"]))
	var snippet map[string]any
	require.NoError(t, json.Unmarshal(raw["snippet"], &snippet))
	assert.Equal(t, "PL1", snippet["playlistId"])
	assert.Equal(t, float64(0), snippet["position"], "position 0 must be sent, not left out")
	assert.Equal(t, map[string]any{"kind": "youtube#video", "videoId": "a0000000000"}, snippet["resourceId"])

	// the run's view follows the move
	series, err := c.GetPlaylist("PL1")
	require.NoError(t, err)
	assert.Equal(t, "a0000000000", series.Items[0].VideoID)
	assert.Equal(t, 0, series.Items[0].Position)
	assert.Equal(t, 1, series.Items[1].Position)
}

// a playlist not manually ordered refuses positions, and that is reported as such, at
// once, rather than retried
func TestMoveItem_ManualSortRequired(t *testing.T) {
	f := newPlaylistFake()
	f.moveRefusal = "manualSortRequired"
	c := newTestClient(t, f)
	progress := noWaiting(t, c)

	err := c.MoveItem("PL1", ytpkg.Move{ItemID: "i-a", VideoID: "a0000000000", To: 0})
	assert.True(t, errors.Is(err, ErrManualSortRequired), "got %v", err)
	assert.Len(t, f.writesTo("playlistItems"), 1)
	assert.Empty(t, progress.String())

	// any other refusal is passed on as it is
	f.moveRefusal = "invalidPlaylistItemPosition"
	err = c.MoveItem("PL1", ytpkg.Move{ItemID: "i-a", VideoID: "a0000000000", To: 7})
	assert.Error(t, err)
	assert.False(t, errors.Is(err, ErrManualSortRequired))
}

func TestMoveItem_RefusesAnotherChannel(t *testing.T) {
	f := newFakeYouTube()
	theirs := playlist("PLtbo", "TBO")
	theirs.Snippet.ChannelId = "UCtbo"
	f.playlists = []*youtube.Playlist{theirs}
	c := newTestClient(t, f)

	err := c.MoveItem("PLtbo", ytpkg.Move{ItemID: "x", VideoID: "v", To: 0})
	assert.True(t, errors.Is(err, ErrOtherChannel))
	assert.Empty(t, f.writes)
}

func TestUpdatePlaylist_NeverChangesTitle(t *testing.T) {
	f := newFakeYouTube()
	live := playlist("PL1", "CORE | Healthy Limits")
	live.Snippet.Description = "Old."
	live.Snippet.DefaultLanguage = "en"
	f.playlists = []*youtube.Playlist{live}
	c := newTestClient(t, f)

	require.NoError(t, c.UpdatePlaylist(&ytpkg.Series{
		ID: "PL1", Title: "A Different Title", Description: "", Privacy: ytpkg.PrivacyUnlisted,
	}))

	writes := f.writesTo("playlists")
	require.Len(t, writes, 1)
	body := decodeBody(t, writes[0])
	assert.Equal(t, "CORE | Healthy Limits", body["snippet"]["title"], "the live title is sent back")
	assert.Equal(t, "", body["snippet"]["description"], "an empty description is sent, not left out")
	assert.Equal(t, "en", body["snippet"]["defaultLanguage"])
	assert.Equal(t, "unlisted", body["status"]["privacyStatus"])
}

func TestUpdatePlaylist_RefusesAnotherChannel(t *testing.T) {
	f := newFakeYouTube()
	theirs := playlist("PLtbo", "TBO")
	theirs.Snippet.ChannelId = "UCtbo"
	f.playlists = []*youtube.Playlist{theirs}
	c := newTestClient(t, f)

	err := c.UpdatePlaylist(&ytpkg.Series{ID: "PLtbo", Description: "D"})
	assert.True(t, errors.Is(err, ErrOtherChannel))
	assert.Empty(t, f.writes)
}

func TestApplyMove(t *testing.T) {
	items := []ytpkg.PlaylistItem{
		{ItemID: "a", Position: 0}, {ItemID: "b", Position: 1}, {ItemID: "c", Position: 2},
	}
	moved := applyMove(items, ytpkg.Move{ItemID: "a", From: 0, To: 2})
	assert.Equal(t, []ytpkg.PlaylistItem{
		{ItemID: "b", Position: 0}, {ItemID: "c", Position: 1}, {ItemID: "a", Position: 2},
	}, moved)
	assert.Equal(t, items, applyMove(items, ytpkg.Move{ItemID: "missing", To: 0}))
}
