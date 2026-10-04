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
//
// TypeIsHint records where Type came from, which decides how far it is trusted. See
// matchesType.
type MessageLookup struct {
	Date       DateOnly
	Type       MessageType
	Track      int  // 0 means "not supplied"
	TypeIsHint bool // Type was inferred from the file name rather than asserted
}

// matchesType reports whether a catalog message's type satisfies the lookup.
//
// An asserted type - one the operator supplied with --type - is taken literally. An
// inferred one is trusted only as far as the file name can actually go: the name
// distinguishes a prayer from everything else and nothing more, so an inferred
// "message" means "not a prayer" rather than the literal Message type.
//
// Reading an inferred type literally was a bug. A service that also produced a
// training, word, testimony, song or special event left date-plus-type matching
// exactly one row, so the lookup believed it had resolved the video and never offered
// the operator a choice - silently returning the wrong row. Widening only happened
// when the type matched nothing at all, which is not this case.
func (l MessageLookup) matchesType(msgType MessageType) bool {
	if !l.TypeIsHint {
		return msgType == l.Type
	}
	if l.Type == Prayer {
		return msgType == Prayer
	}
	return msgType != Prayer
}

// DescribeType renders the type for a message to the operator, making clear when it is
// only a hint and therefore covers more than the one type named.
func (l MessageLookup) DescribeType() string {
	if l.TypeIsHint && l.Type != Prayer {
		return string(l.Type) + " (or similar)"
	}
	return string(l.Type)
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
		fmt.Fprintf(&b, "no message on %s has track %d. candidates are:",
			e.Lookup.Date.String(), e.Lookup.Track)
	} else {
		fmt.Fprintf(&b, "%d messages on %s could be this video:",
			len(e.Candidates), e.Lookup.Date.String())
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
	// the type is shown only when it is something other than an ordinary message,
	// because that is the whole reason two rows on one date look alike: a training or a
	// word reads exactly like a message once the name is all you have to go on
	if m.Type != "" && m.Type != Message && m.Type != UnknownType {
		desc += fmt.Sprintf("  <%s>", string(m.Type))
	}
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
// Date is the reliable key; type only narrows, and how far it narrows depends on
// whether it was asserted or inferred - see MessageLookup.matchesType. An inferred
// "message" keeps every non-prayer row on the date as a candidate, because the file
// name cannot tell a "training", "word", "testimony", "song" or "special-event" from a
// message and those are roughly 400 of the catalog's messages.
//
// Should the type match nothing at all, it is discarded as a bad hint and the search
// widens to the date alone rather than reporting the message missing.
//
// Returns *NotFoundLookupError if the date matches nothing at all, or
// *AmbiguousLookupError if several messages match and the track number does not
// resolve it. It never guesses between candidates.
func (c *Catalog) FindMessage(lookup MessageLookup) (*CatalogMessage, *CatalogSeri, error) {
	// everything on this date, whatever its type
	var onDate []CatalogMessage
	for index := range c.Messages {
		if c.Messages[index].Date.Time.Equal(lookup.Date.Time) {
			onDate = append(onDate, c.Messages[index])
		}
	}
	if len(onDate) == 0 {
		return nil, nil, &NotFoundLookupError{Lookup: lookup}
	}

	// narrow by type where that leaves anything
	candidates := onDate
	var ofType []CatalogMessage
	for index := range onDate {
		if lookup.matchesType(onDate[index].Type) {
			ofType = append(ofType, onDate[index])
		}
	}
	if len(ofType) > 0 {
		candidates = ofType
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
