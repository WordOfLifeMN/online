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

// AmbiguousLookupError reports that a lookup matched more than one message and no
// track number resolved it. It lists the candidates so the operator can pick one.
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
		fmt.Fprintf(&b, "%d messages on %s are of type %q. re-run with a track number to say which:",
			len(e.Candidates), e.Lookup.Date.String(), string(e.Lookup.Type))
	}

	for _, msg := range e.Candidates {
		track := 0
		if len(msg.Series) > 0 {
			track = msg.Series[0].Index
		}
		fmt.Fprintf(&b, "\n  track %d: %s", track, msg.Name)
	}

	return b.String()
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
	if len(msg.Series) == 0 {
		return msg, nil, nil
	}

	seri, ok := c.FindSeriByName(msg.Series[0].Name)
	if !ok {
		// the message names a series the catalog doesn't define. that is a catalog
		// consistency problem for 'check' to report, not a reason to fail publishing.
		return msg, nil, nil
	}

	return msg, seri, nil
}
