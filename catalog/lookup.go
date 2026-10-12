package catalog

import (
	"fmt"
	"strings"
)

// MessageLookup identifies a single message in the catalog.
//
// Date is the key, and the only one. Neither speaker nor type discriminates reliably:
// a service commonly produces several rows on one date, frequently from the same
// speaker, and the file name cannot name the type of any of them.
//
// Type was once part of the key, inferred from a "p" next to the date. It was removed
// because the inference was wrong often enough to matter and the failure was silent.
// A video whose row the sheet calls a prayer, named without the marker, was not merely
// left ambiguous - the type filter removed the correct row and returned the only
// survivor as a confident answer. The file name's title is far better evidence than its
// markers, and that is now what distinguishes the rows, by ranking rather than
// filtering: ranking cannot eliminate the right answer, it can only order it badly.
//
// Track is an optional tie-breaker, the one way to resolve a date without being asked.
type MessageLookup struct {
	Date  DateOnly
	Track int // 0 means "not supplied"
}

// AmbiguousLookupError reports that a lookup matched more than one message and
// nothing available resolved it. It carries the candidates so the caller can ask the
// operator to choose.
//
// Track number alone is not enough in practice. Measured against the live catalog for
// 2024 onward, 60 groups sharing a date are ambiguous and only 21 of them have distinct
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
	return fmt.Sprintf("no message found dated %s", e.Lookup.Date.String())
}

// FindMessage locates the single message matching the lookup, together with the series
// it belongs to (nil if it is not in one).
//
// Date is the whole key. Every row on the date is a candidate, and where there is more
// than one the caller is expected to ask rather than narrow: nothing else available
// here is reliable enough to eliminate a row, and eliminating the right one is a
// failure the operator never sees. See MessageLookup.
//
// Returns *NotFoundLookupError if the date matches nothing, or *AmbiguousLookupError if
// several messages match and the track number does not resolve it. It never guesses
// between candidates.
func (c *Catalog) FindMessage(lookup MessageLookup) (*CatalogMessage, *CatalogSeri, error) {
	// everything on this date, whatever its type
	var candidates []CatalogMessage
	for index := range c.Messages {
		if c.Messages[index].Date.Time.Equal(lookup.Date.Time) {
			candidates = append(candidates, c.Messages[index])
		}
	}

	switch len(candidates) {
	case 0:
		return nil, nil, &NotFoundLookupError{Lookup: lookup}
	case 1:
		// the date was enough. a supplied track number is redundant here, so it is
		// ignored rather than used to reject the one obvious match.
		return c.withSeries(&candidates[0])
	}

	// more than one row on the date. an explicit track is the operator answering the
	// question in advance; without one, the question gets asked.
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
