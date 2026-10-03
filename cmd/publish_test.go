package cmd

import (
	"testing"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/stretchr/testify/suite"
)

func TestPublishTestSuite(t *testing.T) {
	suite.Run(t, new(PublishTestSuite))
}

type PublishTestSuite struct {
	suite.Suite
}

// +---------------------------------------------------------------------------
// | Privacy mapping
// +---------------------------------------------------------------------------

func (t *PublishTestSuite) TestPrivacy_Public() {
	privacy, err := getYouTubePrivacy(catalog.Public)
	t.NoError(err)
	t.Equal(PrivacyPublic, privacy)
}

func (t *PublishTestSuite) TestPrivacy_PartnerAndPrivateBothUnlisted() {
	// partner and private are not distinguishable on YouTube
	partner, err := getYouTubePrivacy(catalog.Partner)
	t.NoError(err)
	t.Equal(PrivacyUnlisted, partner)

	private, err := getYouTubePrivacy(catalog.Private)
	t.NoError(err)
	t.Equal(PrivacyUnlisted, private)

	t.Equal(partner, private)
}

func (t *PublishTestSuite) TestPrivacy_RawIsRefused() {
	_, err := getYouTubePrivacy(catalog.Raw)
	t.Error(err)
	t.Contains(err.Error(), "livestream")
}

func (t *PublishTestSuite) TestPrivacy_UnknownIsRefused() {
	_, err := getYouTubePrivacy(catalog.UnknownView)
	t.Error(err)
}

// +---------------------------------------------------------------------------
// | Channel mapping
// +---------------------------------------------------------------------------

func (t *PublishTestSuite) TestChannel_AllMinistries() {
	expected := map[catalog.Ministry]YouTubeChannel{
		catalog.WordOfLife:                     ChannelWOL,
		catalog.AskThePastor:                   ChannelWOL,
		catalog.CenterOfRelationshipExperience: ChannelWOL,
		catalog.CORE_HealthMatters:             ChannelWOL,
		catalog.CORE_HopeDealers:               ChannelWOL,
		catalog.CORE_RecoveryClasses:           ChannelWOL,
		catalog.CORE_CounselingClasses:         ChannelWOL,
		catalog.FaithAndFreedom:                ChannelFaithAndFreedom,
		catalog.TheBridgeOutreach:              ChannelTBO,
	}

	for ministry, channel := range expected {
		t.Equal(channel, getYouTubeChannel(ministry), "ministry %s", string(ministry))
	}

	// every ministry the catalog knows about must map somewhere
	for _, ministry := range catalog.AllMinistries {
		t.NotEqual(ChannelUnknown, getYouTubeChannel(ministry),
			"ministry %s has no channel", string(ministry))
	}
}

// +---------------------------------------------------------------------------
// | Playlist naming
// +---------------------------------------------------------------------------

func (t *PublishTestSuite) TestPlaylist_WOLSeriesIsUnprefixed() {
	msg := catalog.CatalogMessage{
		Ministry: catalog.WordOfLife,
		Series:   []catalog.SeriesReference{{Name: "Walking in Faith", Index: 3}},
	}
	seri := catalog.CatalogSeri{Name: "Walking in Faith"}

	t.Equal("Walking in Faith", getPlaylistName(&msg, &seri))
}

func (t *PublishTestSuite) TestPlaylist_CoreSeriesIsPrefixed() {
	msg := catalog.CatalogMessage{
		Ministry: catalog.CenterOfRelationshipExperience,
		Series:   []catalog.SeriesReference{{Name: "Boundaries", Index: 1}},
	}
	seri := catalog.CatalogSeri{Name: "Boundaries"}

	t.Equal("CORE: Boundaries", getPlaylistName(&msg, &seri))
}

