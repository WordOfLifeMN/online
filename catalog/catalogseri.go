package catalog

import (
	"log"
	"sort"
	"strings"

	"github.com/WordOfLifeMN/online/util"
)

// CatalogSeri describes one series in a catalog. A Series describes a
// collection of related messages. This could be a collection of one or more
// messages.
//
// IsRaw determines if the data is raw from the online content source. In this view,
type CatalogSeri struct {
	ID          string           `json:"id"`                    // web- and file-safe ID
	Name        string           `json:"name"`                  // display name
	StartDate   DateOnly         `json:"start-date,omitempty"`  // date of first message in the series
	StopDate    DateOnly         `json:"end-date,omitempty"`    // date of last message in the series
	Description string           `json:"description,omitempty"` // detailed description of contents of series
	Booklets    []OnlineResource `json:"booklets,omitempty"`    // list of study booklets for this series (pdf)
	Visibility  View             `json:"visibility"`            // visibility of this series as a whole
	Jacket      string           `json:"jacket,omitempty"`      // link to the DVD (or CD) jacket for this series
	Thumbnail   string           `json:"thumbnail,omitempty"`   // link to the thumbnail to use for the series

	// cached or generated data. note that this data could be customized for different views of
	// the series. when read from the online content, the view is "Raw" and the list of messages
	// could contain messages with different visibilities, however, after calling GetView(), the
	// returned series view is not "Raw" and the rest of the data will only include information
	// consistent with the view
	View        View             `json:"-"`                   // view of this cached data, "Raw" if unfiltered yet
	Messages    []CatalogMessage `json:"-"`                   // list of messages in the series
	Speakers    []string         `json:"speakers,omitempty"`  // list of speakers in the series (does not include message speakers)
	Resources   []OnlineResource `json:"resources,omitempty"` // any other online resources (links, docs, youtube, etc) (does not include message resources)
	State       SeriesState      `json:"state,omitempty"`     // is the series in progress?
	initialized bool             `json:"-"`                   // has this object been initialized?
}

type SeriesState int

const (
	State_Unknown       SeriesState = 0
	State_HasNotStarted SeriesState = 1
	State_InProgress    SeriesState = 2
	State_Complete      SeriesState = 3
)

// +---------------------------------------------------------------------------
// | Constructors
// +---------------------------------------------------------------------------

// Creates a new series record from a message. This creates a Series that is a Series of the one message
// that was passed in
func NewSeriesFromMessage(msg *CatalogMessage) CatalogSeri {
	seri := CatalogSeri{}
	seri.Name = msg.Name
	seri.Description = msg.Description
	seri.Visibility = msg.Visibility

	// create a copy of the message for this seri
	message := (*msg).Copy()
	message.Series = []SeriesReference{{Name: seri.Name, Index: 1}}
	seri.Messages = []CatalogMessage{message}

	seri.ID = "SAM-" + util.ComputeHash(seri.Name)

	seri.Initialize()
	seri.Normalize()

	return seri
}

// Copy creates a deep copy of the Series
func (s *CatalogSeri) Copy() CatalogSeri {
	// make the shallow copy
	seri := *s

	// make the deep copy
	// NOTE: this creates new arrays, but not new objects in the arrays
	copy(seri.Speakers, s.Speakers)
	copy(seri.Booklets, s.Booklets)
	copy(seri.Resources, s.Resources)

	seri.Messages = nil
	for _, message := range s.Messages {
		seri.Messages = append(seri.Messages, message.Copy())
	}

	return seri
}

// Initializes the series for use. This assumes that the series was just read in from disk or
// network and will take care of setting up the internal bookkeeping that is necessary for
// performance
func (s *CatalogSeri) Initialize() error {
	if s.initialized {
		return nil
	}
	defer func() { s.initialized = true }()

	// ensure the visibility is valid
	if s.Visibility == "" || s.Visibility == UnknownView {
		// log.Printf("Series '%s' has no visibility, defaulting to private", s.Name)
		s.Visibility = Private
	}

	// default the view to the current visibility
	if s.View == "" || s.View == UnknownView {
		s.View = s.Visibility
	}

	if s.State == State_Unknown {
		s.State = State_HasNotStarted
		if !s.StartDate.IsZero() {
			s.State = State_InProgress
			if !s.StopDate.IsZero() {
				s.State = State_Complete
			}
		}
	}

	return nil
}

