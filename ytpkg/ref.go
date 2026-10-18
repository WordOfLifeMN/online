package ytpkg

import (
	"regexp"
	"strings"
)

// videoIDPattern is a YouTube video ID, which is always eleven characters
var videoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// videoLinkPattern finds the video ID in every link form found in the sheet:
//
//	https://youtu.be/ID
//	https://youtu.beID                      (the slash lost when the row was typed)
//	https://www.youtube.com/watch?v=ID
//	https://www.youtube.com/watch?feature=x&v=ID&list=...
//	https://youtube.com/live/ID?feature=share
//	https://www.youtube.com/shorts/ID
//
// The ID is taken as the eleven characters after the marker, whatever follows them,
// because the sheet has rows where a second link was pasted on without a space.
var videoLinkPattern = regexp.MustCompile(
	`(?i)(?:youtu\.be/?|youtube\.com/(?:watch\?(?:[^#\s]*&)?v=|live/|shorts/|embed/))([A-Za-z0-9_-]{11})`)

// playlistIDPattern is a playlist ID: a short type prefix and a long random part.
// Uploads (UU), favourites and the like share the form.
var playlistIDPattern = regexp.MustCompile(`^(?:PL|UU|LL|FL|OL|RD)[A-Za-z0-9_-]{10,}$`)

// playlistLinkPattern finds the list= parameter of a playlist or watch link
var playlistLinkPattern = regexp.MustCompile(`(?i)youtube\.com/[^#\s]*[?&]list=([A-Za-z0-9_-]+)`)

// ParseVideoRef extracts a video ID from a bare ID or a YouTube link. A watch link that
// also names a playlist means the video. Anything else, including Rumble and S3 links,
// is not a video reference.
func ParseVideoRef(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if videoIDPattern.MatchString(s) {
		return s, true
	}
	if match := videoLinkPattern.FindStringSubmatch(s); match != nil {
		return match[1], true
	}
	return "", false
}

// ParsePlaylistRef extracts a playlist ID from a bare ID, a playlist link, or the list=
// parameter of a watch link
func ParsePlaylistRef(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if playlistIDPattern.MatchString(s) {
		return s, true
	}
	if match := playlistLinkPattern.FindStringSubmatch(s); match != nil {
		return match[1], true
	}
	return "", false
}

// VideoURL is the short link to a video, as the sheet stores it
func VideoURL(id string) string {
	return "https://youtu.be/" + id
}

// PlaylistURL is the public link to a playlist
func PlaylistURL(id string) string {
	return "https://www.youtube.com/playlist?list=" + id
}
