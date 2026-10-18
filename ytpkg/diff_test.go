package ytpkg

import (
	"strings"
	"testing"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/util/box"
	"github.com/stretchr/testify/suite"
)

type boxT = box.Box

func TestDiffTestSuite(t *testing.T) {
	suite.Run(t, new(DiffTestSuite))
}

type DiffTestSuite struct {
	suite.Suite
}

// liveVideo is a video exactly as the sheet row below wants it
func liveVideo() *Message {
	return &Message{
		ID:                   "01LiNfRzMo8",
		ChannelID:            "UCZF_jDrfP6OltpviYmj-mkA",
		Title:                "CORE | Boundaries | Mary Peltz | October 4, 2026",
		Description:          "About boundaries.\n\nNotes\nhttps://x/notes.pdf",
		Privacy:              PrivacyPublic,
		CategoryID:           "29",
		DefaultLanguage:      "en",
		DefaultAudioLanguage: "en",
		MadeForKids:          false,

		// unmanaged, and never compared
		Tags:       []string{"faith"},
		License:    "youtube",
		Embeddable: true,
	}
}

func sheetRow() *catalog.CatalogMessage {
	return &catalog.CatalogMessage{
		Name:        "Boundaries",
		Speakers:    []string{"Pastor Mary Peltz"},
		Date:        catalog.MustParseDateOnly("2026-10-04"),
		Ministry:    catalog.CenterOfRelationshipExperience,
		Visibility:  catalog.Public,
		Description: "About boundaries.",
		Resources:   []catalog.OnlineResource{{URL: "https://x/notes.pdf", Name: "Notes"}},
	}
}

func (t *DiffTestSuite) want(live *Message, row *catalog.CatalogMessage) *Message {
	want, err := NewMessageForLive(row, nil, live)
	t.Require().NoError(err)
	return want
}

func fields(changes []Change) []string {
	var out []string
	for _, change := range changes {
		out = append(out, change.Field)
	}
	return out
}

func (t *DiffTestSuite) TestInSync() {
	live := liveVideo()
	t.Empty(DiffMessage(live, t.want(live, sheetRow())))
}

func (t *DiffTestSuite) TestEveryManagedField() {
	live := liveVideo()
	live.Title = "Boundaries | Mary Peltz | October 4, 2026"
	live.Description = "About boundaries."
	live.CategoryID = "22"
	live.DefaultLanguage = ""
	live.DefaultAudioLanguage = "en-US"
	live.MadeForKids = true
	live.Privacy = PrivacyUnlisted

	changes := DiffMessage(live, t.want(live, sheetRow()))

	t.Equal([]string{"Title", "Privacy", "Category", "Language", "Audio lang", "For kids", "Description"},
		fields(changes))
	t.Equal(Change{Field: "Category", Now: "22", New: "29"}, changes[2])
	t.Equal(Change{Field: "For kids", Now: "true", New: "false"}, changes[5])
	t.False(HasDestructive(changes), "adding links to a description is not destructive")
}

// tags, license, embeddable and the rest are left alone, so they never show as changes
func (t *DiffTestSuite) TestUnmanagedFieldsAreIgnored() {
	live := liveVideo()
	want := t.want(live, sheetRow())
	want.Tags = nil
	want.License = "creativeCommon"
	want.Embeddable = false
	want.ThumbnailURL = "elsewhere"

	t.Empty(DiffMessage(live, want))
}

// YouTube strips trailing whitespace and may hand back Windows line endings
func (t *DiffTestSuite) TestDescriptionWhitespaceIsNotAChange() {
	live := liveVideo()
	live.Description = "About boundaries.  \r\n\r\nNotes\r\nhttps://x/notes.pdf\n\n"
	t.Empty(DiffMessage(live, t.want(live, sheetRow())))
}

// +---------------------------------------------------------------------------
// | Privacy (Q8)
// +---------------------------------------------------------------------------

func (t *DiffTestSuite) privacyChange(view catalog.View, live Privacy) []Change {
	video := liveVideo()
	video.Privacy = live
	row := sheetRow()
	row.Visibility = view

	var out []Change
	for _, change := range DiffMessage(video, t.want(video, row)) {
		if change.Field == FieldPrivacy {
			out = append(out, change)
		}
	}
	return out
}