// Normalize updates all the series fields to reflect the data in the messages list. This
// includes start and stop dates, speakers, and resources.
func (s *CatalogSeri) Normalize() {
	// fast bail if nothing to do
	if len(s.Messages) == 0 {
		return
	}

	// sort by index number (0's at the end)
	sort.SliceStable(s.Messages,
		func(i, j int) bool {
			series1 := s.Messages[i].Series
			series2 := s.Messages[j].Series

			if len(series1) == 0 || series1[0].Index == 0 {
				return false
			}
			if len(series2) == 0 || series2[0].Index == 0 {
				return true
			}
			return series1[0].Index < series2[0].Index
		})

	// initialize the fields we'll be updating
	s.StartDate = DateOnly{}
	s.StopDate = DateOnly{}
	s.Speakers = nil
	s.Resources = nil

	// iterate messages and update fields
	for _, msg := range s.Messages {
		// set start date
		if s.StartDate.IsZero() || msg.Date.Before(s.StartDate.Time) {
			s.StartDate = msg.Date
		}

		// set stop date
		if s.StopDate.IsZero() || msg.Date.After(s.StopDate.Time) {
			s.StopDate = msg.Date
		}

		// update speakers
		for _, speaker := range msg.Speakers {
			s.AddSpeakerToSeries(speaker)
		}

		// update resources
		for _, resource := range msg.Resources {
			s.AddResourceToSeries(resource)
		}
	}
}

// +---------------------------------------------------------------------------
// | Accessors
// +---------------------------------------------------------------------------

// Gets the ID of a series. If the series has an explicit ID (from the spreadsheet) then it will
// be returned. If the series doesn't have an ID yet, then one will be created from the name.
// Ideally, the ID of a series should be unique and persistent, so this is why we use the ID
// from the spreadsheet first (because it should never change). Generating an ID from the name
// is second-best because it is only persistent unless someone changes the name
func (s *CatalogSeri) GetID() string {
	if s.ID != "" {
		// we already have an ID
		return s.ID
	}

	// if there is no message, then there is no ID
	if len(s.Messages) == 0 {
		log.Printf("WARNING: Tried to generate an ID for series '%s' with no messages", s.Name)
		return ""
	}

	// generate an ID from the name
	prefix := "ID-"
	switch s.GetMinistry() {
	case WordOfLife:
		prefix = "WOLS-"
	case CenterOfRelationshipExperience:
		prefix = "CORE-"
	case AskThePastor:
		prefix = "ATP-"
	case FaithAndFreedom:
		prefix = "FandF-"
	case TheBridgeOutreach:
		prefix = "TBO-"
	}
	s.ID = prefix + util.ComputeHash(s.Name)

	return s.ID
}

// GetViewID gets an ID for the series for a specific view. If the view is public, then the
// mystic ID is the same as the normal ID. However, if the view is partner or private, then the
// mystic ID is additionally hashed to obscure/uniqueify the name
func (s *CatalogSeri) GetViewID(view View) string {
	// get the base ID for a public view
	id := s.GetID()

	// the public view is just the ID
	if view == Public {
		return id
	}

	// all other views have an additional hash
	return id + "-" + util.ComputeHash(id+string(view))
}

// DateString gets the date of the series in a displayable string
func (s *CatalogSeri) DateString() string {
	if s.State == State_Unknown || s.State == State_HasNotStarted {
		return "Coming Soon"
	}

	if s.State == State_InProgress {
		return "Started " + s.StartDate.Time.Format("January 2, 2006")
	}

	if s.StartDate.Year() == s.StopDate.Year() {
		return s.StartDate.Time.Format("January 2") + " - " + s.StopDate.Time.Format("January 2, 2006")
	}

	return s.StartDate.Time.Format("January 2, 2006") + " - " + s.StopDate.Time.Format("January 2, 2006")
}

