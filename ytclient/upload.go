package ytclient

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/WordOfLifeMN/online/ytpkg"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/youtube/v3"
)

// uploadChunkSize is how much of a video is sent per request of a resumable upload. A
// failed chunk is resent rather than the whole file. Zero sends the file in one
// request, which is what the tests' fake server understands.
var uploadChunkSize = googleapi.DefaultUploadChunkSize

// Upload sends a video file to the channel and returns the new video's ID.
//
// The video is always uploaded private, whatever m.Privacy says. Its target privacy is
// set last, once the thumbnail and playlists are in place, so a failure part way never
// leaves a half-finished video public. The title, description and the house defaults
// - category, languages, made-for-kids - go up with it.
//
// progress, when given, is called as the upload proceeds, with the bytes sent so far
// and the file's size.
func (c *Client) Upload(m *ytpkg.Message, videoPath string, progress func(sent, total int64)) (string, error) {
	if err := ytpkg.Validate(m.Title, m.Description); err != nil {
		return "", err
	}

	file, err := os.Open(videoPath)
	if err != nil {
		return "", fmt.Errorf("cannot read the video: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("cannot read the video: %w", err)
	}
	size := info.Size()

	video := &youtube.Video{
		Snippet: &youtube.VideoSnippet{
			Title:                m.Title,
			Description:          m.Description,
			CategoryId:           m.CategoryID,
			DefaultLanguage:      m.DefaultLanguage,
			DefaultAudioLanguage: m.DefaultAudioLanguage,
			Tags:                 m.Tags,
		},
		Status: &youtube.VideoStatus{
			PrivacyStatus:           string(ytpkg.PrivacyPrivate),
			SelfDeclaredMadeForKids: m.MadeForKids,
			// false is a real answer to the made-for-kids question, and must be sent
			ForceSendFields: []string{"SelfDeclaredMadeForKids"},
		},
	}

	call := c.svc.Videos.Insert([]string{"snippet", "status"}, video).
		Media(file, googleapi.ChunkSize(uploadChunkSize)).
		Context(c.ctx)
	if progress != nil {
		call = call.ProgressUpdater(func(current, _ int64) { progress(current, size) })
	}

	uploaded, err := call.Do()
	if err != nil {
		return "", fmt.Errorf("cannot upload the video: %w", explainQuota(err))
	}
	if progress != nil {
		progress(size, size)
	}

	c.mu.Lock()
	c.uploaded[uploaded.Id] = true
	c.mu.Unlock()

	return uploaded.Id, nil
}

// RecentUploads lists the channel's most recent uploads, newest first, up to n (at
// most 50). It reads the channel's uploads playlist, whose ID is the channel's with UC
// changed to UU.
func (c *Client) RecentUploads(n int) ([]ytpkg.PlaylistItem, error) {
	if !strings.HasPrefix(c.ChannelID, "UC") {
		return nil, fmt.Errorf("cannot find the uploads of channel %s", c.ChannelID)
	}
	uploads := "UU" + strings.TrimPrefix(c.ChannelID, "UC")

	resp, err := c.svc.PlaylistItems.List([]string{"snippet", "contentDetails", "status"}).
		PlaylistId(uploads).
		MaxResults(int64(min(max(n, 1), maxPerCall))).
		Context(c.ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("cannot read the channel's recent uploads: %w", err)
	}

	items := make([]ytpkg.PlaylistItem, 0, len(resp.Items))
	for _, item := range resp.Items {
		items = append(items, itemFromPlaylistItem(item))
	}
	return items, nil
}

// explainQuota puts the daily quota running out into plain words. An upload costs
// about 1,600 of the project's 10,000 units a day, shared with yt-api.
func explainQuota(err error) error {
	var apiErr *googleapi.Error
	if !errors.As(err, &apiErr) {
		return err
	}
	for _, item := range apiErr.Errors {
		if item.Reason == "quotaExceeded" || item.Reason == "uploadLimitExceeded" {
			return fmt.Errorf("YouTube's daily quota is used up (an upload takes about 1,600 of "+
				"10,000 units, shared with yt-api); it resets at midnight Pacific time: %w", err)
		}
	}
	return err
}
