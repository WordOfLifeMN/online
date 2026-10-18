package ytclient

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WordOfLifeMN/online/ytpkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/youtube/v3"
)

// oneRequestUploads sends uploads in a single request for one test, which is what the
// fake server understands
func oneRequestUploads(t *testing.T) {
	saved := uploadChunkSize
	uploadChunkSize = 0
	t.Cleanup(func() { uploadChunkSize = saved })
}

func uploadPackage() *ytpkg.Message {
	return &ytpkg.Message{
		Title:                "Pay Attention | Mary Peltz | October 4, 2026",
		Description:          "A summary.\n\nNotes\nhttps://x/notes.pdf",
		Privacy:              ytpkg.PrivacyPublic, // the target, set last; not what is uploaded
		CategoryID:           ytpkg.DefaultCategoryID,
		DefaultLanguage:      ytpkg.DefaultLanguage,
		DefaultAudioLanguage: ytpkg.DefaultLanguage,
		MadeForKids:          false,
	}
}

func testVideoFile(t *testing.T) (string, int64) {
	path := filepath.Join(t.TempDir(), "2026-10-04 Pay Attention.mp4")
	content := "pretend this is an hour of video"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path, int64(len(content))
}

func TestUpload(t *testing.T) {
	oneRequestUploads(t)
	f := newFakeYouTube()
	c := newTestClient(t, f)
	path, size := testVideoFile(t)

	var lastSent, lastTotal int64
	id, err := c.Upload(uploadPackage(), path, func(sent, total int64) { lastSent, lastTotal = sent, total })
	require.NoError(t, err)
	assert.NotEmpty(t, id)
	assert.Equal(t, size, lastSent, "the last progress report is the whole file")
	assert.Equal(t, size, lastTotal)

	writes := f.writesTo("videos")
	require.Len(t, writes, 1)
	body := string(writes[0].body)

	// the metadata part of the upload
	assert.Contains(t, body, `"title":"Pay Attention | Mary Peltz | October 4, 2026"`)
	assert.Contains(t, body, `"categoryId":"29"`)
	assert.Contains(t, body, `"defaultLanguage":"en"`)
	assert.Contains(t, body, `"defaultAudioLanguage":"en"`)
	assert.Contains(t, body, `"privacyStatus":"private"`, "always uploaded private")
	assert.NotContains(t, body, `"privacyStatus":"public"`)
	assert.Contains(t, body, `"selfDeclaredMadeForKids":false`, "false must be sent")

	// and the file itself
	assert.Contains(t, body, "pretend this is an hour of video")
	assert.Contains(t, f.lastWriteURL("videos"), "part=snippet")
}

func TestUpload_ValidatesFirst(t *testing.T) {
	oneRequestUploads(t)
	f := newFakeYouTube()
	c := newTestClient(t, f)
	path, _ := testVideoFile(t)

	pkg := uploadPackage()
	pkg.Title = strings.Repeat("x", 101)
	_, err := c.Upload(pkg, path, nil)
	assert.Error(t, err)
	assert.Empty(t, f.writes)
}

func TestUpload_QuotaInPlainWords(t *testing.T) {
	oneRequestUploads(t)
	f := newFakeYouTube()
	f.refuseUpload = "quotaExceeded"
	c := newTestClient(t, f)
	path, _ := testVideoFile(t)

	_, err := c.Upload(uploadPackage(), path, nil)
	assert.ErrorContains(t, err, "daily quota is used up")
	assert.ErrorContains(t, err, "midnight Pacific")
}

// straight after an upload YouTube can say the new video does not exist; the steps
// that follow wait for it rather than failing
func TestUpload_NewVideoNotYetVisible(t *testing.T) {
	oneRequestUploads(t)
	f := newFakeYouTube()
	c := newTestClient(t, f)
	progress := noWaiting(t, c)
	path, _ := testVideoFile(t)

	id, err := c.Upload(uploadPackage(), path, nil)
	require.NoError(t, err)
	f.videos[id] = video(id, "Pay Attention")
	f.lagVideoReads = 2

	thumb := filepath.Join(t.TempDir(), "thumb.jpg")
	require.NoError(t, os.WriteFile(thumb, []byte("jpeg"), 0o644))
	require.NoError(t, c.SetThumbnail(id, thumb))
	assert.Equal(t, 2, strings.Count(progress.String(), "not ready to show the new video"))

	// a video this run did not upload is not waited for
	f.lagVideoReads = 1
	f.videos["other000000"] = video("other000000", "Other")
	err = c.SetThumbnail("other000000", thumb)
	assert.ErrorContains(t, err, "not found")
}

// a new video can also be refused by a playlist for a moment
func TestAddToPlaylist_NewVideoNotYetVisible(t *testing.T) {
	oneRequestUploads(t)
	f := newPlaylistFake()
	c := newTestClient(t, f)
	noWaiting(t, c)
	path, _ := testVideoFile(t)

	id, err := c.Upload(uploadPackage(), path, nil)
	require.NoError(t, err)

	assert.True(t, c.retryableFor(id)(notFound()))
	assert.False(t, c.retryableFor("other000000")(notFound()))
}

func TestRecentUploads(t *testing.T) {
	f := newFakeYouTube()
	uploads := "UU" + strings.TrimPrefix(testChannel, "UC")
	f.items[uploads] = []*youtube.PlaylistItem{
		item("u1", "newest00000", "Pay Attention | Mary Peltz | October 4, 2026", 0),
		item("u2", "older000000", "Older", 1),
	}
	c := newTestClient(t, f)

	recent, err := c.RecentUploads(50)
	require.NoError(t, err)
	require.Len(t, recent, 2)
	assert.Equal(t, "newest00000", recent[0].VideoID)
	assert.Equal(t, "Pay Attention | Mary Peltz | October 4, 2026", recent[0].Title)
}

func notFound() error {
	return fmt.Errorf("context: %w", &googleapi.Error{Code: http.StatusNotFound})
}
