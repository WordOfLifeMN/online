package ytclient

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"unicode/utf8"

	"github.com/WordOfLifeMN/online/ytpkg"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/youtube/v3"
)

// ErrOtherChannel is returned when a write is asked of a video or playlist that
// belongs to a channel other than the signed-in one. It is checked before anything is
// sent, because a token for one channel can read another's videos but never change
// them, and a write that half-applies is worse than one refused up front.
var ErrOtherChannel = errors.New("belongs to another channel")

// maxPlaylistTitleChars is YouTube's limit on a playlist title
const maxPlaylistTitleChars = 150

// UpdateVideo makes a video's managed fields match want.
//
// It is a read-modify-write: the live video is read again, only the managed fields are
// changed (applyManagedFields), and the whole of snippet and status goes back. YouTube
// clears any mutable property an update leaves out, so anything not sent - tags,
// license, embeddable - would otherwise be lost.
func (c *Client) UpdateVideo(want *ytpkg.Message) error {
	if err := ytpkg.Validate(want.Title, want.Description); err != nil {
		return err
	}

	video, err := c.ownedVideo(want.ID)
	if err != nil {
		return err
	}

	applyManagedFields(video, want)

	_, err = c.svc.Videos.Update([]string{"snippet", "status"}, video).Context(c.ctx).Do()
	if err != nil {
		return fmt.Errorf("cannot update video %s: %w", want.ID, err)
	}
	return nil
}

// SetThumbnail uploads an image as a video's thumbnail
func (c *Client) SetThumbnail(videoID, path string) error {
	if _, err := c.ownedVideo(videoID); err != nil {
		return err
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot read the thumbnail: %w", err)
	}
	defer file.Close()

	if _, err := c.svc.Thumbnails.Set(videoID).Media(file).Context(c.ctx).Do(); err != nil {
		return fmt.Errorf("cannot set the thumbnail of video %s: %w", videoID, err)
	}
	return nil
}

// AddToPlaylist appends a video to the end of a playlist, returning the new item's ID.
// The order is put right afterwards, by the reorder.
func (c *Client) AddToPlaylist(playlistID, videoID string) (string, error) {
	if err := c.checkPlaylistOwned(playlistID); err != nil {
		return "", err
	}

	// a playlist created moments ago refuses items for a few seconds, so the insert is
	// retried. Adding is not idempotent - a playlist can hold a video twice - so each
	// retry first checks whether an attempt reported as failed went through after all.
	var itemID string
	insert := func() error {
		item, err := c.svc.PlaylistItems.Insert([]string{"snippet"}, &youtube.PlaylistItem{
			Snippet: &youtube.PlaylistItemSnippet{
				PlaylistId: playlistID,
				ResourceId: &youtube.ResourceId{Kind: "youtube#video", VideoId: videoID},
			},
		}).Context(c.ctx).Do()
		if err != nil {
			return err
		}
		itemID = item.Id

		// keep this run's view of the playlist current, so a second video added to
		// it, or the reorder, sees this one
		added := itemFromPlaylistItem(item)
		c.mu.Lock()
		if items, ok := c.items[playlistID]; ok {
			added.Position = len(items)
			c.items[playlistID] = append(items, added)
		}
		c.mu.Unlock()
		return nil
	}
	alreadyAdded := func() (bool, error) {
		items, err := c.rereadItems(playlistID)
		if err != nil {
			return false, err
		}
		for _, item := range items {
			if item.VideoID == videoID {
				itemID = item.ItemID
				return true, nil
			}
		}
		return false, nil
	}

	err := c.withRetry("add to the playlist", c.retryableFor(videoID), insert, alreadyAdded)
	if err != nil {
		return "", fmt.Errorf("cannot add video %s to playlist %s: %w", videoID, playlistID, err)
	}
	return itemID, nil
}

// rereadItems reads a playlist's items from YouTube, not from this run's copy, and
// replaces the copy with them
func (c *Client) rereadItems(playlistID string) ([]ytpkg.PlaylistItem, error) {
	c.mu.Lock()
	delete(c.items, playlistID)
	c.mu.Unlock()
	return c.playlistItems(playlistID)
}

