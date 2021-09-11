package catalog

import (
	"time"
)

// Describes the catalog model.
//
// In this package "seri" is the singlular form of "series", just so we can
// avoid the name collision of a "series" is a list of "series" (since "series"
// is both singular and plural)

// Catalog contains all information about online content
type Catalog struct {
	Created time.Time `json:"created,omitempty"`

	Series   []CatalogSeri    `json:"series,omitempty"`   // series defined in the online content
	Messages []CatalogMessage `json:"messages,omitempty"` // messages defined in the online content

	messageSeries []CatalogSeri `json:"-"` // series generated from single messages
	allSeries     []CatalogSeri `json:"-"` // all the series, both from online content, and generated from single messages
}

// CatalogSeri describes one series in a catalog. A Series describes a
// collection of related messages. This could be a collection of one or more
// messages.
//
// IsRaw determines if the data is raw from the online content source. In this view,
type CatalogSeri struct {
	ID          string           `json:"id"`                    // web- and file-safe ID
	Name        string           `json:"name"`                  // display name
	Description string           `json:"description,omitempty"` // detailed description of contents of series
	Booklets    []OnlineResource `json:"booklets,omitempty"`    // list of study booklets for this series (pdf)
	Resources   []OnlineResource `json:"resource,omitempty"`    // any other online resources (links, docs, youtube, etc)
	Visibility  View             `json:"visibility"`            // visibility of this series as a whole
	Jacket      string           `json:"jacket,omitempty"`      // link to the DVD (or CD) jacket for this series
	Thumbnail   string           `json:"thumbnail,omitempty"`   // link to the thumbnail to use for the series
	StartDate   DateOnly         `json:"start-date,omitempty"`  // date of first message in the series
	EndDate     DateOnly         `json:"end-date,omitempty"`    // date of last message in the series

	// cached or generated data. note that this data could be customized for
	// different views of the series. when read from the online content, the
	// view is "Raw" and the list of messages could contain messages with
	// different visibilities, however, after calling GetView(), the returned
	// series view is not "Raw" and the rest of the data will only include
	// information consistent with the view
	View     View             `json:"-"` // view of this cached data, "Raw" if unfiltered yet
	messages []CatalogMessage `json:"-"` // list of messages in the series
	speakers []string         `json:"-"` // list of speakers in the series
}

// CatalogMessage describes one message. The message may be part of a series or
// not. A message is one media event (audio + video recording). A message may
// contain information linking it to a series, but the message just has that
// metadata (like series name and index), and it is up to an external process if
// that information will be used to assemble related messages into a series
type CatalogMessage struct {
	Date        DateOnly          `json:"date"`                  // date message was given/recorded (required)
	Name        string            `json:"name"`                  // name of the message (required)
	Description string            `json:"description,omitempty"` // detailed description of this message
	Speakers    []string          `json:"speakers"`              // names of significant speakers in the message, typically in order they spoke
	Ministry    Ministry          `json:"ministry"`              // which ministry this message was presented for
	Type        MessageType       `json:"type"`                  // category of this message
	Visibility  View              `json:"visibility,omitempty"`  // visibility of this message
	Series      []SeriesReference `json:"series,omitempty"`      // which series this message belongs to
	Playlist    []string          `json:"playlist,omitempty"`    // playlist(s) this message is in. used to generate podcasts
	Audio       string            `json:"audio,omitempty"`       // URL of the audio file
	Video       string            `json:"video,omitempty"`       // URL of the video. normally on YouTube, BitChute, Rumble, or S3
	Resources   []OnlineResource  `json:"resources,omitempty"`   // list of online resources for this message (links, docs, video, etc)
}
