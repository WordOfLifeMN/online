package ytpkg

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/util"
)

// This file is the single place that decides how anything reaching YouTube is worded:
// titles, descriptions, playlist names and the house defaults. Nothing else in the
// codebase builds these strings, so a per-ministry format, or new description metadata,
// is a change to this file only.

// +---------------------------------------------------------------------------
// | House defaults
// +---------------------------------------------------------------------------

const (
	// DefaultCategoryID is "Nonprofits & Activism"
	DefaultCategoryID = "29"

	// DefaultLanguage is both the metadata language and the audio language
	DefaultLanguage = "en"

	// DefaultMadeForKids is false: services are not made for children, and saying
	// they are turns off comments and personalised features
	DefaultMadeForKids = false
)

// +---------------------------------------------------------------------------
// | Titles
// +---------------------------------------------------------------------------

// titleSeparator joins the components of a video title and of a CORE playlist title
const titleSeparator = " | "

// coreMarker leads every CORE title and playlist, so that CORE content can be told
// apart from WOL content sharing the same channel. It matches the live channel after
// the 2026-10-12 retitle.
const coreMarker = "CORE"

// askThePastorPlaylist is the single playlist that every Ask the Pastor message lives
// in, regardless of any series it names
const askThePastorPlaylist = "Ask the Pastor"

// titleDateFormat is how the date appears in a YouTube title. The house style is the
// full month name - "October 2, 2026", not "Oct 2, 2026".
const titleDateFormat = "January 2, 2006"

// FormatTitle assembles a video title around a name:
//
//	[CORE | ]Name | Date
//
// The speaker is not in the title. The generated description names them in full, and
// a title without them leaves more of YouTube's truncated title for the message's
// name. Titles uploaded before 2026-10-17 do carry the speaker, so sync proposes a new
// title for those.
//
// The date is formatted here rather than through CatalogMessage.DateString, which
// prefixes "Scheduled for " on a future date. That reads sensibly on a web page
// listing upcoming messages, but a message recorded ahead of its service date would
// otherwise be titled "... | Scheduled for October 2, 2026".
func FormatTitle(msg *catalog.CatalogMessage, name string) string {
	var parts []string
	if IsCoreMinistry(msg.Ministry) {
		parts = append(parts, coreMarker)
	}
	parts = append(parts, name)
	if !msg.Date.IsZero() {
		parts = append(parts, msg.Date.Time.Format(titleDateFormat))
	}
	return strings.Join(parts, titleSeparator)
}

// AlternativeTitle renders the generated title in the same shape as the real one, so
// the two can be read against each other and either can be pasted.
//
// Returns "" when there is nothing worth showing: no generated title, or one that says
// the same thing as the sheet's. "Walking In Faith" and "Walking in Faith" are the same
// title, and offering a choice between them teaches the operator that this line is
// noise. The comparison ignores case and punctuation for that reason, which is the same
// thing util.NormalizeForMatch does when ranking a file name against the sheet.
//
// Only the name is compared. The date comes from the row either way, so it is
// identical on both lines by construction and would dilute the comparison.
func AlternativeTitle(msg *catalog.CatalogMessage, generated string) string {
	generated = strings.TrimSpace(generated)
	if generated == "" {
		return ""
	}
	if util.NormalizeForMatch(generated) == util.NormalizeForMatch(msg.Name) {
		return ""
	}
	return FormatTitle(msg, generated)
}

// PlaylistTitle names the playlist for a series: the bare series name, or
// "CORE | <series>" for the CORE family. Ask the Pastor is not a series playlist, and
// is handled by the caller through AskThePastorPlaylistTitle.
func PlaylistTitle(ministry catalog.Ministry, seriesName string) string {
	if IsCoreMinistry(ministry) {
		return coreMarker + titleSeparator + seriesName
	}
	return seriesName
}

// AskThePastorPlaylistTitle is the one playlist every Ask the Pastor message goes in
func AskThePastorPlaylistTitle() string {
	return askThePastorPlaylist
}