// InsertPlaylist creates a playlist on the signed-in channel, returning its ID
func (c *Client) InsertPlaylist(want *ytpkg.Series) (string, error) {
	if want.Title == "" {
		return "", fmt.Errorf("cannot create a playlist without a title")
	}
	if n := utf8.RuneCountInString(want.Title); n > maxPlaylistTitleChars {
		return "", fmt.Errorf("the playlist title is %d characters, over YouTube's limit of %d",
			n, maxPlaylistTitleChars)
	}
	if err := ytpkg.ValidateDescription(want.Description); err != nil {
		return "", err
	}

	// list the channel's playlists before creating one, so that this run's list
	// includes the new playlist exactly once. A list read afterwards might not show it
	// yet: YouTube's reads lag its writes.
	if _, err := c.ownPlaylists(); err != nil {
		return "", err
	}

	privacy := string(want.Privacy)
	if privacy == "" {
		privacy = string(ytpkg.PrivacyPrivate)
	}

	playlist, err := c.svc.Playlists.Insert([]string{"snippet", "status"}, &youtube.Playlist{
		Snippet: &youtube.PlaylistSnippet{
			Title:           want.Title,
			Description:     want.Description,
			DefaultLanguage: ytpkg.DefaultLanguage,
		},
		Status: &youtube.PlaylistStatus{PrivacyStatus: privacy},
	}).Context(c.ctx).Do()
	if err != nil {
		return "", fmt.Errorf("cannot create playlist %q: %w", want.Title, err)
	}

	// the new playlist is this channel's and empty; record it so this run finds it
	created := seriesFromPlaylist(playlist)
	if created.ChannelID == "" {
		created.ChannelID = c.ChannelID
	}
	c.mu.Lock()
	c.playlists = append(c.playlists, created)
	c.items[playlist.Id] = []ytpkg.PlaylistItem{}
	c.created[playlist.Id] = true
	c.mu.Unlock()

	return playlist.Id, nil
}

// ownedVideo reads a video fresh, for a write, and refuses one on another channel.
//
// A video this run uploaded may not be readable for a little while - YouTube's reads
// lag its writes - so for one of those, not finding it is retried.
func (c *Client) ownedVideo(id string) (*youtube.Video, error) {
	var video *youtube.Video
	read := func() error {
		resp, err := c.svc.Videos.List([]string{"snippet", "status"}).Id(id).Context(c.ctx).Do()
		if err != nil {
			return err
		}
		if len(resp.Items) == 0 {
			return &googleapi.Error{Code: http.StatusNotFound, Message: "video " + id + " was not found"}
		}
		video = resp.Items[0]
		return nil
	}
	if err := c.withRetry("show the new video", c.retryableFor(id), read, nil); err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("video %s was not found", id)
		}
		return nil, fmt.Errorf("cannot read video %s: %w", id, err)
	}

	if video.Snippet == nil || video.Snippet.ChannelId != c.ChannelID {
		owner := ""
		if video.Snippet != nil {
			owner = video.Snippet.ChannelId
		}
		return nil, fmt.Errorf("video %s %w (%s), not the signed-in %s",
			id, ErrOtherChannel, owner, c.ChannelID)
	}
	return video, nil
}

// checkPlaylistOwned reads a playlist fresh, for a write, and refuses one on another
// channel.
//
// A playlist this run created is not read: it is this channel's by construction, and
// straight after creating it YouTube can still answer that it does not exist.
func (c *Client) checkPlaylistOwned(id string) error {
	c.mu.Lock()
	created := c.created[id]
	c.mu.Unlock()
	if created {
		return nil
	}

	resp, err := c.svc.Playlists.List([]string{"snippet"}).Id(id).Context(c.ctx).Do()
	if err != nil {
		return fmt.Errorf("cannot read playlist %s: %w", id, err)
	}
	if len(resp.Items) == 0 {
		return fmt.Errorf("playlist %s was not found", id)
	}
	if owner := resp.Items[0].Snippet.ChannelId; owner != c.ChannelID {
		return fmt.Errorf("playlist %s %w (%s), not the signed-in %s",
			id, ErrOtherChannel, owner, c.ChannelID)
	}
	return nil
}

// ErrManualSortRequired is returned when YouTube refuses to position an item because
// the playlist's Video ordering in Studio is not Manual. The API can neither read nor
// change that setting, so the operator has to.
var ErrManualSortRequired = errors.New(`the playlist's Video ordering is not "Manual"`)

