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

// The file name can only tell a prayer from a message. It gives no signal for the
// other types - training, word, testimony, song, special-event - which account for
// roughly 400 messages in the live catalog. When the inferred type matches nothing on
// that date, the type must be discarded as a bad hint rather than the message being
// reported missing.
func (t *LookupTestSuite) TestFindMessage_WrongTypeFallsBackToDate() {
	cat := &Catalog{
		Messages: []CatalogMessage{
			{
				Name: "Leadership Training", Date: MustParseDateOnly("2026-10-02"),
				Type: Training, Speakers: []string{"Terry Francis"},
				Ministry: WordOfLife, Visibility: Public,
			},
		},
	}

	// the file name implies "message", but the row is a "training"
	msg, _, err := cat.FindMessage(MessageLookup{
		Date: MustParseDateOnly("2026-10-02"),
		Type: Message,
	})

	t.NoError(err)
	t.Equal("Leadership Training", msg.Name)
	t.Equal("Terry Francis", msg.SpeakerString())
}

// widening on a bad type hint must not override a good one
func (t *LookupTestSuite) TestFindMessage_GoodTypeHintStillNarrows() {
	cat := t.newLookupCatalog() // a prayer and a message on the same date

	msg, _, err := cat.FindMessage(MessageLookup{
		Date: MustParseDateOnly("2026-03-08"),
		Type: Prayer,
	})

	t.NoError(err)
	t.Equal("Opening Prayer", msg.Name)
}

// newServiceDayCatalog reproduces 2026-10-04 in the live sheet: one service produced a
// prayer, a training and a message, and two of them were filmed. Neither video file
// name can say which of the last two it is.
func (t *LookupTestSuite) newServiceDayCatalog() *Catalog {
	return &Catalog{
		Messages: []CatalogMessage{
			{
				Name: "Pay Attention", Date: MustParseDateOnly("2026-10-04"),
				Type: Prayer, Speakers: []string{"Pastor Vern Peltz"},
				Ministry: WordOfLife, Visibility: Public,
			},
			{
				Name: "The Importance of Voting", Date: MustParseDateOnly("2026-10-04"),
				Type: Training, Speakers: []string{"Pastor Vern Peltz"},
				Ministry: WordOfLife, Visibility: Public,
			},
			{
				Name: "The Works of God", Date: MustParseDateOnly("2026-10-04"),
				Type: Message, Speakers: []string{"Pastor Vern Peltz"},
				Ministry: WordOfLife, Visibility: Public,
			},
		},
	}
}

// An inferred "message" must not silently claim the one row literally typed "message"
// while a training sits beside it. Both are candidates and the operator decides.
func (t *LookupTestSuite) TestFindMessage_InferredTypeDoesNotHideOtherTypes() {
	cat := t.newServiceDayCatalog()

	_, _, err := cat.FindMessage(MessageLookup{
		Date:       MustParseDateOnly("2026-10-04"),
		Type:       Message,
		TypeIsHint: true,
	})

	var ambiguous *AmbiguousLookupError
	t.ErrorAs(err, &ambiguous)
	t.Len(ambiguous.Candidates, 2)

	names := []string{ambiguous.Candidates[0].Name, ambiguous.Candidates[1].Name}
	t.Contains(names, "The Importance of Voting")
	t.Contains(names, "The Works of God")

	// the prayer is never a candidate - that is the one distinction the file name
	// genuinely makes
	t.NotContains(names, "Pay Attention")
}

// an inferred prayer stays exact, so the prayer resolves without a question
func (t *LookupTestSuite) TestFindMessage_InferredPrayerStaysExact() {
	cat := t.newServiceDayCatalog()

	msg, _, err := cat.FindMessage(MessageLookup{
		Date:       MustParseDateOnly("2026-10-04"),
		Type:       Prayer,
		TypeIsHint: true,
	})

	t.NoError(err)
	t.Equal("Pay Attention", msg.Name)
}

// --type is an assertion, so it narrows to exactly what was asked for
func (t *LookupTestSuite) TestFindMessage_AssertedTypeNarrowsExactly() {
	cat := t.newServiceDayCatalog()

	training, _, err := cat.FindMessage(MessageLookup{
		Date: MustParseDateOnly("2026-10-04"),
		Type: Training,
	})
	t.NoError(err)
	t.Equal("The Importance of Voting", training.Name)

	message, _, err := cat.FindMessage(MessageLookup{
		Date: MustParseDateOnly("2026-10-04"),
		Type: Message,
	})
	t.NoError(err)
	t.Equal("The Works of God", message.Name)
}

