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
}

// +---------------------------------------------------------------------------
// | Constructors
// +---------------------------------------------------------------------------

// initialize prepares the message for use. Performs the following checks:
//  - If the audio/video URL isn't a URL, then deletes it (assumes it was one of the statuses,
//    like "in progress", "rendering", etc)
func (m *CatalogMessage) initialize() error {
	// clean audio
	if !strings.Contains(m.Audio, "://") {
		m.Audio = ""
	}

	// clean video
	if !strings.Contains(m.Video, "://") {
		m.Video = ""
	}

	return nil
}

// +---------------------------------------------------------------------------
// | Accessors
// +---------------------------------------------------------------------------

// Gets the series reference from this message for the specified series. nil if
// the message is not in the series
func (m *CatalogMessage) FindSeriesReference(seriesName string) *SeriesReference {
	for _, ref := range m.Series {
		if ref.Name == seriesName {
			return &ref
		}
	}
	return nil
}

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
	for _, ref := range m.Series {
		if ref.Name == seriesName {
			return true
		}
	}
	return false
}