// Gets the Ministry of a series
func (s *CatalogSeri) GetMinistry() Ministry {
	if len(s.Messages) == 0 {
		return UnknownMinistry
	}
	return s.Messages[0].Ministry
}

// SpeakerString gets the list of speakers as a display string
func (s *CatalogSeri) SpeakerString() string {
	return strings.Join(s.Speakers, ", ")
}

// AddSpeakerToSeries adds a speaker to the list of series and message speakers if they aren't
// already in the list
func (s *CatalogSeri) AddSpeakerToSeries(speaker string) {
	for _, existing := range s.Speakers {
		if existing == speaker {
			return
		}
	}
	s.Speakers = append(s.Speakers, speaker)
}

// AddResourceToSeries adds a resource to the list of series and message resources if it isn't
// already in the list
func (s *CatalogSeri) AddResourceToSeries(resource OnlineResource) {
	for _, existing := range s.Resources {
		if existing.URL == resource.URL {
			return
		}
	}
	s.Resources = append(s.Resources, resource)
}

// +---------------------------------------------------------------------------
// | Queries
// +---------------------------------------------------------------------------

// IsBooklet determines if this series just represents a booklet. A booklet "series" looks like
// a series, except it has a booklet but no messages or ID. Note that a normal series can also
// have a booklet but a "booklet series" has no messages
func (s *CatalogSeri) IsBooklet() bool {
	return len(s.Booklets) > 0 && len(s.Messages) == 0 && s.ID == ""
}

// IsMessageRelevant reports whether the specified message is relavant for this series. In order
// for a message to be relevant, it needs to belong to the series, have a non-zero track number,
// and have a visibility compatible with the series current view
func (s *CatalogSeri) IsMessageRelevant(msg *CatalogMessage) bool {
	// find the series reference for this series
	seriesRef := msg.FindSeriesReference(s.Name)
	if seriesRef == nil {
		return false
	}

	// not relevant if index is 0
	if seriesRef.Index <= 0 {
		return false
	}

	if !IsVisibleInView(msg.Visibility, s.View) {
		return false
	}

	return true
}

// +---------------------------------------------------------------------------
// | Filters
// +---------------------------------------------------------------------------

// FilterSeriesByMinistry takes a slice of series and returns another slice that only contains
// the series that are in the specified ministry. Returns nil slice if none of the series in the
// input slice is in the ministry
func FilterSeriesByMinistry(corpus []CatalogSeri, ministry Ministry) []CatalogSeri {
	var series []CatalogSeri

	for _, seri := range corpus {
		if seri.GetMinistry() == ministry {
			series = append(series, seri)
		}
	}

	return series
}

// FilterSeriesByView takes a slice of series and returns another slice that contains the series
// that are applicable for the view. So a "partner" view can still display series with
// visibility of "public" and "partner" but not "private". The resulting slice will be updated
// so that only messages that match the view are included. In other words, if you ask for a
// "public" view and there is a "public" series with a "private" message, the "private" message
// will be removed from the series before returning
func FilterSeriesByView(corpus []CatalogSeri, view View) []CatalogSeri {
	var series []CatalogSeri

	// TODO - IMHERE

	for _, seri := range corpus {
		// rule out any seri that doesn't have an acceptable visibilty
		if !IsVisibleInView(seri.Visibility, view) {
			continue
		}

		// make a copy of the series with only visible messages
		candidate := seri.Copy()
		candidate.Messages = nil
		for _, msg := range seri.Messages {
			if IsVisibleInView(msg.Visibility, view) {
				candidate.Messages = append(candidate.Messages, msg)
			}
		}

		// if no messages were appropriate then skip it
		if len(candidate.Messages) == 0 {
			continue
		}

		candidate.Normalize()
		candidate.View = view
		series = append(series, candidate)
	}

	return series
}

// FilterSeriesByVisibility takes a slice of series and returns another slice that contains the
// series that have the specific visibility. In other words, if you ask for a "public" view and
// there is a "public" series with a "private" message, the "private" message will be removed
// from the series before returning
func FilterSeriesByVisibility(corpus []CatalogSeri, view View) []CatalogSeri {
	var series []CatalogSeri

	// TODO - implement

	return series
}
