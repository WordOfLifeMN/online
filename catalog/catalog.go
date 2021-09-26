package catalog

import (
	"sort"
	"strings"
	"time"
)

// Describes the catalog model.
//
// In this package "seri" is the singlular form of "series", just so we can avoid the name
// collision of a "series" is a list of "series" (since "series" is both singular and plural)
// * hello
// * there

// Catalog contains all information about online content
type Catalog struct {
	Created     time.Time        `json:"created,omitempty"`
	Series      []CatalogSeri    `json:"series,omitempty"`   // series defined in the online content
	Messages    []CatalogMessage `json:"messages,omitempty"` // messages defined in the online content
	initialized bool             `json:"-"`                  // true if the catalog has been initialized
}

// +---------------------------------------------------------------------------
// | Constructors
// | Usage: Create a Catalog, check for IsValid(), then Initalize() it
// +---------------------------------------------------------------------------

// initialize prepares the catalog for use. It will
// - add the messages to the series
// - create single-message series for all the standalone messages
func (c *Catalog) Initialize() error {
	if c.initialized {
		return nil
	}
	defer func() { c.initialized = true }()

	// initialize series and messages
	for _, seri := range c.Series {
		seri.Initialize()
	}
	for _, msg := range c.Messages {
		msg.Initialize()
	}

	// add series to the messages that they belong to
	if err := c.addMessagesToTheirSeries(); err != nil {
		return err
	}

	if err := c.createStandAloneMessageSeries(); err != nil {
		return err
	}

	return nil
}

// addMessagesToTheirSeries finds all the messages that belong to each series and adds them to
// the series in track order
func (c *Catalog) addMessagesToTheirSeries() error {
	for index, _ := range c.Series {
		seri := &c.Series[index]

		// skip series that are already set up
		if len(seri.messages) > 0 {
			continue
		}

		seri.messages = c.FindMessagesInSeries(seri.Name)
		seri.Normalize()
	}

	return nil
}

// createStandAloneMessageSeries creates a series entry for every message that isn't already in
// a series. These new series are appended to the Series list. In addition, it also looks for
// messages that are in the `SAM` (Stand Alone Message) "series" and creates a new
// single-message series for those messages
func (c *Catalog) createStandAloneMessageSeries() error {
	// crete
	for _, msg := range c.Messages {
		// if the message isn't in a series, then add one
		if len(msg.Series) == 0 {
			c.Series = append(c.Series, NewSeriesFromMessage(&msg))
			continue
		}

		// if the message is in a "SAM" series, then create one
		for _, ref := range msg.Series {
			if ref.Name == "SAM" {
				c.Series = append(c.Series, NewSeriesFromMessage(&msg))
				break
			}
		}
	}

	return nil
}

// +---------------------------------------------------------------------------
// | Access
// +---------------------------------------------------------------------------

// Finds a series by name and returns it. (nil, false) if not found
func (c *Catalog) FindSeries(targetName string) (seri *CatalogSeri, ok bool) {
	// search for the series
	targetNameUC := strings.ToUpper(targetName)
	for _, seri := range c.Series {
		seri.Initialize()
		if strings.ToUpper(seri.Name) == targetNameUC {
			return &seri, true
		}
	}
	return nil, false
}

// Given a series name, finds all the messages that are in that series. This returns a slice of
// such messages which is a copy of the original messages, except that all the series
// information that not for the requested series has been removed. In other words, the messages
// in the result will only have one Series element and it will be for the series requested. The
// messages will also be order by the series track number ascending but 0's at the end, so like
// 1, 2, 3, 4, 0, 0, 0. This makes no evaluation of "relevance" in that all messages are added
// to the series, even if they don't have a track index or are private
func (c *Catalog) FindMessagesInSeries(seriesName string) []CatalogMessage {
	var msgs []CatalogMessage

	// special cases
	if seriesName == "SAM" {
		// this is a magic cookie series name that indicates this message should be treated as a
		// stand-alone message in addition to being added to another series. so if we see it,
		// then there are no messages for it
		return msgs
	}

	// get all the messages in this series
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

	// sort by index number (note that each message now has exactly one series reference)
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
