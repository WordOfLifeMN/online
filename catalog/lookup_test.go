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

// One row on the date needs no question, and carries its series with it
func (t *LookupTestSuite) TestFindMessage_OnlyRowOnDate() {
	cat := &Catalog{
		Series: []CatalogSeri{{ID: "S1", Name: "SERIES", Visibility: Public}},
		Messages: []CatalogMessage{
			{
				Name: "Walking in Faith", Date: MustParseDateOnly("2026-03-08"),
				Type: Message, Ministry: WordOfLife, Visibility: Public,
				Series: []SeriesReference{{Name: "SERIES", Index: 3}},
			},
		},
	}

	msg, seri, err := cat.FindMessage(MessageLookup{Date: MustParseDateOnly("2026-03-08")})

	t.NoError(err)
	t.Equal("Walking in Faith", msg.Name)
	t.NotNil(seri)
	t.Equal("SERIES", seri.Name)
}

// A lone row resolves whatever its type. Nothing is compared against the type, so a
// training, word, testimony, song or special-event is found exactly as a message is -
// roughly 400 of the live catalog's rows that an inferred "message" used to risk.
func (t *LookupTestSuite) TestFindMessage_TypeIsNeverCompared() {
	cat := &Catalog{
		Messages: []CatalogMessage{
			{
				Name: "Leadership Training", Date: MustParseDateOnly("2026-10-02"),
				Type: Training, Speakers: []string{"Terry Francis"},
				Ministry: WordOfLife, Visibility: Public,
			},
		},
	}

	msg, _, err := cat.FindMessage(MessageLookup{Date: MustParseDateOnly("2026-10-02")})

	t.NoError(err)
	t.Equal("Leadership Training", msg.Name)
	t.Equal("Terry Francis", msg.SpeakerString())
}

// A prayer and a message on one date are both candidates. Type used to separate them
// silently, which is the whole problem: it was right often enough to be trusted and
// wrong often enough to publish the wrong row.
func (t *LookupTestSuite) TestFindMessage_PrayerIsAlwaysACandidate() {
	cat := t.newLookupCatalog()

	_, _, err := cat.FindMessage(MessageLookup{Date: MustParseDateOnly("2026-03-08")})

	var ambiguous *AmbiguousLookupError
	t.ErrorAs(err, &ambiguous)
	t.Len(ambiguous.Candidates, 2)

	names := []string{ambiguous.Candidates[0].Name, ambiguous.Candidates[1].Name}
	t.Contains(names, "Opening Prayer")
	t.Contains(names, "Walking in Faith")
}

// Regression, 2026-10-11. "2026-10-11-m Learning at Moriah.mp4" resolved silently to
// "Servanthood": the file name's "-m" was read as "not a prayer", which eliminated the
// row the sheet calls a prayer and left exactly one survivor. The title - the one piece
// of evidence that actually identified the video - was never consulted, because it is
// only used to rank candidates and by then there was nothing to rank.
func (t *LookupTestSuite) TestFindMessage_PrayerRowSurvivesAMessageFileName() {
	cat := &Catalog{
		Messages: []CatalogMessage{
			{
				Name: "Learning at Moriah", Date: MustParseDateOnly("2026-10-11"),
				Type: Prayer, Ministry: WordOfLife, Visibility: Public,
			},
			{
				Name: "Servanthood", Date: MustParseDateOnly("2026-10-11"),
				Type: Message, Ministry: WordOfLife, Visibility: Public,
			},
		},
	}

	_, _, err := cat.FindMessage(MessageLookup{Date: MustParseDateOnly("2026-10-11")})

	var ambiguous *AmbiguousLookupError
	t.ErrorAs(err, &ambiguous)
	t.Len(ambiguous.Candidates, 2, "the prayer row must reach the operator's choice")
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

// Every row on the date is offered, prayer included. Three rows on one service day
// means a list of three, not a narrowed guess.
func (t *LookupTestSuite) TestFindMessage_WholeServiceDayIsOffered() {
	cat := t.newServiceDayCatalog()

	_, _, err := cat.FindMessage(MessageLookup{Date: MustParseDateOnly("2026-10-04")})

	var ambiguous *AmbiguousLookupError
	t.ErrorAs(err, &ambiguous)
	t.Len(ambiguous.Candidates, 3)

	names := []string{
		ambiguous.Candidates[0].Name,
		ambiguous.Candidates[1].Name,
		ambiguous.Candidates[2].Name,
	}
	t.Contains(names, "Pay Attention")
	t.Contains(names, "The Importance of Voting")
	t.Contains(names, "The Works of God")
}

func (t *LookupTestSuite) TestFindMessage_NotFound() {
	cat := t.newLookupCatalog()

	_, _, err := cat.FindMessage(MessageLookup{
		Date: MustParseDateOnly("2026-03-15"),
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
	// the date already identifies one message, so a track that does not match it must
	// not cause the obvious match to be rejected
	cat := &Catalog{
		Messages: []CatalogMessage{
			{
				Name: "Walking in Faith", Date: MustParseDateOnly("2026-03-08"),
				Type: Message, Ministry: WordOfLife, Visibility: Public,
				Series: []SeriesReference{{Name: "SERIES", Index: 3}},
			},
		},
	}

	msg, _, err := cat.FindMessage(MessageLookup{
		Date:  MustParseDateOnly("2026-03-08"),
		Track: 99,
	})

	t.NoError(err)
	t.Equal("Walking in Faith", msg.Name)
}

func (t *LookupTestSuite) TestTrack_ResolvesAmbiguity() {
	cat := t.newAmbiguousCatalog()

	msg, _, err := cat.FindMessage(MessageLookup{
		Date:  MustParseDateOnly("2026-03-08"),
		Track: 5,
	})

	t.NoError(err)
	t.Equal("Part Two", msg.Name)
}

func (t *LookupTestSuite) TestTrack_AmbiguousWithNoTrack() {
	cat := t.newAmbiguousCatalog()

	_, _, err := cat.FindMessage(MessageLookup{
		Date: MustParseDateOnly("2026-03-08"),
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
	})
	t.Error(err)

	var ambiguous *AmbiguousLookupError
	t.ErrorAs(err, &ambiguous)
	t.Len(ambiguous.Candidates, 2)

	// supplying a track cannot help: neither candidate has one
	_, _, err = cat.FindMessage(MessageLookup{
		Date:  MustParseDateOnly("2026-05-17"),
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
	})

	t.NoError(err)
	t.Equal("Standalone", msg.Name)
	t.Nil(seri)
}
