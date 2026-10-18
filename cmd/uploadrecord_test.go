package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeVideo(t *testing.T, dir, name, content string) string {
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestUploadRecord_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	recordPath := filepath.Join(dir, "state", "youtube-uploads.json")
	video := writeVideo(t, dir, "2026-10-04 Pay Attention.mp4", "not really a video")

	record, err := loadUploadRecord(recordPath)
	require.NoError(t, err, "a missing record is an empty one")
	id, err := record.find(video)
	require.NoError(t, err)
	assert.Empty(t, id)

	require.NoError(t, record.add(recordPath, video, "newvid00001", "Pay Attention | October 4, 2026"))

	loaded, err := loadUploadRecord(recordPath)
	require.NoError(t, err)
	id, err = loaded.find(video)
	require.NoError(t, err)
	assert.Equal(t, "newvid00001", id)

	for _, entry := range loaded.Uploads {
		assert.Equal(t, int64(len("not really a video")), entry.Size)
		assert.Equal(t, "Pay Attention | October 4, 2026", entry.Title)
		assert.False(t, entry.UploadedAt.IsZero())
	}
}

// a re-exported file of a different size is a different upload
func TestUploadRecord_SizeIsPartOfTheKey(t *testing.T) {
	dir := t.TempDir()
	recordPath := filepath.Join(dir, "youtube-uploads.json")
	video := writeVideo(t, dir, "message.mp4", "first cut")

	record, err := loadUploadRecord(recordPath)
	require.NoError(t, err)
	require.NoError(t, record.add(recordPath, video, "newvid00001", "T"))

	require.NoError(t, os.WriteFile(video, []byte("a longer second cut"), 0o644))
	id, err := record.find(video)
	require.NoError(t, err)
	assert.Empty(t, id)
}

// the same file named by a relative path, or in another case on Windows, is the same
// upload
func TestUploadRecord_PathIsNormalised(t *testing.T) {
	dir := t.TempDir()
	recordPath := filepath.Join(dir, "youtube-uploads.json")
	video := writeVideo(t, dir, "Message.mp4", "content")

	record, err := loadUploadRecord(recordPath)
	require.NoError(t, err)
	require.NoError(t, record.add(recordPath, video, "newvid00001", "T"))

	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(wd) })

	id, err := record.find("Message.mp4")
	require.NoError(t, err)
	assert.Equal(t, "newvid00001", id)

	if runtime.GOOS == "windows" {
		id, err = record.find(strings.ToUpper(video))
		require.NoError(t, err)
		assert.Equal(t, "newvid00001", id)
	}
}

func TestUploadRecord_Forget(t *testing.T) {
	dir := t.TempDir()
	recordPath := filepath.Join(dir, "youtube-uploads.json")
	video := writeVideo(t, dir, "message.mp4", "content")

	record, err := loadUploadRecord(recordPath)
	require.NoError(t, err)
	require.NoError(t, record.add(recordPath, video, "deleted0001", "T"))
	require.NoError(t, record.forget(recordPath, "deleted0001"))

	loaded, err := loadUploadRecord(recordPath)
	require.NoError(t, err)
	id, err := loaded.find(video)
	require.NoError(t, err)
	assert.Empty(t, id)
}

func TestUploadRecord_Corrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "youtube-uploads.json")
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o644))

	_, err := loadUploadRecord(path)
	assert.ErrorContains(t, err, "cannot read the upload record")
}