func (t *DiffTestSuite) TestPrivacy_PrivateIsKept() {
	t.Empty(t.privacyChange(catalog.Private, PrivacyPrivate))
	t.Empty(t.privacyChange(catalog.Private, PrivacyUnlisted))
}

func (t *DiffTestSuite) TestPrivacy_PartnerPrivateBecomesUnlisted() {
	t.Equal([]Change{{Field: "Privacy", Now: "private", New: "unlisted"}},
		t.privacyChange(catalog.Partner, PrivacyPrivate))
}

func (t *DiffTestSuite) TestPrivacy_PublicCorrected() {
	t.Equal([]Change{{Field: "Privacy", Now: "unlisted", New: "public"}},
		t.privacyChange(catalog.Public, PrivacyUnlisted))
	t.Equal([]Change{{Field: "Privacy", Now: "public", New: "unlisted"}},
		t.privacyChange(catalog.Private, PrivacyPublic))
}

func (t *DiffTestSuite) TestPrivacy_RawRefused() {
	row := sheetRow()
	row.Visibility = catalog.Raw
	_, err := NewMessageForLive(row, nil, liveVideo())
	t.Error(err)
}

// the description for a live video is the sheet's, not a generated summary
func (t *DiffTestSuite) TestWantUsesSheetDescription() {
	want := t.want(liveVideo(), sheetRow())
	t.Equal("About boundaries.\n\nNotes\nhttps://x/notes.pdf", want.Description)
	t.Equal("01LiNfRzMo8", want.ID)
}

// +---------------------------------------------------------------------------
// | Destructive descriptions (Q9)
// +---------------------------------------------------------------------------

func (t *DiffTestSuite) TestDestructive() {
	// adding links is not destructive
	t.False(IsDestructiveDescription("A summary.", "A summary.\n\nNotes\nhttps://x/notes.pdf"))

	// an empty YouTube description loses nothing
	t.False(IsDestructiveDescription("", "Anything."))
	t.False(IsDestructiveDescription(" \n\n ", "Anything."))

	// reflowing blank lines or indentation loses nothing
	t.False(IsDestructiveDescription("One.\n\n\nTwo.", "  One.\nTwo."))

	// dropping a line is destructive, and the lost line is reported
	t.True(IsDestructiveDescription("A summary.\nWatch live Sundays at 10.", "A summary."))
	t.Equal([]string{"Watch live Sundays at 10."},
		LostDescriptionLines("A summary.\n\nWatch live Sundays at 10.", "A summary.\n\nNotes\nhttps://x"))

	// rewording a line is destructive too: the old wording is not in the sheet
	t.True(IsDestructiveDescription("Pastor Mary talks about attention.", "Sermon notes"))
}

// an edit made in the sheet is not a loss, even though YouTube's line is not in the new
// description word for word (Deviation 2)
func (t *DiffTestSuite) TestDestructive_EditsAreNotLosses() {
	abc := "Pastor Mary talks about paying attention. She walks through three of God's " +
		"breadcrumbs. Then she shows how to follow them every day."
	ac := "Pastor Mary talks about paying attention. Then she shows how to follow them every day."

	// a sentence deleted from the paragraph in the sheet
	t.False(IsDestructiveDescription(abc, ac))
	t.False(IsDestructiveDescription(abc+"\n\nNotes\nhttps://x/notes.pdf", ac+"\n\nNotes\nhttps://x/notes.pdf"))

	// a typo fixed, a word added, punctuation and case changed
	t.False(IsDestructiveDescription("Pastor Mary talks abot attention.", "Pastor Mary talks about attention."))
	t.False(IsDestructiveDescription(ac, ac+" Don't miss it!"))
	t.False(IsDestructiveDescription("pastor mary talks about attention", "Pastor Mary talks about attention."))

	// a paragraph split in two in the sheet keeps its halves
	t.False(IsDestructiveDescription(
		"Pastor Mary talks about paying attention to God. Then she shows how to follow.",
		"Pastor Mary talks about paying attention to God.\nThen she shows how to follow her lead."))
}