// asManualSortRequired turns YouTube's manualSortRequired refusal into
// ErrManualSortRequired, and returns any other error as it is
func asManualSortRequired(err error) error {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		for _, item := range apiErr.Errors {
			if item.Reason == "manualSortRequired" {
				return fmt.Errorf("%w: %s", ErrManualSortRequired, apiErr.Message)
			}
		}
	}
	return err
}

// UpdatePlaylist makes a playlist's description and privacy match want. The title is
// never changed: it is the key the playlist is found by.
//
// As with videos, it is a read-modify-write of the whole snippet and status, so the
// fields not managed here are sent back as they were.
func (c *Client) UpdatePlaylist(want *ytpkg.Series) error {
	if err := ytpkg.ValidateDescription(want.Description); err != nil {
		return err
	}

	resp, err := c.svc.Playlists.List([]string{"snippet", "status"}).Id(want.ID).Context(c.ctx).Do()
	if err != nil {
		return fmt.Errorf("cannot read playlist %s: %w", want.ID, err)
	}
	if len(resp.Items) == 0 {
		return fmt.Errorf("playlist %s was not found", want.ID)
	}
	playlist := resp.Items[0]
	if playlist.Snippet == nil || playlist.Snippet.ChannelId != c.ChannelID {
		return fmt.Errorf("playlist %s %w, not the signed-in %s", want.ID, ErrOtherChannel, c.ChannelID)
	}

	playlist.Snippet.Description = want.Description
	playlist.Snippet.ForceSendFields = append(playlist.Snippet.ForceSendFields, "Description")
	if want.Privacy != "" {
		if playlist.Status == nil {
			playlist.Status = &youtube.PlaylistStatus{}
		}
		playlist.Status.PrivacyStatus = string(want.Privacy)
	}

	_, err = c.svc.Playlists.Update([]string{"snippet", "status"}, playlist).Context(c.ctx).Do()
	if err != nil {
		return fmt.Errorf("cannot update playlist %s: %w", want.ID, err)
	}
	return nil
}

// MoveItem repositions one item of a playlist, as PlanReorder planned it. Returns
// ErrManualSortRequired when the playlist is not manually ordered.
func (c *Client) MoveItem(playlistID string, move ytpkg.Move) error {
	if err := c.checkPlaylistOwned(playlistID); err != nil {
		return err
	}

	update := func() error {
		snippet := &youtube.PlaylistItemSnippet{
			PlaylistId: playlistID,
			ResourceId: &youtube.ResourceId{Kind: "youtube#video", VideoId: move.VideoID},
			Position:   int64(move.To),
			// position 0 is a real position, and must be sent
			ForceSendFields: []string{"Position"},
		}
		_, err := c.svc.PlaylistItems.Update([]string{"snippet"}, &youtube.PlaylistItem{
			Id:      move.ItemID,
			Snippet: snippet,
		}).Context(c.ctx).Do()
		return err
	}

	// a move is idempotent - the same position twice is the same result - so a retry
	// needs no check
	if err := c.withRetry("reorder the playlist", retryable, update, nil); err != nil {
		return fmt.Errorf("cannot move %q in playlist %s: %w", move.Title, playlistID, asManualSortRequired(err))
	}

	// keep this run's view of the playlist in step
	c.mu.Lock()
	if items, ok := c.items[playlistID]; ok {
		c.items[playlistID] = applyMove(items, move)
	}
	c.mu.Unlock()
	return nil
}

// applyMove returns items with one moved as YouTube moves it, positions renumbered
func applyMove(items []ytpkg.PlaylistItem, move ytpkg.Move) []ytpkg.PlaylistItem {
	list := make([]ytpkg.PlaylistItem, 0, len(items))
	var moved *ytpkg.PlaylistItem
	for index := range items {
		if items[index].ItemID == move.ItemID {
			item := items[index]
			moved = &item
			continue
		}
		list = append(list, items[index])
	}
	if moved == nil {
		return items
	}

	to := min(max(move.To, 0), len(list))
	list = append(list[:to], append([]ytpkg.PlaylistItem{*moved}, list[to:]...)...)
	for index := range list {
		list[index].Position = index
	}
	return list
}
