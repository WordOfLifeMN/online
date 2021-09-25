package catalog

import (
	"log"
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
	nameLC   string           `json:"-"` // lowercase version of the series name (used for searching)
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
	seri.EndDate = msg.Date
	seri.messages = []CatalogMessage{*msg}

	seri.ID = "SAM-" + util.ComputeHash(seri.Name)

	seri.initialize()

	return seri
}

// Initializes the series for use. This assumes that the series was just read in from disk or
// network and will take care of setting up the internal bookkeeping that is necessary for
// performance
func (s *CatalogSeri) initialize() error {
	// use the lower-case name as a flag whether this has been initialized or not
	if s.nameLC != "" {
		return nil
	}

	s.nameLC = strings.ToLower(s.Name)

	// TODO - make sure resources have names?

	return nil
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