// a hint that matches nothing still widens to the date rather than reporting the
// message missing - here every row on the date is a prayer
func (t *LookupTestSuite) TestFindMessage_HintMatchingNothingStillWidens() {
	cat := &Catalog{
		Messages: []CatalogMessage{
			{
				Name: "Opening Prayer", Date: MustParseDateOnly("2026-10-11"),
				Type: Prayer, Ministry: WordOfLife, Visibility: Public,
			},
		},
	}

	msg, _, err := cat.FindMessage(MessageLookup{
		Date:       MustParseDateOnly("2026-10-11"),
		Type:       Message,
		TypeIsHint: true,
	})

	t.NoError(err)
	t.Equal("Opening Prayer", msg.Name)
}

func (t *LookupTestSuite) TestDescribeType() {
	// a hint covers more than the one type it names, and says so
	t.Equal("message (or similar)",
		MessageLookup{Type: Message, TypeIsHint: true}.DescribeType())

	// an inferred prayer is exact, so it is named plainly
	t.Equal("prayer", MessageLookup{Type: Prayer, TypeIsHint: true}.DescribeType())

	// an asserted type is exact whatever it is
	t.Equal("message", MessageLookup{Type: Message}.DescribeType())
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

// TestTrack_CannotResolveZeroTracks covers the shape that track numbers genuinely
// cannot disambiguate, and which is common in the live catalog: a message and its Q&A
// session on the same date, neither in a series, so both carry track 0. Measured over
// 2024+, 39 of 60 ambiguous groups look like this. The lookup must surface the
// candidates for the operator to choose between rather than guess.
func (t *LookupTestSuite) TestTrack_CannotResolveZeroTracks() {
	cat := &Catalog{
		Messages: []CatalogMessage{
			{
				Name: "Power of Prayer", Date: MustParseDateOnly("2026-05-17"), Type: Message,
				Ministry: WordOfLife, Visibility: Public,
			},
			{
				Name: "Power of Prayer: Q&A", Date: MustParseDateOnly("2026-05-17"), Type: Message,
				Ministry: WordOfLife, Visibility: Public,
			},
		},
	}

	// no track supplied
	_, _, err := cat.FindMessage(MessageLookup{
		Date: MustParseDateOnly("2026-05-17"),
		Type: Message,
	})
	t.Error(err)

	var ambiguous *AmbiguousLookupError
	t.ErrorAs(err, &ambiguous)
	t.Len(ambiguous.Candidates, 2)

	// supplying a track cannot help: neither candidate has one
	_, _, err = cat.FindMessage(MessageLookup{
		Date:  MustParseDateOnly("2026-05-17"),
		Type:  Message,
		Track: 1,
	})
	t.ErrorAs(err, &ambiguous)
	t.Len(ambiguous.Candidates, 2)
}

func (t *LookupTestSuite) TestDescribeForChoice() {
	// a message in a series shows the series and track
	inSeries := CatalogMessage{
		Name:     "Offense, Part 1",
		Speakers: []string{"Pastor Vern Peltz"},
		Series:   []SeriesReference{{Name: "Offense", Index: 1}},
	}
	t.Equal("Offense, Part 1  [Offense, track 1]  (Pastor Vern Peltz)",
		inSeries.DescribeForChoice())

	// a standalone message shows neither
	standalone := CatalogMessage{
		Name:     "Power of Prayer: Q&A",
		Speakers: []string{"Pastor Vern Peltz"},
	}
	t.Equal("Power of Prayer: Q&A  (Pastor Vern Peltz)", standalone.DescribeForChoice())

	// a row that is not an ordinary message says so, because that is what tells it
	// apart from the message beside it on the same date
	training := CatalogMessage{
		Name:     "The Importance of Voting",
		Type:     Training,
		Speakers: []string{"Pastor Vern Peltz"},
	}
	t.Equal("The Importance of Voting  <training>  (Pastor Vern Peltz)",
		training.DescribeForChoice())

	// an ordinary message does not, so the common case stays uncluttered
	ordinary := CatalogMessage{
		Name:     "The Works of God",
		Type:     Message,
		Speakers: []string{"Pastor Vern Peltz"},
	}
	t.Equal("The Works of God  (Pastor Vern Peltz)", ordinary.DescribeForChoice())
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