func (t *PublishTestSuite) TestPlaylist_CoreSubMinistriesAreAlsoPrefixed() {
	// the prefix comes from the ministry family, NOT from Ministry.Description(),
	// which returns "C.O.R.E." for the parent but "CORE: Health Matters" for the subs
	subMinistries := []catalog.Ministry{
		catalog.CORE_HealthMatters,
		catalog.CORE_HopeDealers,
		catalog.CORE_RecoveryClasses,
		catalog.CORE_CounselingClasses,
	}

	for _, ministry := range subMinistries {
		msg := catalog.CatalogMessage{
			Ministry: ministry,
			Series:   []catalog.SeriesReference{{Name: "Eating Well", Index: 1}},
		}
		seri := catalog.CatalogSeri{Name: "Eating Well"}

		t.Equal("CORE: Eating Well", getPlaylistName(&msg, &seri),
			"ministry %s", string(ministry))
	}
}

func (t *PublishTestSuite) TestPlaylist_AskThePastorIsFixed() {
	// an ATP message goes in the ATP playlist even when it names another series
	msg := catalog.CatalogMessage{
		Ministry: catalog.AskThePastor,
		Series:   []catalog.SeriesReference{{Name: "Some Other Series", Index: 4}},
	}
	seri := catalog.CatalogSeri{Name: "Some Other Series"}

	t.Equal("Ask the Pastor", getPlaylistName(&msg, &seri))
}

func (t *PublishTestSuite) TestPlaylist_NoSeries() {
	msg := catalog.CatalogMessage{Ministry: catalog.WordOfLife}
	t.Equal("", getPlaylistName(&msg, nil))
}

// +---------------------------------------------------------------------------
// | Title
// +---------------------------------------------------------------------------

func (t *PublishTestSuite) TestTitle_Format() {
	msg := catalog.CatalogMessage{
		Name:     "Walking in Faith",
		Speakers: []string{"Pastor Vern Peltz"},
		Date:     catalog.MustParseDateOnly("2021-03-08"),
	}

	t.Equal("Walking in Faith | Pastor Vern Peltz | Mar 8, 2021", getUploadTitle(&msg))
}

func (t *PublishTestSuite) TestTitle_MultipleSpeakers() {
	msg := catalog.CatalogMessage{
		Name:     "Together",
		Speakers: []string{"Pastor Vern Peltz", "Pastor Mary Peltz"},
		Date:     catalog.MustParseDateOnly("2021-03-08"),
	}

	t.Equal("Together | Pastor Vern Peltz, Pastor Mary Peltz | Mar 8, 2021", getUploadTitle(&msg))
}

func (t *PublishTestSuite) TestTitle_NoSpeaker() {
	msg := catalog.CatalogMessage{
		Name: "Walking in Faith",
		Date: catalog.MustParseDateOnly("2021-03-08"),
	}

	t.Equal("Walking in Faith | Mar 8, 2021", getUploadTitle(&msg))
}

// +---------------------------------------------------------------------------
// | Description
// +---------------------------------------------------------------------------

func (t *PublishTestSuite) TestDescription_NoResources() {
	msg := catalog.CatalogMessage{Name: "MSG"}
	t.Equal("A short summary.", getUploadDescription(&msg, nil, "A short summary."))
}

func (t *PublishTestSuite) TestDescription_OneResource() {
	msg := catalog.CatalogMessage{
		Name:      "MSG",
		Resources: []catalog.OnlineResource{{URL: "https://x/notes.pdf", Name: "Sermon Notes"}},
	}

	result := getUploadDescription(&msg, nil, "A short summary.")
	t.Contains(result, "A short summary.")
	t.Contains(result, "Sermon Notes")
	t.Contains(result, "https://x/notes.pdf")
}

