package ytpkg

import "time"

// PlaylistRef is a video's membership of one playlist
type PlaylistRef struct {
	Title    string `json:"title"`
	ID       string `json:"id,omitempty"`       // empty until looked up or created
	Position int    `json:"position,omitempty"` // the track; 0 means no position (Ask the Pastor)
}

// Message is the ytPackage for one video: the union of what YouTube tracks and what this
// tool generates. Built from the sheet it describes what the video should be; read from
// YouTube it describes what the video is.
type Message struct {
	// YouTube's side
	ID                   string        `json:"id,omitempty"`
	ChannelID            string        `json:"channelId,omitempty"`
	Title                string        `json:"title"`
	Description          string        `json:"description"`
	Privacy              Privacy       `json:"privacy,omitempty"`
	CategoryID           string        `json:"categoryId,omitempty"`
	Tags                 []string      `json:"tags,omitempty"`
	DefaultLanguage      string        `json:"defaultLanguage,omitempty"`
	DefaultAudioLanguage string        `json:"defaultAudioLanguage,omitempty"`
	MadeForKids          bool          `json:"madeForKids"`
	License              string        `json:"license,omitempty"`
	Embeddable           bool          `json:"embeddable"`
	PublishedAt          time.Time     `json:"publishedAt,omitzero"`
	ThumbnailURL         string        `json:"thumbnailUrl,omitempty"` // live, largest available
	Playlists            []PlaylistRef `json:"playlists,omitempty"`

	// this tool's side
	Channel       Channel `json:"channel,omitempty"`
	Alternative   string  `json:"alternativeTitle,omitempty"` // the generated title, when it differs from Title
	ThumbnailPath string  `json:"thumbnailPath,omitempty"`    // local file to upload
}
