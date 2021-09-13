package catalog

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/WordOfLifeMN/online/util"
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

	initialized   bool          `json:"-"` // true if the catalog has been initialized
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

/*
 * Access
 */

// Finds a series by name and returns it. (nil, false) if not found
func (c *Catalog) FindSeries(seriesName string) (seri *CatalogSeri, ok bool) {
	// search all series, but fall back to explicit series if unprepared
	corpus := c.allSeries
	if len(corpus) == 0 {
		corpus = c.Series
	}

	// search for the series
	for _, seri := range corpus {
		if seri.Name == seriesName {
			return &seri, true
		}
	}
	return nil, false
}

// Given a series name, finds all the messages that are in that series. This
// returns a slice of such messages which is a copy of the original messages,
// except that all the series information that not for the requested series has
// been removed. In other words, the messages in the result will only have one
// Series element and it will be for the series requested. The messages will
// also be order by the series track number ascending but 0's at the end, so
// like 1, 2, 3, 4, 0, 0, 0
func (c *Catalog) FindMessagesInSeries(seriesName string) []CatalogMessage {
	// get all the messages in this series
	var msgs []CatalogMessage
	for _, msg := range c.Messages {
		if msg.IsInSeries(seriesName) {
			msgs = append(msgs, msg)
		}
	}

	// go through each message and remove any other series data
	for index, msg := range msgs {
		if len(msg.Series) == 1 {
			// only one series, nothing to do here
			continue
		}

		// rebuild the Series list with just the one series in it
		ref := msg.FindSeriesReference(seriesName)
		if ref == nil {
			panic("Cannot find series reference? Not possible because we just filtered on it!")
		}
		msgs[index].Series = []SeriesReference{*ref}
	}

	// sort by index number (note that each message now has exactly one series
	// reference)
	sort.SliceStable(msgs,
		func(i, j int) bool {
			index1 := msgs[i].Series[0].Index
			index2 := msgs[j].Series[0].Index

			if index1 == 0 {
				return false
			}
			if index2 == 0 {
				return true
			}
			return index1 < index2
		})

	return msgs
}

/*
 * Validation
 */

// Validates that all aspects of the catalog are valid. Returns true if the
// entire catalog is valid. Returns false and outputs to stderr if any problems
// are found. Note that just because the catalog is not valid does not mean it
// cannot be used.
//
// Validations:
//  - All series referenced in messages exist
//  - Series track indexes start with 1 and are sequential
//  - Series names are unique
//  - Message names are unique
//  - Message names not in a series are unique wrt Series names
func (c *Catalog) IsValid() bool {
	valid := true

	valid = valid && c.IsMessageSeriesValid()
	valid = valid && c.IsMessageSeriesIndexValid()
	valid = valid && c.IsSeriesNamesValid()
	valid = valid && c.IsMessageNamesValid()
	valid = valid && c.IsSeriesAndMessageNamesValid()

	return valid
}

// Validates that all the series referenced by messages actually exist in the
// series records. Any problems will be output to stderr
func (c *Catalog) IsMessageSeriesValid() bool {
	valid := true

	for _, msg := range c.Messages {
		if msg.Series == nil {
			continue
		}

		for _, ref := range msg.Series {
			if _, ok := c.FindSeries(ref.Name); !ok {
				valid = false
				c.printProblem("Message '%s' references series named '%s' which cannot be found\n",
					msg.Name, ref.Name)
			}
		}
	}

	return valid
}

