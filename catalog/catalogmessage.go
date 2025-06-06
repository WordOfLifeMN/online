package catalog

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// CatalogMessage describes one message. The message may be part of a series or not. A message
// is one media event (audio + video recording). A message may contain information linking it to
// a series, but the message just has that metadata (like series name and index), and it is up
// to an external process if that information will be used to assemble related messages into a
// series
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
	Audio       *OnlineResource   `json:"audio,omitempty"`       // URL of the audio file
	Video       *OnlineResource   `json:"video,omitempty"`       // URL of the video. normally on YouTube, BitChute, Rumble, or S3
	Resources   []OnlineResource  `json:"resources,omitempty"`   // list of online resources for this message (links, docs, video, etc)
	initialized bool              `json:"-"`                     // has this object been initialized?
}

// +---------------------------------------------------------------------------
// | Constructors
// +---------------------------------------------------------------------------

func (m *CatalogMessage) Copy() CatalogMessage {
	// make a shallow copy
	msg := *m

	// make the deep copies
	// NOTE: this creates new arrays, but not new objects in the arrays
	copy(msg.Speakers, m.Speakers)
	copy(msg.Series, m.Series)
	copy(msg.Playlist, m.Playlist)
	copy(msg.Resources, m.Resources)

	return msg
}

// initialize prepares the message for use. Performs the following checks:
//   - If the audio/video URL isn't a URL, then deletes it (assumes it was one of the statuses,
//     like "in progress", "rendering", etc)
//   - If the speakers are one of the well-known ones (Vern, Mary, Dave), then make sure the name
//     is correct
func (m *CatalogMessage) Initialize() error {
	if m.initialized {
		return nil
	}
	defer func() { m.initialized = true }()

	// clean audio
	if m.Audio != nil && !strings.Contains(m.Audio.URL, "://") {
		m.Audio = nil
	}

	// clean video
	if m.Video != nil && !strings.Contains(m.Video.URL, "://") {
		m.Video = nil
	}

	// clean the names
	for index, speaker := range m.Speakers {
		m.Speakers[index] = m.normalizeSpeakerName(speaker)
	}

	return nil
}

// normalizeSpeakerName standardizes the speaker name abbreviations and adds titles where
// appropriate (this is mostly a convenience function so I don't have to type full titles and
// names in the spreadsheet)
func (m *CatalogMessage) normalizeSpeakerName(speaker string) string {
	speaker = strings.TrimSpace(speaker)

	switch strings.ToLower(speaker) {
	case "vp", "vern", "vern peltz", "pastor vern peltz", "pastor vern":
		speaker = "Pastor Vern Peltz"
	case "dw", "dave", "dave warren", "pastor dave", "pastor warren", "warren":
		speaker = "Pastor Dave Warren"
	case "ji", "jim", "jim isakson", "isakson":
		speaker = "Pastor Jim Isakson"
	case "ik", "igor", "igor kondratyuk", "pastor igor kondratyuk", "pastor igor", "pastor kondratyuk", "kondratyuk":
		speaker = "Pastor Igor Kondratyuk"
	case "tk", "tania", "tania kondratyuk":
		speaker = "Tania Kondratyuk"
	case "mp", "mary", "mary peltz", "pastor mary peltz", "pastor mary":
		speaker = "Pastor Mary Peltz"
		if m.Ministry == CenterOfRelationshipExperience {
			speaker = "Mary Peltz"
		}
	}

	return speaker
}

// +---------------------------------------------------------------------------
// | Accessors
// +---------------------------------------------------------------------------

func (m *CatalogMessage) DateString() string {
	if m.Date.IsZero() {
		return ""
	}

	if m.Date.After(time.Now()) {
		return "Scheduled for " + m.Date.Time.Format("Jan 2, 2006")
	}

	return m.Date.Time.Format("Jan 2, 2006")
}

// SpeakerString gets all the speakers in a descriptive string
func (m *CatalogMessage) SpeakerString() string {
	return strings.Join(m.Speakers, ", ")
}

func (m *CatalogMessage) HasAudio() bool {
	return m != nil && m.Audio != nil && strings.Contains(m.Audio.URL, "://")
}

func (m *CatalogMessage) HasVideo() bool {
	return m != nil && m.Video != nil && strings.Contains(m.Video.URL, "://")
}

// GetAudioSize gets the size of the audio file in bytes. Returns -1 on error,
// or 0 if no audio URL. Note this makes network calls to get the content size
func (m *CatalogMessage) GetAudioSize() int {
	m.Initialize()
	if m.Audio == nil {
		return 0
	}

	resp, err := http.Head(m.Audio.URL)
	if err != nil {
		log.Printf("WARNING: Could not get file size of %s: %s", m.Audio.URL, err.Error())
		return -1
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("WARNING: Unsuccessful status code getting file size of %s: %d", m.Audio.URL, resp.StatusCode)
		return -1
	}

	length, err := strconv.Atoi(resp.Header.Get("Content-Length"))
	if err != nil {
		log.Printf("WARNING: Could not parse the file size '%s': %s", resp.Header.Get("Content-Length"), err.Error())
		return -1
	}

	return length
}

// +---------------------------------------------------------------------------
// | Queries
// +---------------------------------------------------------------------------

// Determines if the message is in the specified series
func (m *CatalogMessage) IsInSeries(seriesName string) bool {
	return m.FindSeriesReference(seriesName) != nil
}

// FindSeriesReference returns this messages reference to the specfied series. Return nil if
// this message not in the series
func (m *CatalogMessage) FindSeriesReference(seriesName string) *SeriesReference {
	seriesNameUC := strings.ToUpper(seriesName)
	for _, ref := range m.Series {
		if strings.ToUpper(ref.Name) == seriesNameUC {
			return &ref
		}
	}
	return nil
}
