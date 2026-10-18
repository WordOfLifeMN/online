package ytpkg

import (
	"strings"
	"testing"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/stretchr/testify/suite"
)

// the real catalog is what callers pass
var _ SeriesFinder = (*catalog.Catalog)(nil)

func TestCatalogTestSuite(t *testing.T) {
	suite.Run(t, new(CatalogTestSuite))
}

type CatalogTestSuite struct {
	suite.Suite
}

// fakeFinder finds series by case-insensitive name, as the catalog does
type fakeFinder []catalog.CatalogSeri

func (f fakeFinder) FindSeriByName(name string) (*catalog.CatalogSeri, bool) {
	for index := range f {
		if strings.EqualFold(f[index].Name, name) {
			return &f[index], true
		}
	}
	return nil, false
}

func (t *CatalogTestSuite) TestMessage_Public() {
	msg := catalog.CatalogMessage{
		Name:       "Walking in Faith",
		Speakers:   []string{"Pastor Vern Peltz"},
		Date:       catalog.MustParseDateOnly("2021-03-08"),
		Ministry:   catalog.WordOfLife,
		Visibility: catalog.Public,
		Series:     []catalog.SeriesReference{{Name: "Walking in Faith", Index: 3}},
	}
	cat := fakeFinder{{Name: "Walking in Faith"}}

	pkg, err := NewMessageFromCatalog(&msg, cat, "Walking in Faith", "A summary.", "/path/thumb.jpg")
	t.NoError(err)

	t.Equal(ChannelWOL, pkg.Channel)
	t.Equal("Walking in Faith | March 8, 2021", pkg.Title)
	t.Empty(pkg.Alternative, "the generated title agrees with the sheet")
	t.Equal("A summary.", pkg.Description)
	t.Equal([]PlaylistRef{{Title: "Walking in Faith", Position: 3}}, pkg.Playlists)
	t.Equal(PrivacyPublic, pkg.Privacy)
	t.Equal("/path/thumb.jpg", pkg.ThumbnailPath)

	// house defaults
	t.Equal("29", pkg.CategoryID)
	t.Equal("en", pkg.DefaultLanguage)
	t.Equal("en", pkg.DefaultAudioLanguage)
	t.False(pkg.MadeForKids)
}

func (t *CatalogTestSuite) TestMessage_AlternativeTitle() {
	msg := catalog.CatalogMessage{
		Name:       "Walking in Faith",
		Speakers:   []string{"Pastor Vern Peltz"},
		Date:       catalog.MustParseDateOnly("2021-03-08"),
		Ministry:   catalog.WordOfLife,
		Visibility: catalog.Public,
	}

	pkg, err := NewMessageFromCatalog(&msg, nil, "Trusting the Unseen", "A summary.", "")
	t.NoError(err)

	// the sheet still supplies the title; the alternative is offered in the same shape
	t.Equal("Walking in Faith | March 8, 2021", pkg.Title)
	t.Equal("Trusting the Unseen | March 8, 2021", pkg.Alternative)
}

func (t *CatalogTestSuite) TestMessage_Core() {
	msg := catalog.CatalogMessage{
		Name:       "Boundaries",
		Speakers:   []string{"Pastor Mary Peltz"},
		Date:       catalog.MustParseDateOnly("2026-10-04"),
		Ministry:   catalog.CORE_HopeDealers,
		Visibility: catalog.Partner,
		Series:     []catalog.SeriesReference{{Name: "Healthy Limits", Index: 2}},
	}

	pkg, err := NewMessageFromCatalog(&msg, fakeFinder{{Name: "Healthy Limits"}}, "", "S.", "")
	t.NoError(err)

	t.Equal(ChannelWOL, pkg.Channel)
	t.Equal("CORE | Boundaries | October 4, 2026", pkg.Title)
	t.Equal([]PlaylistRef{{Title: "CORE | Healthy Limits", Position: 2}}, pkg.Playlists)
	t.Equal(PrivacyUnlisted, pkg.Privacy)
}

