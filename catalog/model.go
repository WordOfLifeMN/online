package catalog

import "time"

// Describes the catalog model.
//
// In this package "seri" is the singlular form of "series", just so we can
// avoid the name collision of a "series" is a list of "series" (since "series"
// is both singular and plural)

// visibility of series and messages
type View string

const (
	Raw      View = "raw"      // undetermined or unedited
	Public   View = "public"   // available to anyone
	Partners View = "partners" // available to covenant partners
	Private  View = "private"  // not to be displayed online to anyone
)

// ministries
type Ministry string

const (
	WordOfLife                     Ministry = "wol"
	CenterOfRelationshipExperience Ministry = "core"
	TheBridgeOutreach              Ministry = "tbo"
	AskThePastor                   Ministry = "ask-pastor"
	FaithAndFreedom                Ministry = "faith-freedom"
)

// message types
type MessageType string

const (
	Message      MessageType = "message"       // a teaching or preached message
	Prayer       MessageType = "prayer"        // prayer for someone or something
	Song         MessageType = "song"          // song
	SpecialEvent MessageType = "special-event" // wedding, funeral, child-dedication, etc
	Testimony    MessageType = "testimony"     // someone testifying about something God has done
	Training     MessageType = "training"      // general leadership training or specific ministry training
	Word         MessageType = "word"          // a prophesy, encouragment, or other utterance under the Holy Spirit
)

// Catalog contains all information about online content
type Catalog struct {
	// series defined in the online content
	Series []CatalogSeri `json:"series,omitempty"`
	// messages defined in the online content
	Messages []CatalogMessage `json:"messages,omitempty"`

	// series generated from single messages
	messageSeries []CatalogSeri `json:"-"`
	// all the series, both from online content, and generated from single messages
	allSeries []CatalogSeri `json:"-"`
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

	// cached or generated data. note that this data could be customized for
	// different views of the series. when read from the online content, the
	// view is "Raw" and the list of messages could contain messages with
	// different visibilities, however, after calling GetView(), the returned
	// series view is not "Raw" and the rest of the data will only include
	// information consistent with the view
	View      View             `json:"-"` // view of this cached data, "Raw" if unfiltered yet
	messages  []CatalogMessage `json:"-"` // list of messages in the series
	speakers  []string         `json:"-"` // list of speakers in the series
	startDate time.Time        `json:"-"` // date of first message in the series
	endDate   time.Time        `json:"-"` // date of last message in the series
}

// CatalogMessage describes one message. The message may be part of a series or
// not. A message is one media event (audio + video recording). A message may
// contain information linking it to a series, but the message just has that
// metadata (like series name and index), and it is up to an external process if
// that information will be used to assemble related messages into a series
type CatalogMessage struct {
	Date        time.Time         `json:"date"`                  // date message was given/recorded (required)
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

// SeriesReference contains information that a message stores about the series
// it belongs to. This information may or may not be accurate or meaningful, it
// is just data without meaning until an external process decides to give it
// meaning
type SeriesReference struct {
	// name of a series. it is a warning if this series isn't defined, but not
	// an error. however, if the series doesn't exist, then this message may not
	// be displayed
	Name string `json:"name"`
	// index within a series. same as a track on a CD. the first index in a
	// series is "1".
	//
	// index "0" means it is part of the series but hidden. this is not
	// recommended as you should use the "View" to mark a message private if it
	// shouldn't be visible in a series, but sometimes a message is in multiple
	// series, and it should be private in one and public in another
	Index int `json:"index,omitempty"`
}

// OnlineResource describes a resource (pdf, document, video, website, etc)
// online that is used as reference material for a series or message
type OnlineResource struct {
	// URL of the resource, required
	URL string `json:"url"`
	// name of the resource, optional. if undefined, then GetDisplayName() will
	// generate one from the URL
	Name string `json:"name,omitempty"`

	// cached or generated data

	// URL of small thumbnail for the resource, optional. this is generated
	// dynamically for the current context of the resource display
	thumbnail string `json:"-"`
	// short string to clarify the type of the resource. generated dynamically to help the user
	// identify what will happen when they click on it, like "video", "pdf"
	classifier string `json:"-"`
}
