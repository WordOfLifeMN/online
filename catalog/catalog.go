package catalog

import (
	"sort"
	"strings"
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

/*
 * Access
 */

// Finds a series by name and returns it. (nil, false) if not found
func (c *Catalog) FindSeries(targetName string) (seri *CatalogSeri, ok bool) {
	// search all series, but fall back to explicit series if unprepared
	corpus := c.allSeries
	if len(corpus) == 0 {
		corpus = c.Series
	}

	// search for the series
	targetNameLC := strings.ToLower(targetName)
	for _, seri := range corpus {
		seri.initialize()
		if seri.nameLC == targetNameLC {
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