// Validates that all the series referenced by messages have consistent track
// indexes. Checks for duplicate or skipped track numbers. Allows any number of
// track "0" though (which flags a message as part of a series that should not
// be displayed)
func (c *Catalog) IsMessageSeriesIndexValid() bool {
	valid := true

	// check each series
	for _, seri := range c.Series {
		msgs := c.FindMessagesInSeries(seri.Name)

		for index := 0; index < len(msgs)-1; index++ {
			seriesIndex1 := msgs[index].Series[0].Index
			seriesIndex2 := msgs[index+1].Series[0].Index
			if seriesIndex2 == 0 {
				// quit when we encounter the 0's
				break
			}
			if index == 0 && seriesIndex1 != 1 {
				valid = false
				c.printProblem("Series '%s' first message '%s' has index %d\n",
					seri.Name, msgs[index].Name, seriesIndex1)
			}
			if seriesIndex1 == seriesIndex2 {
				valid = false
				c.printProblem("Series '%s' has at least two messages with index %d: '%s' and '%s'\n",
					seri.Name, seriesIndex1, msgs[index].Name, msgs[index+1].Name)
			} else if seriesIndex2 > seriesIndex1+1 {
				valid = false
				c.printProblem("Series '%s' has a gap between indexes %d ('%s') and %d ('%s')\n",
					seri.Name, seriesIndex1, msgs[index].Name, seriesIndex2, msgs[index+1].Name)
			}
		}
	}

	return valid
}

// Verifies that all series names are unique
func (c *Catalog) IsSeriesNamesValid() bool {
	valid := true

	// extract series names
	names := make([]string, len(c.Series))
	for index, seri := range c.Series {
		names[index] = seri.Name
	}
	sort.Strings(names)

	// look for duplicates
	for index := 0; index < len(names)-1; index++ {
		if names[index] == names[index+1] {
			valid = false
			c.printProblem("There are multiple series with the name '%s'", names[index])
		}
	}
	return valid
}

// Verifies that all series names are unique
func (c *Catalog) IsMessageNamesValid() bool {
	valid := true

	// extract message names
	names := make([]string, len(c.Messages))
	for index, msg := range c.Messages {
		names[index] = msg.Name
	}
	sort.Strings(names)

	// look for duplicates
	for index := 0; index < len(names)-1; index++ {
		if names[index] == names[index+1] {
			valid = false
			c.printProblem("There are multiple messages with the name '%s'", names[index])
		}
	}
	return valid
}

// Verifies that all messages (that are not in a series) have names that are unique
func (c *Catalog) IsSeriesAndMessageNamesValid() bool {
	valid := true

	// extract series and message names
	names := []string{}
	for _, seri := range c.Series {
		names = append(names, seri.Name)
	}
	for _, msg := range c.Messages {
		if len(msg.Series) > 0 {
			continue
		}
		names = append(names, msg.Name)
	}
	sort.Strings(names)

	// look for duplicates
	for index := 0; index < len(names)-1; index++ {
		if names[index] == names[index+1] {
			valid = false
			c.printProblem("Message name '%s' conflicts with another message with the same name", names[index])
		}
	}

	return valid
}

// Prints a validation problem to the appropriate output channel
func (c *Catalog) printProblem(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

// ****************************************************************************
// Catalog Seri
// ****************************************************************************

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

	return seri
}

// Gets the ID of a series. If the series has an explicit ID (from the spreadsheet) then it will
// be returned. If the series doesn't have an ID yet, then one will be created from the name.
// Ideally, the ID of a series should be unique and persistent, so this is why we use the ID
// from the spreadsheet first (because it should never change). Generating an ID from the name
// is second-best because it is only persistent unless somone changes the name
func (s *CatalogSeri) GetID() string {
	if s.ID == "" {
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
		}
		s.ID = prefix + util.ComputeHash(s.Name)
	}

	return s.ID
}

// Gets the Ministry of a series
func (s *CatalogSeri) GetMinistry() Ministry {
	if len(s.messages) > 0 {
		return s.messages[0].Ministry
	}
	return UnknownMinistry
}

// ****************************************************************************
// Catalog Message
// ****************************************************************************

// Determines if the message is in the specified series
func (m *CatalogMessage) IsInSeries(seriesName string) bool {
	for _, ref := range m.Series {
		if ref.Name == seriesName {
			return true
		}
	}
	return false
}

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
