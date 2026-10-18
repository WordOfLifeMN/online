package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/WordOfLifeMN/online/util"
	"github.com/WordOfLifeMN/online/util/cache"
)

// uploadRecordName is the local record of every video this tool has uploaded, kept
// beside the YouTube tokens
const uploadRecordName = "youtube-uploads.json"

// uploadRecordPath is where the record of uploads is kept
func uploadRecordPath() string {
	return filepath.Join(util.NormalizePath(youtubeTokenDir), uploadRecordName)
}

// uploadEntry is one upload: which file, and the video it became
type uploadEntry struct {
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	VideoID    string    `json:"videoId"`
	Title      string    `json:"title"`
	UploadedAt time.Time `json:"uploadedAt"`
}

// uploadRecord maps a video file to the video it was uploaded as. It is the first of
// the two guards against uploading one file twice: the spreadsheet is not written to,
// so until the new link is pasted into its Video column, this is the only memory that
// the upload happened.
type uploadRecord struct {
	Uploads map[string]uploadEntry `json:"uploads"`
}

// uploadKey identifies a file by its absolute path and size, so that a re-edited
// export saved over the old one is a different file
func uploadKey(path string, size int64) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	if runtime.GOOS == "windows" {
		// the same file can be named in any case
		abs = strings.ToLower(abs)
	}
	return fmt.Sprintf("%s|%d", abs, size), nil
}

// loadUploadRecord reads the record of uploads. A missing record is an empty one.
func loadUploadRecord(path string) (*uploadRecord, error) {
	record := &uploadRecord{Uploads: map[string]uploadEntry{}}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return record, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read the upload record %s: %w", path, err)
	}
	if err := json.Unmarshal(data, record); err != nil {
		return nil, fmt.Errorf("cannot read the upload record %s: %w", path, err)
	}
	if record.Uploads == nil {
		record.Uploads = map[string]uploadEntry{}
	}
	return record, nil
}

// find returns the video a file was uploaded as, or "" if it was not
func (r *uploadRecord) find(videoPath string) (string, error) {
	info, err := os.Stat(videoPath)
	if err != nil {
		return "", err
	}
	key, err := uploadKey(videoPath, info.Size())
	if err != nil {
		return "", err
	}
	return r.Uploads[key].VideoID, nil
}

// add records an upload and saves the record at once, so that a run that fails at
// any later step still remembers it
func (r *uploadRecord) add(path string, videoPath, videoID, title string) error {
	info, err := os.Stat(videoPath)
	if err != nil {
		return err
	}
	key, err := uploadKey(videoPath, info.Size())
	if err != nil {
		return err
	}

	abs, _ := filepath.Abs(videoPath)
	r.Uploads[key] = uploadEntry{
		Path:       abs,
		Size:       info.Size(),
		VideoID:    videoID,
		Title:      title,
		UploadedAt: time.Now().UTC(),
	}
	return cache.Write(path, r)
}

// forget drops an upload whose video no longer exists on YouTube, so it no longer
// stands in the way of uploading the file again
func (r *uploadRecord) forget(path string, videoID string) error {
	for key, entry := range r.Uploads {
		if entry.VideoID == videoID {
			delete(r.Uploads, key)
		}
	}
	return cache.Write(path, r)
}