func (t *CatalogTestSuite) TestMessage_RawIsRefused() {
	msg := catalog.CatalogMessage{
		Name:       "Raw Stream",
		Ministry:   catalog.WordOfLife,
		Visibility: catalog.Raw,
	}

	pkg, err := NewMessageFromCatalog(&msg, nil, "Raw Stream", "A summary.", "")
	t.Nil(pkg)
	t.ErrorContains(err, "livestream")
}

// track 0 is ancillary to the public series, so it gives no playlist
func (t *CatalogTestSuite) TestMessage_TrackZeroHasNoPlaylist() {
	msg := catalog.CatalogMessage{
		Name:       "Q&A",
		Ministry:   catalog.WordOfLife,
		Visibility: catalog.Public,
		Series:     []catalog.SeriesReference{{Name: "Prayer", Index: 0}},
	}

	pkg, err := NewMessageFromCatalog(&msg, fakeFinder{{Name: "Prayer"}}, "", "S.", "")
	t.NoError(err)
	t.Empty(pkg.Playlists)
}

func (t *CatalogTestSuite) TestMessage_MultipleSeries() {
	msg := catalog.CatalogMessage{
		Name:       "Pay Attention",
		Ministry:   catalog.WordOfLife,
		Visibility: catalog.Public,
		Series: []catalog.SeriesReference{
			{Name: "prayer", Index: 3},
			{Name: "Hidden", Index: 0},
			{Name: "Women of Faith", Index: 7},
		},
	}
	cat := fakeFinder{{Name: "Prayer"}, {Name: "Hidden"}, {Name: "Women of Faith"}}

	pkg, err := NewMessageFromCatalog(&msg, cat, "", "S.", "")
	t.NoError(err)
	t.Equal([]PlaylistRef{
		{Title: "Prayer", Position: 3}, // the sheet's spelling of the series
		{Title: "Women of Faith", Position: 7},
	}, pkg.Playlists)
}

// an Ask the Pastor message goes in the one ATP playlist even when it names another
// series, and without a position
func (t *CatalogTestSuite) TestMessage_AskThePastor() {
	msg := catalog.CatalogMessage{
		Name:       "Is Tithing for Today?",
		Ministry:   catalog.AskThePastor,
		Visibility: catalog.Public,
		Series:     []catalog.SeriesReference{{Name: "Some Other Series", Index: 4}},
	}

	pkg, err := NewMessageFromCatalog(&msg, fakeFinder{{Name: "Some Other Series"}}, "", "S.", "")
	t.NoError(err)
	t.Equal([]PlaylistRef{{Title: "Ask the Pastor"}}, pkg.Playlists)
}

// a message marked stand-alone gets no playlist
func (t *CatalogTestSuite) TestMessage_StandAloneHasNoPlaylist() {
	msg := catalog.CatalogMessage{
		Name:       "Christmas Eve",
		Ministry:   catalog.WordOfLife,
		Visibility: catalog.Public,
		Series:     []catalog.SeriesReference{{Name: "SAM", Index: 1}},
	}

	pkg, err := NewMessageFromCatalog(&msg, fakeFinder{{ID: "SAM-abc", Name: "Christmas Eve"}}, "", "S.", "")
	t.NoError(err)
	t.Empty(pkg.Playlists)
}

// A series with no Series row of its own shares its name with another message, which,
// being in no series, the catalog gave a stand-alone series of that name. The reference
// is to the real series, so it still gets its playlist. Built through the real catalog,
// as found on 2026-10-17 with "Voting".
func (t *CatalogTestSuite) TestMessage_StandAloneNameCollision() {
	cat := &catalog.Catalog{Messages: []catalog.CatalogMessage{
		{
			Date: catalog.MustParseDateOnly("2026-10-03"), Name: "Voting",
			Ministry: catalog.WordOfLife, Visibility: catalog.Public,
		},
		{
			Date: catalog.MustParseDateOnly("2026-10-04"), Name: "The Importance of Voting",
			Ministry: catalog.WordOfLife, Visibility: catalog.Partner,
			Series: []catalog.SeriesReference{{Name: "Voting", Index: 1}},
		},
	}}
	t.Require().NoError(cat.Initialize())

	seri, ok := cat.FindSeriByName("Voting")
	t.Require().True(ok, "the catalog made a stand-alone series for the message")
	t.True(strings.HasPrefix(seri.ID, "SAM-"))
	t.Nil(FindSeries(cat, "Voting"), "but it is not the series the reference means")

	pkg, err := NewMessageFromCatalog(&cat.Messages[1], cat, "", "", "")
	t.NoError(err)
	t.Equal([]PlaylistRef{{Title: "Voting", Position: 1}}, pkg.Playlists)
}