// text YouTube alone has is still a loss, edited lines or not
func (t *DiffTestSuite) TestDestructive_RemovalsStillWarn() {
	summary := "Pastor Mary talks about paying attention."

	// a line with no counterpart, beside one that was only edited
	t.Equal([]string{"Watch live Sundays at 10am on our channel."},
		LostDescriptionLines(
			summary+" She has three points.\nWatch live Sundays at 10am on our channel.",
			summary))

	// a hand-written description replaced by a different summary
	t.True(IsDestructiveDescription(
		"Join us for a powerful word on hearing God in the everyday.",
		"Pastor Mary talks about paying attention."))

	// most of a paragraph deleted keeps too little of it to count as an edit
	t.True(IsDestructiveDescription(
		"One two three four five six seven eight nine ten.", "One two three."))
}

func (t *DiffTestSuite) TestDestructiveIsMarkedOnTheChange() {
	live := liveVideo()
	live.Description = "Pastor Mary talks about paying attention."

	changes := DiffMessage(live, t.want(live, sheetRow()))
	t.Require().Len(changes, 1)
	t.True(changes[0].Destructive)
	t.True(HasDestructive(changes))
}

func (t *DiffTestSuite) TestDescriptionDiff() {
	diff := DescriptionDiff("Old line.\nShared.", "Shared.\nNew line.")
	t.Equal("--- YouTube\n+++ Sheet\n@@ -1,2 +1,2 @@\n-Old line.\n Shared.\n+New line.", diff)
}

// +---------------------------------------------------------------------------
// | Series
// +---------------------------------------------------------------------------

func (t *DiffTestSuite) TestDiffSeries() {
	live := &Series{Title: "Old Title", Description: "About.", Privacy: PrivacyPublic}

	t.Empty(DiffSeries(live, &Series{Title: "New Title", Description: "About.", Privacy: PrivacyPublic}),
		"the title is never synced")

	changes := DiffSeries(live, &Series{Description: "About.\n\nBooklet\nhttps://x", Privacy: PrivacyUnlisted})
	t.Equal([]string{"Privacy", "Description"}, fields(changes))
	t.False(changes[1].Destructive)
}

// +---------------------------------------------------------------------------
// | Printing
// +---------------------------------------------------------------------------

func (t *DiffTestSuite) TestPrintChanges() {
	changes := []Change{
		{Field: FieldPrivacy, Now: "unlisted", New: "public"},
		{Field: FieldDefaultLanguage, Now: "", New: "en"},
		{Field: FieldPlaylist, Summary: `add to "Prayer" (track 3)`},
		{Field: FieldDescription, Now: "Pastor Mary talks.", New: "Sermon notes", Destructive: true},
	}

	out := render(func(b *boxT) { PrintChanges(b, changes) })

	positions := []int{}
	for _, marker := range []string{
		"├─ Changes ",
		"│ Privacy   : now: unlisted ",
		"│             new: public ",
		"│ Language  : now: (empty) ",
		"│             new: en ",
		`│ Playlist  : add to "Prayer" (track 3) `,
		"│ Description: see below ",
		"├─ Description ",
		"\n--- YouTube\n+++ Sheet\n",
		"\n-Pastor Mary talks.\n+Sermon notes\n",
		"├─ WARNING ",
		"│ This removes YouTube description text that is not in the sheet: ",
		"\nPastor Mary talks.\n╰",
	} {
		index := strings.Index(out, marker)
		t.GreaterOrEqual(index, 0, "missing %q in\n%s", marker, out)
		positions = append(positions, index)
	}
	t.IsIncreasing(positions)
}

func (t *DiffTestSuite) TestPrintChanges_NoDescriptionNoWarning() {
	out := render(func(b *boxT) { PrintChanges(b, []Change{{Field: FieldTitle, Now: "a", New: "b"}}) })
	t.NotContains(out, "├─ Description ")
	t.NotContains(out, "WARNING")
}
