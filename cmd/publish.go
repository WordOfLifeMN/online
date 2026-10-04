package cmd

import (
	"fmt"
	"strings"

	"github.com/WordOfLifeMN/online/catalog"
)

// YouTubeChannel identifies which of the three channels a message belongs on
type YouTubeChannel string

const (
	ChannelWOL             YouTubeChannel = "WOL"
	ChannelFaithAndFreedom YouTubeChannel = "Faith & Freedom"
	ChannelTBO             YouTubeChannel = "TBO"
	ChannelUnknown         YouTubeChannel = ""
)

// YouTubePrivacy is the privacy setting a video is published with
type YouTubePrivacy string

const (
	PrivacyPublic   YouTubePrivacy = "public"
	PrivacyUnlisted YouTubePrivacy = "unlisted"
)

// corePlaylistPrefix distinguishes CORE series from WOL series, which otherwise share
// the WOL channel and both use the bare series title as the playlist name.
//
// This is a convention invented for the new website, and it only works if the site can
// filter playlists on it. It is deliberately referenced in exactly one place
// (getPlaylistName) so that a different answer from the website developer costs one
// function rather than a search-and-replace.
const corePlaylistPrefix = "CORE: "

// askThePastorPlaylist is the single playlist that every Ask the Pastor message lives
// in, regardless of any series it names
const askThePastorPlaylist = "Ask the Pastor"

// UploadPacket is everything needed to publish one message to YouTube by hand.
//
// Upload is manual for now: the operator pastes these fields into YouTube Studio. The
// packet is a struct rather than a pile of printed strings so that automating the
// upload later means serializing it to the YouTube Data API instead of to the console.
type UploadPacket struct {
	Channel     YouTubeChannel
	Title       string
	Description string
	Playlist    string
	Position    int
	Privacy     YouTubePrivacy
	Thumbnail   string
}

// NewUploadPacket assembles the packet for a message.
//
// Returns an error for raw footage: raw is the livestream, which is already on YouTube
// as private from the moment it streams and is never downloaded or edited. Reaching
// this code with a raw message means something is wrong upstream.
func NewUploadPacket(
	msg *catalog.CatalogMessage,
	seri *catalog.CatalogSeri,
	summary string,
	thumbnailPath string,
) (*UploadPacket, error) {
	if msg == nil {
		return nil, fmt.Errorf("cannot build an upload packet without a message")
	}

	privacy, err := getYouTubePrivacy(msg.Visibility)
	if err != nil {
		return nil, err
	}

	packet := UploadPacket{
		Channel:     getYouTubeChannel(msg.Ministry),
		Title:       getUploadTitle(msg),
		Description: getUploadDescription(msg, seri, summary),
		Playlist:    getPlaylistName(msg, seri),
		Position:    getPlaylistPosition(msg),
		Privacy:     privacy,
		Thumbnail:   thumbnailPath,
	}

	return &packet, nil
}

// getYouTubePrivacy maps the catalog's four-valued visibility onto YouTube's privacy
// settings. Only two values are reachable: partner and private are not distinguishable
// on YouTube, which matches what the old site did in practice.
func getYouTubePrivacy(view catalog.View) (YouTubePrivacy, error) {
	switch view {
	case catalog.Public:
		return PrivacyPublic, nil
	case catalog.Partner, catalog.Private:
		return PrivacyUnlisted, nil
	case catalog.Raw:
		return "", fmt.Errorf(
			"refusing to build an upload packet for raw footage: raw is the livestream, " +
				"which is already on YouTube as private. point the spreadsheet at the " +
				"existing stream instead of uploading an edited copy")
	}
	return "", fmt.Errorf("cannot publish a message with visibility %q", string(view))
}

// getYouTubeChannel maps a ministry to the channel that hosts it. The WOL channel
// hosts three ministries; the other two have a channel each.
func getYouTubeChannel(ministry catalog.Ministry) YouTubeChannel {
	switch ministry {
	case catalog.WordOfLife,
		catalog.AskThePastor,
		catalog.CenterOfRelationshipExperience,
		catalog.CORE_HealthMatters,
		catalog.CORE_HopeDealers,
		catalog.CORE_RecoveryClasses,
		catalog.CORE_CounselingClasses:
		return ChannelWOL
	case catalog.FaithAndFreedom:
		return ChannelFaithAndFreedom
	case catalog.TheBridgeOutreach:
		return ChannelTBO
	}
	return ChannelUnknown
}

// isCoreMinistry reports whether a ministry belongs to the CORE family. This is
// deliberately not derived from Ministry.Description(), which returns "C.O.R.E." for
// the parent ministry but "CORE: Health Matters" for the sub-ministries - reusing it
// would produce two different playlist prefixes.
func isCoreMinistry(ministry catalog.Ministry) bool {
	switch ministry {
	case catalog.CenterOfRelationshipExperience,
		catalog.CORE_HealthMatters,
		catalog.CORE_HopeDealers,
		catalog.CORE_RecoveryClasses,
		catalog.CORE_CounselingClasses:
		return true
	}
	return false
}

// getPlaylistName determines the YouTube playlist a message belongs in.
//
// This is the single place the CORE naming convention lives.
func getPlaylistName(msg *catalog.CatalogMessage, seri *catalog.CatalogSeri) string {
	// Ask the Pastor messages all share one playlist, whatever series they name
	if msg.Ministry == catalog.AskThePastor {
		return askThePastorPlaylist
	}

	name := ""
	if seri != nil {
		name = seri.Name
	} else if len(msg.Series) > 0 {
		name = msg.Series[0].Name
	}
	if name == "" {
		return ""
	}

	if isCoreMinistry(msg.Ministry) {
		return corePlaylistPrefix + name
	}
	return name
}

