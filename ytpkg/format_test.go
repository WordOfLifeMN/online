package ytpkg

import (
	"strings"
	"testing"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/stretchr/testify/suite"
)

func TestFormatTestSuite(t *testing.T) {
	suite.Run(t, new(FormatTestSuite))
}

type FormatTestSuite struct {
	suite.Suite
}

var coreFamily = []catalog.Ministry{
	catalog.CenterOfRelationshipExperience,
	catalog.CORE_HealthMatters,
	catalog.CORE_HopeDealers,
	catalog.CORE_RecoveryClasses,
	catalog.CORE_CounselingClasses,
}

// +---------------------------------------------------------------------------
// | Playlist naming
// +---------------------------------------------------------------------------

func (t *FormatTestSuite) TestPlaylist_WOLSeriesIsUnmarked() {
	t.Equal("Walking in Faith", PlaylistTitle(catalog.WordOfLife, "Walking in Faith"))
}

// the marker comes from the ministry family, NOT from Ministry.Description(), which
// returns "C.O.R.E." for the parent but "CORE: Health Matters" for the subs
func (t *FormatTestSuite) TestPlaylist_CoreFamilyIsMarked() {
	for _, ministry := range coreFamily {
		t.Equal("CORE | Eating Well", PlaylistTitle(ministry, "Eating Well"),
			"ministry %s", string(ministry))
	}
}

func (t *FormatTestSuite) TestPlaylist_AskThePastor() {
	t.Equal("Ask the Pastor", AskThePastorPlaylistTitle())
}

// +---------------------------------------------------------------------------
// | Title
// +---------------------------------------------------------------------------

func (t *FormatTestSuite) TestTitle_Format() {
	msg := catalog.CatalogMessage{
		Name:     "Walking in Faith",
		Speakers: []string{"Pastor Vern Peltz"},
		Date:     catalog.MustParseDateOnly("2021-03-08"),
	}

	t.Equal("Walking in Faith | Vern Peltz | March 8, 2021", FormatTitle(&msg, msg.Name))
}

func (t *FormatTestSuite) TestTitle_CoreFamilyIsMarked() {
	for _, ministry := range coreFamily {
		msg := catalog.CatalogMessage{
			Name:     "Boundaries",
			Speakers: []string{"Pastor Mary Peltz"},
			Date:     catalog.MustParseDateOnly("2026-10-04"),
			Ministry: ministry,
		}

		t.Equal("CORE | Boundaries | Mary Peltz | October 4, 2026", FormatTitle(&msg, msg.Name),
			"ministry %s", string(ministry))
	}
}

func (t *FormatTestSuite) TestTitle_NonCoreIsUnmarked() {
	for _, ministry := range []catalog.Ministry{
		catalog.WordOfLife, catalog.AskThePastor, catalog.FaithAndFreedom, catalog.TheBridgeOutreach,
	} {
		msg := catalog.CatalogMessage{Name: "Boundaries", Ministry: ministry}
		t.Equal("Boundaries", FormatTitle(&msg, msg.Name), "ministry %s", string(ministry))
	}
}

func (t *FormatTestSuite) TestTitle_MultipleSpeakers() {
	msg := catalog.CatalogMessage{
		Name:     "Together",
		Speakers: []string{"Pastor Vern Peltz", "Pastor Mary Peltz"},
		Date:     catalog.MustParseDateOnly("2021-03-08"),
	}

	t.Equal("Together | Vern Peltz, Mary Peltz | March 8, 2021", FormatTitle(&msg, msg.Name))
}

// A message recorded ahead of its service date must still be titled with the plain
// date. CatalogMessage.DateString would prefix "Scheduled for " here.
func (t *FormatTestSuite) TestTitle_FutureDateHasNoPrefix() {
	msg := catalog.CatalogMessage{
		Name:     "Christmas Eve Service",
		Speakers: []string{"Pastor Vern Peltz"},
		Date:     catalog.MustParseDateOnly("2099-12-24"),
	}

	title := FormatTitle(&msg, msg.Name)
	t.Equal("Christmas Eve Service | Vern Peltz | December 24, 2099", title)
	t.NotContains(title, "Scheduled")
}

