package ytpkg

import (
	"fmt"
	"strings"
	"time"

	"github.com/WordOfLifeMN/online/util/box"
)

// Print writes the package's sections into a box the caller has opened, so that each
// field can be selected and copied in one gesture. Every value that gets pasted into
// YouTube is a copy line: bare, unwrapped and unbordered, so a selection of the line
// contains exactly the value and nothing else. Labels stay inside the border.
func (m *Message) Print(b *box.Box) {
	b.Section("UPLOAD PACKET")
	b.Labeled("Channel : ", string(m.Channel))
	b.Labeled("Privacy : ", string(m.Privacy))
	for _, playlist := range m.Playlists {
		b.Labeled("Playlist: ", playlist.describe())
	}
	if m.ThumbnailPath != "" {
		// the path is pasted into the upload dialog, so it is a copy line too, and a
		// long one would otherwise be wrapped in the middle of a folder name
		b.Line("Thumb   :")
		b.Copy(m.ThumbnailPath)
	}

	// the alternative shares the Titles section: it is the same field, offered twice,
	// and a reader comparing them should not have to look between two sections
	b.Section("Titles")
	b.Line("Title")
	b.Copy(m.Title)
	if m.Alternative != "" {
		b.Line("Alternative Title")
		b.Copy(m.Alternative)
	}

	b.Section("Description")
	b.Copy(m.Description)

	if len(m.Playlists) > 0 {
		b.Section("Playlist")
		for _, playlist := range m.Playlists {
			b.Copy(playlist.Title)
		}
	}
}

// describe renders a playlist membership for a labeled line: its title, and its
// position when it has one
func (p PlaylistRef) describe() string {
	if p.Position > 0 {
		return fmt.Sprintf("%s (position %d)", p.Title, p.Position)
	}
	return p.Title
}

// PrintLive writes everything YouTube holds for a video, for 'youtube get'. The link,
// thumbnail, title and description are copy lines.
//
// Playlist positions count from 1, like tracks. An empty Playlists means the video was
// found in none of the channel's playlists.
func (m *Message) PrintLive(b *box.Box) {
	b.Section("VIDEO")
	b.Labeled("ID        : ", m.ID)
	b.Labeled("Channel ID: ", m.ChannelID)
	b.Labeled("Privacy   : ", string(m.Privacy))
	b.Labeled("Category  : ", m.CategoryID)
	b.Labeled("Language  : ", m.DefaultLanguage)
	b.Labeled("Audio lang: ", m.DefaultAudioLanguage)
	b.Labeled("For kids  : ", yesNo(m.MadeForKids))
	b.Labeled("License   : ", m.License)
	b.Labeled("Embeddable: ", yesNo(m.Embeddable))
	b.Labeled("Published : ", formatPublished(m.PublishedAt))
	b.Labeled("Tags      : ", strings.Join(m.Tags, ", "))
	b.Line("Link      :")
	b.Copy(VideoURL(m.ID))
	if m.ThumbnailURL != "" {
		b.Line("Thumbnail :")
		b.Copy(m.ThumbnailURL)
	}

	b.Section("Title")
	b.Copy(m.Title)

	b.Section("Description")
	b.Copy(m.Description)

	b.Section("Playlists")
	if len(m.Playlists) == 0 {
		b.Line("Not in any of this channel's playlists")
	}
	for _, playlist := range m.Playlists {
		b.Labeled(playlist.ID+"  ", playlist.describe())
	}
}

// Print writes a playlist and its items, for 'youtube get'. Item positions count from
// 1, like tracks.
func (s *Series) Print(b *box.Box) {
	b.Section("PLAYLIST")
	b.Labeled("ID        : ", s.ID)
	b.Labeled("Channel ID: ", s.ChannelID)
	b.Labeled("Privacy   : ", string(s.Privacy))
	b.Labeled("Items     : ", fmt.Sprintf("%d", len(s.Items)))
	b.Line("Link      :")
	b.Copy(PlaylistURL(s.ID))
	if s.ThumbnailURL != "" {
		b.Line("Thumbnail :")
		b.Copy(s.ThumbnailURL)
	}

	b.Section("Title")
	b.Copy(s.Title)

	b.Section("Description")
	b.Copy(s.Description)

	b.Section("Items")
	if len(s.Items) == 0 {
		b.Line("The playlist is empty")
	}
	for _, item := range s.Items {
		b.Labeled(fmt.Sprintf("%3d  %s  %-8s  ", item.Position+1, item.VideoID, item.Privacy), item.Title)
	}
}

// yesNo renders a flag for reading
func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

// formatPublished renders a publish time in local time, or nothing when there is none
func formatPublished(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04")
}