// +---------------------------------------------------------------------------
// | Descriptions
// +---------------------------------------------------------------------------

// MessageDescription builds a video description: the generated summary (or, without
// one, the sheet's description), a blank line, then one name and URL pair per resource
// attached to the message or its series
func MessageDescription(
	msg *catalog.CatalogMessage,
	seri *catalog.CatalogSeri,
	summary string,
) string {
	text := summary
	if text == "" {
		text = msg.Description
	}
	return withResources(text, collectResources(msg, seri))
}

// SeriesDescription builds a playlist description: the series description followed by
// its booklets, laid out the same way as a video description
func SeriesDescription(seri *catalog.CatalogSeri) string {
	if seri == nil {
		return ""
	}
	return withResources(seri.Description, uniqueResources(seri.Booklets))
}

// withResources appends one "name\nURL" block per resource, each separated from what
// precedes it by a blank line
func withResources(text string, resources []catalog.OnlineResource) string {
	var b strings.Builder
	b.WriteString(text)

	for _, resource := range resources {
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
//
// Only the series' booklets are series-wide. CatalogSeri.Resources is NOT the
// series' own resource list - Normalize() clears it and refills it with the
// resources of every message in the series, so including it here would put the
// sermon notes for all eight messages in the description of message eight. A
// Series row's own resources are parsed into Booklets, not Resources.
func collectResources(
	msg *catalog.CatalogMessage,
	seri *catalog.CatalogSeri,
) []catalog.OnlineResource {
	list := append([]catalog.OnlineResource{}, msg.Resources...)
	if seri != nil {
		list = append(list, seri.Booklets...)
	}
	return uniqueResources(list)
}

// uniqueResources drops resources with no URL and repeats of a URL already seen
func uniqueResources(list []catalog.OnlineResource) []catalog.OnlineResource {
	var resources []catalog.OnlineResource
	seen := map[string]bool{}

	for _, resource := range list {
		if resource.URL == "" || seen[resource.URL] {
			continue
		}
		seen[resource.URL] = true
		resources = append(resources, resource)
	}

	return resources
}

// +---------------------------------------------------------------------------
// | Validation
// +---------------------------------------------------------------------------

const (
	// maxTitleChars is YouTube's limit on a title, in characters
	maxTitleChars = 100

	// maxDescriptionBytes is YouTube's limit on a description, in bytes
	maxDescriptionBytes = 5000
)

// Validate checks a title and description against YouTube's limits before anything is
// written: a non-empty title of at most 100 characters, a description of at most 5000
// bytes, and no angle brackets in either, which YouTube rejects outright.
func Validate(title, description string) error {
	var problems []string

	if strings.TrimSpace(title) == "" {
		problems = append(problems, "the title is empty")
	}
	if n := utf8.RuneCountInString(title); n > maxTitleChars {
		problems = append(problems, fmt.Sprintf(
			"the title is %d characters, over YouTube's limit of %d", n, maxTitleChars))
	}
	if strings.ContainsAny(title, "<>") {
		problems = append(problems, "the title contains < or >, which YouTube does not allow")
	}

	problems = append(problems, descriptionProblems(description)...)

	if len(problems) > 0 {
		return fmt.Errorf("cannot write to YouTube: %s", strings.Join(problems, "; "))
	}
	return nil
}

// ValidateDescription checks a description alone, as for a playlist, whose title has
// limits of its own
func ValidateDescription(description string) error {
	if problems := descriptionProblems(description); len(problems) > 0 {
		return fmt.Errorf("cannot write to YouTube: %s", strings.Join(problems, "; "))
	}
	return nil
}

// descriptionProblems lists what YouTube would refuse in a description
func descriptionProblems(description string) []string {
	var problems []string
	if n := len(description); n > maxDescriptionBytes {
		problems = append(problems, fmt.Sprintf(
			"the description is %d bytes, over YouTube's limit of %d", n, maxDescriptionBytes))
	}
	if strings.ContainsAny(description, "<>") {
		problems = append(problems, "the description contains < or >, which YouTube does not allow")
	}
	return problems
}