func (t *FormatTestSuite) TestStripSpeakerTitle() {
	t.Equal("Vern Peltz", stripSpeakerTitle("Pastor Vern Peltz"))

	// a name carrying no honorific is untouched
	t.Equal("Terry Francis", stripSpeakerTitle("Terry Francis"))

	// only a leading honorific goes, and only once - a surname that happens to contain
	// the word must survive
	t.Equal("Vern Pastor", stripSpeakerTitle("Vern Pastor"))
	t.Equal("Pastor Vern Peltz", stripSpeakerTitle("Pastor Pastor Vern Peltz"))

	// a speaker recorded as the bare word is left alone rather than emptied
	t.Equal("Pastor", stripSpeakerTitle("Pastor"))

	// tolerant of casing and stray whitespace in the sheet
	t.Equal("Vern Peltz", stripSpeakerTitle("pastor Vern Peltz"))
	t.Equal("Vern Peltz", stripSpeakerTitle("  Pastor   Vern Peltz  "))
}

// Stripping is confined to the title. SpeakerString keeps the honorific, which is what
// the summary prompt and the spreadsheet confirmation line use, so it still reaches the
// description through the generated summary.
func (t *FormatTestSuite) TestTitle_StrippingDoesNotAffectSpeakerString() {
	msg := catalog.CatalogMessage{
		Name:     "The Importance of Voting",
		Speakers: []string{"Pastor Vern Peltz"},
	}

	t.Equal("Pastor Vern Peltz", msg.SpeakerString())
	t.Equal("Vern Peltz", getTitleSpeakers(&msg))
}

func (t *FormatTestSuite) TestTitle_NoSpeaker() {
	msg := catalog.CatalogMessage{
		Name: "Walking in Faith",
		Date: catalog.MustParseDateOnly("2021-03-08"),
	}

	t.Equal("Walking in Faith | March 8, 2021", FormatTitle(&msg, msg.Name))
}

// +---------------------------------------------------------------------------
// | Alternative title
// +---------------------------------------------------------------------------

func (t *FormatTestSuite) TestAlternative_ShownWhenItDiffers() {
	msg := catalog.CatalogMessage{
		Name:     "Walking in Faith",
		Speakers: []string{"Pastor Vern Peltz"},
		Date:     catalog.MustParseDateOnly("2021-03-08"),
	}

	t.Equal("Trusting the Unseen | Vern Peltz | March 8, 2021",
		AlternativeTitle(&msg, "Trusting the Unseen"))
}

func (t *FormatTestSuite) TestAlternative_CoreIsMarked() {
	msg := catalog.CatalogMessage{
		Name:     "Boundaries",
		Ministry: catalog.CenterOfRelationshipExperience,
	}

	t.Equal("CORE | Healthy Limits", AlternativeTitle(&msg, "Healthy Limits"))
}

// A title that says the same thing is not an alternative. Offering a choice between
// "Walking In Faith" and "Walking in Faith" teaches the operator to stop reading the
// line, which costs the real alternatives their audience.
func (t *FormatTestSuite) TestAlternative_SuppressedWhenItAgrees() {
	msg := catalog.CatalogMessage{
		Name:     "Power of Prayer: Q&A",
		Speakers: []string{"Pastor Vern Peltz"},
		Date:     catalog.MustParseDateOnly("2021-03-08"),
	}

	t.Empty(AlternativeTitle(&msg, "Power of Prayer: Q&A"), "identical")
	t.Empty(AlternativeTitle(&msg, "power of prayer: q&a"), "case only")
	t.Empty(AlternativeTitle(&msg, "Power of Prayer - Q&A"), "punctuation only")
	t.Empty(AlternativeTitle(&msg, "  Power of Prayer: Q&A  "), "whitespace only")

	// rewording is not punctuation, so this does count as an alternative. The rule is
	// deliberately literal: judging whether two phrasings mean the same thing is the
	// operator's job, and this line exists to put the question in front of them.
	t.NotEmpty(AlternativeTitle(&msg, "Power of Prayer - Q and A"))
}

// no generated title - the model was never called, or returned nothing
func (t *FormatTestSuite) TestAlternative_SuppressedWhenAbsent() {
	msg := catalog.CatalogMessage{Name: "Walking in Faith"}

	t.Empty(AlternativeTitle(&msg, ""))
	t.Empty(AlternativeTitle(&msg, "   "))
}

// +---------------------------------------------------------------------------
// | Description
// +---------------------------------------------------------------------------

func (t *FormatTestSuite) TestDescription_NoResources() {
	msg := catalog.CatalogMessage{Name: "MSG"}
	t.Equal("A short summary.", MessageDescription(&msg, nil, "A short summary."))
}

func (t *FormatTestSuite) TestDescription_FallsBackToSheetDescription() {
	msg := catalog.CatalogMessage{Name: "MSG", Description: "From the sheet."}
	t.Equal("From the sheet.", MessageDescription(&msg, nil, ""))
}