// getPlaylistPosition returns the message's track number within its series, or 0 if it
// has none
func getPlaylistPosition(msg *catalog.CatalogMessage) int {
	if len(msg.Series) == 0 {
		return 0
	}
	return msg.Series[0].Index
}

// titleDateFormat is how the date appears in a YouTube title. The house style is the
// full month name - "October 2, 2026", not "Oct 2, 2026".
const titleDateFormat = "January 2, 2006"

// getUploadTitle builds the YouTube title: "Name | Speaker | Date"
//
// The date is formatted here rather than through CatalogMessage.DateString, which
// prefixes "Scheduled for " on a future date. That reads sensibly on a web page
// listing upcoming messages, but it has no place in the title of a video that is being
// uploaded right now - and a message recorded ahead of its service date would
// otherwise be titled "... | Scheduled for October 2, 2026".
func getUploadTitle(msg *catalog.CatalogMessage) string {
	parts := []string{msg.Name}
	if speakers := getTitleSpeakers(msg); speakers != "" {
		parts = append(parts, speakers)
	}
	if !msg.Date.IsZero() {
		parts = append(parts, msg.Date.Time.Format(titleDateFormat))
	}
	return strings.Join(parts, " | ")
}

// speakerTitles are honorifics stripped from the speaker name in the YouTube title.
//
// This is about title length. YouTube truncates titles in search results, in the
// sidebar and on mobile, and "Pastor " costs seven characters of a budget the message
// name and the date have the stronger claim on. The description is not length
// constrained, so it keeps the honorific - see getUploadDescription.
//
// A list of one today. Extend it rather than reaching for a general honorific matcher:
// over-matching silently renames a real person in a published video title.
var speakerTitles = []string{"Pastor"}

// stripSpeakerTitle removes a leading honorific from one speaker's name
func stripSpeakerTitle(speaker string) string {
	speaker = strings.TrimSpace(speaker)

	for _, title := range speakerTitles {
		prefix := title + " "
		// the length test leaves a speaker recorded as bare "Pastor" alone rather than
		// reducing them to an empty name
		if len(speaker) > len(prefix) && strings.EqualFold(speaker[:len(prefix)], prefix) {
			return strings.TrimSpace(speaker[len(prefix):])
		}
	}

	return speaker
}

// getTitleSpeakers renders the speakers for the YouTube title, without honorifics.
// Deliberately not CatalogMessage.SpeakerString, which keeps them for the description.
func getTitleSpeakers(msg *catalog.CatalogMessage) string {
	names := make([]string, 0, len(msg.Speakers))
	for _, speaker := range msg.Speakers {
		if name := stripSpeakerTitle(speaker); name != "" {
			names = append(names, name)
		}
	}

	return strings.Join(names, ", ")
}

// getUploadDescription builds the YouTube description: the generated summary followed
// by one link per resource attached to the message or its series
func getUploadDescription(
	msg *catalog.CatalogMessage,
	seri *catalog.CatalogSeri,
	summary string,
) string {
	var b strings.Builder

	if summary != "" {
		b.WriteString(summary)
	} else if msg.Description != "" {
		b.WriteString(msg.Description)
	}

	for _, resource := range collectResources(msg, seri) {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		name := resource.Name
		if name == "" {
			name = resource.GetNameFromURL()
		}
		fmt.Fprintf(&b, "\n%s\n%s", name, resource.URL)
	}

	return b.String()
}

// collectResources gathers the resources belonging to a message and its series,
// skipping duplicates by URL. Message resources come first.
func collectResources(
	msg *catalog.CatalogMessage,
	seri *catalog.CatalogSeri,
) []catalog.OnlineResource {
	var resources []catalog.OnlineResource
	seen := map[string]bool{}

	add := func(list []catalog.OnlineResource) {
		for _, resource := range list {
			if resource.URL == "" || seen[resource.URL] {
				continue
			}
			seen[resource.URL] = true
			resources = append(resources, resource)
		}
	}

	add(msg.Resources)
	if seri != nil {
		add(seri.Booklets)
		add(seri.Resources)
	}

	return resources
}

// Print writes the packet so that each field can be selected and copied in one
// gesture. The label sits outside the fence and the bare value inside it, so a
// selection between the delimiters contains exactly the value and nothing else.
func (p *UploadPacket) Print() {
	const open = "╭───────────────────────────────────────────────────────────────────────────────────┄┄"
	const close = "╰───────────────────────────────────────────────────────────────────────────────────┄┄"

	fmt.Printf("\nUPLOAD PACKET\n")
	fmt.Printf("Channel : %s\n", p.Channel)
	fmt.Printf("Privacy : %s\n", p.Privacy)
	if p.Playlist != "" {
		fmt.Printf("Playlist: %s", p.Playlist)
		if p.Position > 0 {
			fmt.Printf("  (position %d)", p.Position)
		}
		fmt.Println()
	}
	if p.Thumbnail != "" {
		fmt.Printf("Thumb   : %s\n", p.Thumbnail)
	}

	fmt.Printf("\n%s\n│ Title\n%s\n%s\n", open, p.Title, close)
	fmt.Printf("%s\n│ Description\n%s\n%s\n", open, p.Description, close)
	if p.Playlist != "" {
		fmt.Printf("%s\n│ Playlist\n%s\n%s\n", open, p.Playlist, close)
	}
}
