package catalog

import (
	"log"
	"sort"

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
	Description string           `json:"description,omitempty"` // detailed description of contents of series
	Booklets    []OnlineResource `json:"booklets,omitempty"`    // list of study booklets for this series (pdf)
	Resources   []OnlineResource `json:"resource,omitempty"`    // any other online resources (links, docs, youtube, etc)
	Visibility  View             `json:"visibility"`            // visibility of this series as a whole
	Jacket      string           `json:"jacket,omitempty"`      // link to the DVD (or CD) jacket for this series
	Thumbnail   string           `json:"thumbnail,omitempty"`   // link to the thumbnail to use for the series
	StartDate   DateOnly         `json:"start-date,omitempty"`  // date of first message in the series
	StopDate    DateOnly         `json:"end-date,omitempty"`    // date of last message in the series

	// cached or generated data. note that this data could be customized for
	// different views of the series. when read from the online content, the
	// view is "Raw" and the list of messages could contain messages with
	// different visibilities, however, after calling GetView(), the returned
	// series view is not "Raw" and the rest of the data will only include
	// information consistent with the view
	View        View             `json:"-"` // view of this cached data, "Raw" if unfiltered yet
	messages    []CatalogMessage `json:"-"` // list of messages in the series
	speakers    []string         `json:"-"` // list of speakers in the series
	initialized bool             `json:"-"` // has this object been initialized?
}

// +---------------------------------------------------------------------------
// | Constructors
// +---------------------------------------------------------------------------

// Creates a new series record from a message. This creates a Series that is a Series of the one message
// that was passed in
func NewSeriesFromMessage(msg *CatalogMessage) CatalogSeri {
	seri := CatalogSeri{}
	seri.Name = msg.Name
	seri.Description = msg.Description
	seri.Resources = msg.Resources
	seri.Visibility = msg.Visibility
	seri.StartDate = msg.Date
	seri.StopDate = msg.Date
	seri.messages = []CatalogMessage{*msg}

	seri.ID = "SAM-" + util.ComputeHash(seri.Name)

	seri.Initialize()

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
		log.Printf("Series '%s' has no visibility, defaulting to private", s.Name)
		s.Visibility = Private
	}

	// default the view to the current visibility
	if s.View == "" || s.View == UnknownView {
		s.View = s.Visibility
	}

	return nil
}

// Normalize updates all the series fields to reflect the data in the messages list. This
// includes start and stop dates, speakers, and resources.
func (s *CatalogSeri) Normalize() {
	// fast bail if nothing to do
	if len(s.messages) == 0 {
		return
	}

	// sort by index number (0's at the end)
	sort.SliceStable(s.messages,
		func(i, j int) bool {
			series1 := s.messages[i].Series
			series2 := s.messages[j].Series

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

	// TODO: we need to merge series resources with message resources, but in order to do that,
	// we need a separate list of series resources

	// iterate messages and update fields
	for _, msg := range s.messages {
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
			if !s.IsSpeaker(speaker) {
				s.speakers = append(s.speakers, speaker)
			}
		}

		// update resources
		for _, resource := range msg.Resources {
			found := false
			for _, existing := range s.Resources {
				if existing.URL == resource.URL {
					found = true
					break
				}
			}
			if !found {
				s.Resources = append(s.Resources, resource)
			}
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

	if s.ID == "" {
		// if there is no message, then there is no ID
		if len(s.messages) == 0 {
			log.Printf("WARNING: Tried to generate an ID for series '%s' with no ministry", s.Name)
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
	}

	return s.ID
}

// Gets the Ministry of a series
func (s *CatalogSeri) GetMinistry() Ministry {
	if len(s.messages) == 0 {
		return UnknownMinistry
	}
	return s.messages[0].Ministry
}

// +---------------------------------------------------------------------------
// | Queries
// +---------------------------------------------------------------------------

// IsBooklet determines if this series just represents a booklet. A booklet "series" looks like
// a series, except it has a booklet but no messages or ID. Note that a normal series can also
// have a booklet but a "booklet series" has no messages
func (s *CatalogSeri) IsBooklet() bool {
	return len(s.Booklets) > 0 && len(s.messages) == 0 && s.ID == ""
}

// IsSpeaker reports whether a specific person is a speaker in this series
func (s *CatalogSeri) IsSpeaker(speaker string) bool {
	for _, s := range s.speakers {
		if s == speaker {
			return true
		}
	}
	return false
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