func (t *FormatTestSuite) TestDescription_Layout() {
	msg := catalog.CatalogMessage{
		Name:      "MSG",
		Resources: []catalog.OnlineResource{{URL: "https://x/notes.pdf", Name: "Sermon Notes"}},
	}
	seri := catalog.CatalogSeri{
		Name:     "SERIES",
		Booklets: []catalog.OnlineResource{{URL: "https://x/booklet.pdf", Name: "Booklet"}},
	}

	t.Equal("A short summary.\n\nSermon Notes\nhttps://x/notes.pdf\n\nBooklet\nhttps://x/booklet.pdf",
		MessageDescription(&msg, &seri, "A short summary."))
}

// A real series carries the resources of every message it contains, because
// Normalize() gathers them there. Only this message's own may reach its
// description - the notes for message 1 do not belong on message 8.
func (t *FormatTestSuite) TestDescription_ExcludesOtherMessagesInTheSeries() {
	mine := catalog.CatalogMessage{
		Name:      "Servanthood",
		Resources: []catalog.OnlineResource{{URL: "https://x/2026-10-11+Servanthood.pdf", Name: "Notes"}},
	}
	theirs := catalog.CatalogMessage{
		Name:      "Humility",
		Resources: []catalog.OnlineResource{{URL: "https://x/2026-09-20+Humility.pdf", Name: "Notes"}},
	}

	seri := catalog.CatalogSeri{
		Name:     "SERIES",
		Booklets: []catalog.OnlineResource{{URL: "https://x/booklet.pdf", Name: "Booklet"}},
		Messages: []catalog.CatalogMessage{mine, theirs},
	}
	seri.Normalize()
	t.Len(seri.Resources, 2, "Normalize() should have gathered both messages' resources")

	result := MessageDescription(&mine, &seri, "Summary.")
	t.Contains(result, "https://x/2026-10-11+Servanthood.pdf")
	t.Contains(result, "https://x/booklet.pdf", "the series booklet is genuinely series-wide")
	t.NotContains(result, "Humility", "another message's notes must not appear")
}

func (t *FormatTestSuite) TestDescription_DeduplicatesResources() {
	msg := catalog.CatalogMessage{
		Name:      "MSG",
		Resources: []catalog.OnlineResource{{URL: "https://x/same.pdf", Name: "Same"}},
	}
	seri := catalog.CatalogSeri{
		Name:     "SERIES",
		Booklets: []catalog.OnlineResource{{URL: "https://x/same.pdf", Name: "Same"}},
	}

	t.Len(collectResources(&msg, &seri), 1)
}

func (t *FormatTestSuite) TestSeriesDescription() {
	seri := catalog.CatalogSeri{
		Name:        "SERIES",
		Description: "About the series.",
		Booklets:    []catalog.OnlineResource{{URL: "https://x/booklet.pdf", Name: "Booklet"}},
	}

	t.Equal("About the series.\n\nBooklet\nhttps://x/booklet.pdf", SeriesDescription(&seri))
	t.Equal("", SeriesDescription(nil))
	t.Equal("Bare.", SeriesDescription(&catalog.CatalogSeri{Description: "Bare."}))
}

// +---------------------------------------------------------------------------
// | Validation
// +---------------------------------------------------------------------------

func (t *FormatTestSuite) TestValidate_Accepts() {
	t.NoError(Validate(strings.Repeat("a", 100), strings.Repeat("b", 5000)))

	// the title limit is characters, not bytes
	t.NoError(Validate(strings.Repeat("é", 100), "—"))
}

func (t *FormatTestSuite) TestValidate_Rejects() {
	t.ErrorContains(Validate(strings.Repeat("a", 101), ""), "101 characters")
	t.ErrorContains(Validate("T", strings.Repeat("b", 5001)), "5001 bytes")
	t.ErrorContains(Validate("a < b", ""), "title contains")
	t.ErrorContains(Validate("T", "a > b"), "description contains")
	t.ErrorContains(Validate("  ", "D"), "empty")
}

func (t *FormatTestSuite) TestValidateDescription() {
	t.NoError(ValidateDescription(""))
	t.NoError(ValidateDescription(strings.Repeat("b", 5000)))
	t.ErrorContains(ValidateDescription(strings.Repeat("b", 5001)), "5001 bytes")
	t.ErrorContains(ValidateDescription("a < b"), "description contains")
}