func (t *CatalogTestSuite) TestMessage_NoSeries() {
	msg := catalog.CatalogMessage{Name: "M", Ministry: catalog.WordOfLife, Visibility: catalog.Public}

	pkg, err := NewMessageFromCatalog(&msg, nil, "", "S.", "")
	t.NoError(err)
	t.Empty(pkg.Playlists)
}

// the booklets of the message's first series reach its description
func (t *CatalogTestSuite) TestMessage_DescriptionUsesFirstSeries() {
	msg := catalog.CatalogMessage{
		Name:       "M",
		Ministry:   catalog.WordOfLife,
		Visibility: catalog.Public,
		Series:     []catalog.SeriesReference{{Name: "Prayer", Index: 1}},
	}
	cat := fakeFinder{{
		Name:     "Prayer",
		Booklets: []catalog.OnlineResource{{URL: "https://x/booklet.pdf", Name: "Booklet"}},
	}}

	pkg, err := NewMessageFromCatalog(&msg, cat, "", "S.", "")
	t.NoError(err)
	t.Equal("S.\n\nBooklet\nhttps://x/booklet.pdf", pkg.Description)
}

func (t *CatalogTestSuite) TestSeries() {
	seri := catalog.CatalogSeri{
		ID:          "CORE-123",
		Name:        "Healthy Limits",
		Description: "About limits.",
		Visibility:  catalog.Private,
		Messages:    []catalog.CatalogMessage{{Ministry: catalog.CenterOfRelationshipExperience}},
	}

	pkg, err := NewSeriesFromCatalog(&seri)
	t.NoError(err)
	t.Equal(ChannelWOL, pkg.Channel)
	t.Equal("CORE | Healthy Limits", pkg.Title)
	t.Equal("About limits.", pkg.Description)
	t.Equal(PrivacyUnlisted, pkg.Privacy)
}

func (t *CatalogTestSuite) TestSeries_Refused() {
	_, err := NewSeriesFromCatalog(&catalog.CatalogSeri{ID: "SAM-abc", Name: "X", Visibility: catalog.Public})
	t.ErrorContains(err, "stand-alone")

	_, err = NewSeriesFromCatalog(&catalog.CatalogSeri{Name: "X", Visibility: catalog.Raw})
	t.Error(err)

	_, err = NewSeriesFromCatalog(nil)
	t.Error(err)
}

func (t *CatalogTestSuite) TestSeriesForLive() {
	seri := catalog.CatalogSeri{
		ID: "CORE-1", Name: "Healthy Limits", Description: "About limits.", Visibility: catalog.Private,
		Messages: []catalog.CatalogMessage{{Ministry: catalog.CenterOfRelationshipExperience}},
	}

	// a private series kept private on YouTube stays private, and the live title stays
	want, err := NewSeriesForLive(&seri, &Series{ID: "PL1", Title: "CORE | Healthy Limits (old)", Privacy: PrivacyPrivate})
	t.NoError(err)
	t.Equal("PL1", want.ID)
	t.Equal("CORE | Healthy Limits (old)", want.Title)
	t.Equal(PrivacyPrivate, want.Privacy)
	t.Equal("About limits.", want.Description)

	// public on YouTube for a private series is corrected
	want, err = NewSeriesForLive(&seri, &Series{ID: "PL1", Privacy: PrivacyPublic})
	t.NoError(err)
	t.Equal(PrivacyUnlisted, want.Privacy)
}
