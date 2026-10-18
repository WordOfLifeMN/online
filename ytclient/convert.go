package ytclient

import (
	"time"

	"github.com/WordOfLifeMN/online/ytpkg"
	"google.golang.org/api/youtube/v3"
)

// messageFromVideo describes a live video as a package
func messageFromVideo(v *youtube.Video) *ytpkg.Message {
	m := &ytpkg.Message{ID: v.Id}

	if s := v.Snippet; s != nil {
		m.ChannelID = s.ChannelId
		m.Title = s.Title
		m.Description = s.Description
		m.CategoryID = s.CategoryId
		m.Tags = s.Tags
		m.DefaultLanguage = s.DefaultLanguage
		m.DefaultAudioLanguage = s.DefaultAudioLanguage
		m.PublishedAt = parseTime(s.PublishedAt)
		m.ThumbnailURL = largestThumbnail(s.Thumbnails)
	}

	if s := v.Status; s != nil {
		m.Privacy = ytpkg.Privacy(s.PrivacyStatus)
		m.MadeForKids = s.MadeForKids
		m.License = s.License
		m.Embeddable = s.Embeddable
	}

	return m
}

// seriesFromPlaylist describes a live playlist as a package, without its items
func seriesFromPlaylist(p *youtube.Playlist) *ytpkg.Series {
	s := &ytpkg.Series{ID: p.Id}

	if sn := p.Snippet; sn != nil {
		s.ChannelID = sn.ChannelId
		s.Title = sn.Title
		s.Description = sn.Description
		s.ThumbnailURL = largestThumbnail(sn.Thumbnails)
	}
	if st := p.Status; st != nil {
		s.Privacy = ytpkg.Privacy(st.PrivacyStatus)
	}

	return s
}

// itemFromPlaylistItem describes one entry of a playlist. Its position is YouTube's,
// which counts from zero.
func itemFromPlaylistItem(i *youtube.PlaylistItem) ytpkg.PlaylistItem {
	item := ytpkg.PlaylistItem{ItemID: i.Id}

	if s := i.Snippet; s != nil {
		item.Title = s.Title
		item.Position = int(s.Position)
		if s.ResourceId != nil {
			item.VideoID = s.ResourceId.VideoId
		}
	}
	if c := i.ContentDetails; c != nil && c.VideoId != "" {
		item.VideoID = c.VideoId
	}
	if s := i.Status; s != nil {
		item.Privacy = ytpkg.Privacy(s.PrivacyStatus)
	}

	return item
}

// largestThumbnail picks the biggest thumbnail YouTube has made: maxres when the
// upload was large enough for one, otherwise the next size down
func largestThumbnail(t *youtube.ThumbnailDetails) string {
	if t == nil {
		return ""
	}
	for _, thumb := range []*youtube.Thumbnail{t.Maxres, t.Standard, t.High, t.Medium, t.Default} {
		if thumb != nil && thumb.Url != "" {
			return thumb.Url
		}
	}
	return ""
}

// parseTime reads an RFC 3339 time from the API, or zero when there is none
func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// applyManagedFields changes the fields this tool manages on a live video, and nothing
// else. It is the only place a live resource is changed before being written back.
//
// The Go client leaves false and empty values out of a request, and YouTube resets a
// property an update leaves out - embeddable would quietly become true again. So the
// booleans are always sent, whatever their value.
func applyManagedFields(v *youtube.Video, want *ytpkg.Message) {
	if v.Snippet == nil {
		v.Snippet = &youtube.VideoSnippet{}
	}
	if v.Status == nil {
		v.Status = &youtube.VideoStatus{}
	}

	v.Snippet.Title = want.Title
	v.Snippet.Description = want.Description
	v.Snippet.CategoryId = want.CategoryID
	v.Snippet.DefaultLanguage = want.DefaultLanguage
	v.Snippet.DefaultAudioLanguage = want.DefaultAudioLanguage

	if want.Privacy != "" {
		v.Status.PrivacyStatus = string(want.Privacy)
	}
	// a scheduled publish time is only allowed on a private video
	if v.Status.PrivacyStatus != string(ytpkg.PrivacyPrivate) {
		v.Status.PublishAt = ""
	}

	// madeForKids is YouTube's verdict and cannot be written; the setting is the
	// self-declaration
	v.Status.SelfDeclaredMadeForKids = want.MadeForKids

	v.Status.ForceSendFields = append(v.Status.ForceSendFields,
		"Embeddable", "PublicStatsViewable", "SelfDeclaredMadeForKids")
}
