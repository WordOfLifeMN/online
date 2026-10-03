package catalog

import (
	"fmt"
	"strings"
)

// MessageLookup identifies a single message in the catalog.
//
// Date and Type - not speaker - are the key. A single service commonly produces both a
// Prayer and a Message on the same date, frequently from the same speaker, so speaker
// does not discriminate between them.
//
// Track is an optional tie-breaker. It is ignored when Date and Type already identify
// one message, and required when they do not.
type MessageLookup struct {
	Date  DateOnly
	Type  MessageType
	Track int // 0 means "not supplied"
}

// AmbiguousLookupError reports that a lookup matched more than one message and
// nothing available resolved it. It carries the candidates so the caller can ask the
// operator to choose.
//
// Track number alone is not enough in practice. Measured against the live catalog for
// 2024 onward, 60 (date, type) groups are ambiguous and only 21 of them have distinct
// non-zero tracks. The rest are things like a message and its Q&A session, or several
// candidate interviews on one date, which all carry track 0 because they are not in a
// series. That is why the caller is expected to resolve this interactively.
type AmbiguousLookupError struct {
	Lookup     MessageLookup
	Candidates []CatalogMessage
}

func (e *AmbiguousLookupError) Error() string {
	var b strings.Builder

	if e.Lookup.Track > 0 {
		fmt.Fprintf(&b, "no message on %s of type %q has track %d. candidates are:",
			e.Lookup.Date.String(), string(e.Lookup.Type), e.Lookup.Track)
	} else {
		fmt.Fprintf(&b, "%d messages on %s are of type %q:",
			len(e.Candidates), e.Lookup.Date.String(), string(e.Lookup.Type))
	}

	for _, msg := range e.Candidates {
		fmt.Fprintf(&b, "\n  %s", msg.DescribeForChoice())
	}

	return b.String()
}

// DescribeForChoice renders a message as one line of a disambiguation list
func (m *CatalogMessage) DescribeForChoice() string {
	track := 0
	if len(m.Series) > 0 {
		track = m.Series[0].Index
	}

	desc := m.Name
	if len(m.Series) > 0 && m.Series[0].Name != "" {
		desc += fmt.Sprintf("  [%s", m.Series[0].Name)
		if track > 0 {
			desc += fmt.Sprintf(", track %d", track)
		}
		desc += "]"
	}
	if speakers := m.SpeakerString(); speakers != "" {
		desc += "  (" + speakers + ")"
	}

	return desc
}

// NotFoundLookupError reports that nothing in the catalog matched the lookup. This is
// not fatal to the publishing pipeline - the generated title and summary are still
// useful - so it reports what was searched for rather than just failing.
type NotFoundLookupError struct {
	Lookup MessageLookup
}

func (e *NotFoundLookupError) Error() string {
	return fmt.Sprintf("no message found dated %s with type %q",
		e.Lookup.Date.String(), string(e.Lookup.Type))
}

// FindMessage locates the single message matching the lookup, together with the series
// it belongs to (nil if it is not in one).
//
// Returns *NotFoundLookupError if nothing matches, or *AmbiguousLookupError if several
// do and the track number does not resolve it. It never guesses between candidates.
func (c *Catalog) FindMessage(lookup MessageLookup) (*CatalogMessage, *CatalogSeri, error) {
	// collect everything matching date and type
	var candidates []CatalogMessage
	for index := range c.Messages {
		msg := c.Messages[index]
		if !msg.Date.Time.Equal(lookup.Date.Time) {
			continue
		}
		if msg.Type != lookup.Type {
			continue
		}
		candidates = append(candidates, msg)
	}

	switch len(candidates) {
	case 0:
		return nil, nil, &NotFoundLookupError{Lookup: lookup}
	case 1:
		// date and type were enough. a supplied track number is redundant here, so it
		// is ignored rather than used to reject the one obvious match.
		return c.withSeries(&candidates[0])
	}

	// more than one candidate, so we need the track number to tell them apart
	if lookup.Track == 0 {
		return nil, nil, &AmbiguousLookupError{Lookup: lookup, Candidates: candidates}
	}

	for index := range candidates {
		msg := candidates[index]
		if len(msg.Series) > 0 && msg.Series[0].Index == lookup.Track {
			return c.withSeries(&candidates[index])
		}
	}

	// a track was supplied but matched none of the candidates
	return nil, nil, &AmbiguousLookupError{Lookup: lookup, Candidates: candidates}
}

// withSeries pairs a message with the series it belongs to, if any
func (c *Catalog) withSeries(msg *CatalogMessage) (*CatalogMessage, *CatalogSeri, error) {
	return msg, c.FindSeriesForMessage(msg), nil
}

// FindSeriesForMessage returns the series a message belongs to, or nil if it is not in
// one. Exported so that a caller which resolved an AmbiguousLookupError by asking the
// operator can pair their choice with its series.
func (c *Catalog) FindSeriesForMessage(msg *CatalogMessage) *CatalogSeri {
	if msg == nil || len(msg.Series) == 0 {
		return nil
	}

	seri, ok := c.FindSeriByName(msg.Series[0].Name)
	if !ok {
		// the message names a series the catalog doesn't define. that is a catalog
		// consistency problem for 'check' to report, not a reason to fail publishing.
		return nil
	}

	return seri
}
