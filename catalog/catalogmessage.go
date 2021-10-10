package catalog

import (
	"log"
	"net/http"
	"strconv"
	"strings"
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
	Audio       string            `json:"audio,omitempty"`       // URL of the audio file
	Video       string            `json:"video,omitempty"`       // URL of the video. normally on YouTube, BitChute, Rumble, or S3
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
//  - If the audio/video URL isn't a URL, then deletes it (assumes it was one of the statuses,
//    like "in progress", "rendering", etc)
//  - If the speakers are one of the well-known ones (Vern, Mary, Dave), then make sure the name
//    is correct
func (m *CatalogMessage) Initialize() error {
	if m.initialized {
		return nil
	}
	defer func() { m.initialized = true }()

	// clean audio
	if !strings.Contains(m.Audio, "://") {
		m.Audio = ""
	}

	// clean video
	if !strings.Contains(m.Video, "://") {
		m.Video = ""
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
	case "vp", "vern", "vern peltz":
		speaker = "Pastor Vern Peltz"
	case "dw", "dave", "dave warren":
		speaker = "Pastor Dave Warren"
	case "mp", "mary", "mary peltz", "pastor mary peltz":
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

// GetAudioURL gets the URL for the audio version of this file. Returns "" if
// there is no audio for this message
func (m *CatalogMessage) GetAudioURL() string {
	if strings.Contains(m.Audio, "://") {
		return m.Audio
	}
	return ""
}

// GetAudioSize gets the size of the audio file in bytes. Returns -1 on error,
// or 0 if no audio URL. Note this makes network calls to get the content size
func (m *CatalogMessage) GetAudioSize() int {
	audioURL := m.GetAudioURL()
	if audioURL == "" {
		return 0
	}

	resp, err := http.Head(audioURL)
	if err != nil {
		log.Printf("WARNING: Could not get file size of %s: %s", audioURL, err.Error())
		return -1
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("WARNING: Unsuccessful status code getting file size of %s: %d", audioURL, resp.StatusCode)
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
