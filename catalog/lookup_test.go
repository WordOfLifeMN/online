package catalog

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestLookupTestSuite(t *testing.T) {
	suite.Run(t, new(LookupTestSuite))
}

type LookupTestSuite struct {
	suite.Suite
}

// newLookupCatalog builds a catalog with a prayer and a message on the same date, by
// the same speaker - the case that makes speaker useless as a lookup key.
func (t *LookupTestSuite) newLookupCatalog() *Catalog {
	return &Catalog{
		Series: []CatalogSeri{
			{ID: "S1", Name: "SERIES", Visibility: Public},
		},
		Messages: []CatalogMessage{
			{
				Name:       "Opening Prayer",
				Date:       MustParseDateOnly("2026-03-08"),
				Type:       Prayer,
				Speakers:   []string{"Pastor Vern Peltz"},
				Ministry:   WordOfLife,
				Visibility: Public,
			},
			{
				Name:       "Walking in Faith",
				Date:       MustParseDateOnly("2026-03-08"),
				Type:       Message,
				Speakers:   []string{"Pastor Vern Peltz"},
				Ministry:   WordOfLife,
				Visibility: Public,
				Series:     []SeriesReference{{Name: "SERIES", Index: 3}},
			},
		},
	}
}

func (t *LookupTestSuite) TestFindMessage_UniqueOnDateAndType() {
	cat := t.newLookupCatalog()

	msg, seri, err := cat.FindMessage(MessageLookup{
		Date: MustParseDateOnly("2026-03-08"),
		Type: Message,
	})

	t.NoError(err)
	t.Equal("Walking in Faith", msg.Name)
	t.NotNil(seri)
	t.Equal("SERIES", seri.Name)
}

func (t *LookupTestSuite) TestFindMessage_PrayerAndMessageSameDate() {
	// the case speaker cannot disambiguate: same date, same speaker, different type
	cat := t.newLookupCatalog()

	prayer, _, err := cat.FindMessage(MessageLookup{
		Date: MustParseDateOnly("2026-03-08"),
		Type: Prayer,
	})
	t.NoError(err)
	t.Equal("Opening Prayer", prayer.Name)

	message, _, err := cat.FindMessage(MessageLookup{
		Date: MustParseDateOnly("2026-03-08"),
		Type: Message,
	})
	t.NoError(err)
	t.Equal("Walking in Faith", message.Name)
}

func (t *LookupTestSuite) TestFindMessage_NotFound() {
	cat := t.newLookupCatalog()

	_, _, err := cat.FindMessage(MessageLookup{
		Date: MustParseDateOnly("2026-03-15"),
		Type: Message,
	})

	t.Error(err)
	t.IsType(&NotFoundLookupError{}, err)
	t.Contains(err.Error(), "2026-03-15")
}

// +---------------------------------------------------------------------------
// | Track tie-breaker
// +---------------------------------------------------------------------------

// newAmbiguousCatalog builds a catalog where date and type match two messages
func (t *LookupTestSuite) newAmbiguousCatalog() *Catalog {
	return &Catalog{
		Messages: []CatalogMessage{
			{
				Name: "Part One", Date: MustParseDateOnly("2026-03-08"), Type: Message,
				Ministry: WordOfLife, Visibility: Public,
				Series: []SeriesReference{{Name: "SERIES", Index: 4}},
			},
			{
				Name: "Part Two", Date: MustParseDateOnly("2026-03-08"), Type: Message,
				Ministry: WordOfLife, Visibility: Public,
				Series: []SeriesReference{{Name: "SERIES", Index: 5}},
			},
		},
	}
}

func (t *LookupTestSuite) TestTrack_IgnoredWhenUnnecessary() {
	// date and type already identify one message, so a track that does not match it
	// must not cause the obvious match to be rejected
	cat := t.newLookupCatalog()

	msg, _, err := cat.FindMessage(MessageLookup{
		Date:  MustParseDateOnly("2026-03-08"),
		Type:  Message,
		Track: 99,
	})

	t.NoError(err)
	t.Equal("Walking in Faith", msg.Name)
}

func (t *LookupTestSuite) TestTrack_ResolvesAmbiguity() {
	cat := t.newAmbiguousCatalog()

	msg, _, err := cat.FindMessage(MessageLookup{
		Date:  MustParseDateOnly("2026-03-08"),
		Type:  Message,
		Track: 5,
	})

	t.NoError(err)
	t.Equal("Part Two", msg.Name)
}

func (t *LookupTestSuite) TestTrack_AmbiguousWithNoTrack() {
	cat := t.newAmbiguousCatalog()

	_, _, err := cat.FindMessage(MessageLookup{
		Date: MustParseDateOnly("2026-03-08"),
		Type: Message,
	})

	t.Error(err)
	t.IsType(&AmbiguousLookupError{}, err)
	// the operator needs to see the candidates and their tracks to pick one
	t.Contains(err.Error(), "Part One")
	t.Contains(err.Error(), "Part Two")
	t.Contains(err.Error(), "track 4")
	t.Contains(err.Error(), "track 5")
}

func (t *LookupTestSuite) TestTrack_MatchesNothing() {
	cat := t.newAmbiguousCatalog()

	_, _, err := cat.FindMessage(MessageLookup{
		Date:  MustParseDateOnly("2026-03-08"),
		Type:  Message,
		Track: 9,
	})

	t.Error(err)
	t.IsType(&AmbiguousLookupError{}, err)
	// says the supplied track matched nothing, and lists what is available
	t.Contains(err.Error(), "track 9")
	t.Contains(err.Error(), "track 4")
	t.Contains(err.Error(), "track 5")
}

func (t *LookupTestSuite) TestFindMessage_MessageWithNoSeries() {
	cat := &Catalog{
		Messages: []CatalogMessage{
			{
				Name: "Standalone", Date: MustParseDateOnly("2026-03-08"), Type: Message,
				Ministry: WordOfLife, Visibility: Public,
			},
		},
	}

	msg, seri, err := cat.FindMessage(MessageLookup{
		Date: MustParseDateOnly("2026-03-08"),
		Type: Message,
	})

	t.NoError(err)
	t.Equal("Standalone", msg.Name)
	t.Nil(seri)
}