func (t *PublishTestSuite) TestDescription_ManyResourcesIncludingSeries() {
	msg := catalog.CatalogMessage{
		Name:      "MSG",
		Resources: []catalog.OnlineResource{{URL: "https://x/notes.pdf", Name: "Notes"}},
	}
	seri := catalog.CatalogSeri{
		Name:      "SERIES",
		Booklets:  []catalog.OnlineResource{{URL: "https://x/booklet.pdf", Name: "Booklet"}},
		Resources: []catalog.OnlineResource{{URL: "https://x/extra.pdf", Name: "Extra"}},
	}

	result := getUploadDescription(&msg, &seri, "Summary.")
	t.Contains(result, "https://x/notes.pdf")
	t.Contains(result, "https://x/booklet.pdf")
	t.Contains(result, "https://x/extra.pdf")
}

func (t *PublishTestSuite) TestDescription_DeduplicatesResources() {
	msg := catalog.CatalogMessage{
		Name:      "MSG",
		Resources: []catalog.OnlineResource{{URL: "https://x/same.pdf", Name: "Same"}},
	}
	seri := catalog.CatalogSeri{
		Name:     "SERIES",
		Booklets: []catalog.OnlineResource{{URL: "https://x/same.pdf", Name: "Same"}},
	}

	resources := collectResources(&msg, &seri)
	t.Len(resources, 1)
}

// +---------------------------------------------------------------------------
// | Packet assembly
// +---------------------------------------------------------------------------

func (t *PublishTestSuite) TestPacket_Public() {
	msg := catalog.CatalogMessage{
		Name:       "Walking in Faith",
		Speakers:   []string{"Pastor Vern Peltz"},
		Date:       catalog.MustParseDateOnly("2021-03-08"),
		Ministry:   catalog.WordOfLife,
		Visibility: catalog.Public,
		Series:     []catalog.SeriesReference{{Name: "Walking in Faith", Index: 3}},
	}
	seri := catalog.CatalogSeri{Name: "Walking in Faith"}

	packet, err := NewUploadPacket(&msg, &seri, "A summary.", "/path/thumb.jpg")
	t.NoError(err)

	t.Equal(ChannelWOL, packet.Channel)
	t.Equal("Walking in Faith | Pastor Vern Peltz | Mar 8, 2021", packet.Title)
	t.Equal("Walking in Faith", packet.Playlist)
	t.Equal(3, packet.Position)
	t.Equal(PrivacyPublic, packet.Privacy)
	t.Equal("/path/thumb.jpg", packet.Thumbnail)
}

func (t *PublishTestSuite) TestPacket_RawIsRefused() {
	msg := catalog.CatalogMessage{
		Name:       "Raw Stream",
		Ministry:   catalog.WordOfLife,
		Visibility: catalog.Raw,
	}

	packet, err := NewUploadPacket(&msg, nil, "A summary.", "")
	t.Nil(packet)
	t.Error(err)
	t.Contains(err.Error(), "livestream")
}

// +---------------------------------------------------------------------------
// | Message type and date inference
// +---------------------------------------------------------------------------

func (t *PublishTestSuite) TestTypeInference() {
	cases := map[string]catalog.MessageType{
		"2026-03-08-pv Opening Prayer.mp4":  catalog.Prayer,
		"2026-03-08-vp Opening Prayer.mp4":  catalog.Prayer,
		"2026-03-08p Opening Prayer.mp4":    catalog.Prayer,
		"2026-03-08-v Walking in Faith.mp4": catalog.Message,
		"2026-03-08 Walking in Faith-v.mp4": catalog.Message,
		"2026-03-08-m Something Useful.mp4": catalog.Message,
	}

	for name, expected := range cases {
		t.Equal(expected, getMessageTypeFromFileName(name), "file %q", name)
	}
}

func (t *PublishTestSuite) TestDateInference() {
	date, err := getDateFromFileName("2026-03-08-v Walking in Faith.mp4")
	t.NoError(err)
	t.Equal("2026-03-08", date.String())
}

func (t *PublishTestSuite) TestDateInference_NoDate() {
	_, err := getDateFromFileName("Walking in Faith.mp4")
	t.Error(err)
	t.Contains(err.Error(), "2025-03-09")
}
