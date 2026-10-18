package ytpkg

// PlaylistItem is one video's entry in a playlist
type PlaylistItem struct {
	ItemID   string  `json:"itemId,omitempty"`
	VideoID  string  `json:"videoId"`
	Title    string  `json:"title,omitempty"`
	Privacy  Privacy `json:"privacy,omitempty"`
	Position int     `json:"position"`
}

// Series is the ytPackage for one playlist
type Series struct {
	ID           string         `json:"id,omitempty"`
	ChannelID    string         `json:"channelId,omitempty"`
	Channel      Channel        `json:"channel,omitempty"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	Privacy      Privacy        `json:"privacy,omitempty"`
	ThumbnailURL string         `json:"thumbnailUrl,omitempty"`
	Items        []PlaylistItem `json:"items,omitempty"`
}
