package cmd

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/WordOfLifeMN/online/catalog"
)

// prayerPatterns match the file naming conventions that mark a prayer rather than a
// message. The "p" sits next to the date, in one of the positions the speaker-initial
// conventions already allow:
//
//	2025-03-09-pv Title.mp4    prayer, Vern
//	2025-03-09-vp Title.mp4    prayer, Vern
//	2025-03-09p Title.mp4      prayer
var prayerPatterns = []*regexp.Regexp{
	regexp.MustCompile(`-[0-9][0-9]-[0-9][0-9]-P[A-Z] `),
	regexp.MustCompile(`-[0-9][0-9]-[0-9][0-9]-[A-Z]P `),
	regexp.MustCompile(`-[0-9][0-9]-[0-9][0-9]P`),
}

// datePattern extracts the leading ISO date from a file name
var datePattern = regexp.MustCompile(`([0-9]{4}-[0-9]{2}-[0-9]{2})`)

// getMessageTypeFromFileName infers the message type from the video file name.
//
// The sheet lookup keys on date and type, so the type has to come from somewhere; the
// existing naming conventions already encode the only distinction that matters in
// practice, which is prayer vs. message. Anything else needs --type.
func getMessageTypeFromFileName(filePath string) catalog.MessageType {
	name := strings.ToUpper(filepath.Base(filePath))
	for _, pattern := range prayerPatterns {
		if pattern.MatchString(name) {
			return catalog.Prayer
		}
	}
	return catalog.Message
}

// leadingMarkers matches what sits between the date and the title: an optional hyphen
// and up to two letters of speaker initial and/or prayer marker, then the gap before
// the title. The letter run is optional, so a plain "2026-10-04 Title" works too.
//
// Anchored, so it can only ever eat the marker. A title beginning with a short word -
// "2026-10-04-v A New Day" - keeps that word, because the letters have already been
// consumed by the marker before the gap is matched.
var leadingMarkers = regexp.MustCompile(`^-?[A-Za-z]{0,2}\s+`)

// trailingInitial matches the other convention, where the initial goes on the end:
// "2026-10-04 Title-v.mp4".
//
// It requires the hyphen. Allowing a bare space would amputate titles that genuinely
// end in a short word - "The Great I Am" would become "The Great I".
var trailingInitial = regexp.MustCompile(`-[A-Za-z]{1,2}$`)

// matchNoise is everything punctuation-ish, flattened to a space before comparing. The
// two sides punctuate differently for reasons that have nothing to do with meaning:
// a colon is illegal in a Windows file name, and "Q&A" is written "QA" or "Q and A"
// depending on who typed it.
var matchNoise = regexp.MustCompile(`[^a-z0-9]+`)

// getTitleFromFileName extracts the human title from a video file name, discarding the
// date, the speaker and prayer markers, and the extension.
//
// Returns "" when there is nothing but markers to work with, which callers must treat
// as "no opinion" rather than as an empty title that matches nothing.
func getTitleFromFileName(filePath string) string {
	name := filepath.Base(filePath)
	name = strings.TrimSuffix(name, filepath.Ext(name))

	// everything after the date is the title, give or take the markers
	if loc := datePattern.FindStringIndex(name); loc != nil {
		name = name[loc[1]:]
	}

	name = leadingMarkers.ReplaceAllString(name, "")
	name = trailingInitial.ReplaceAllString(name, "")

	return strings.TrimSpace(name)
}

// normalizeForMatch reduces a title to the part worth comparing: lower case, no
// punctuation, single spaces. Applied to both sides so neither is penalised for a
// house style the other does not share.
func normalizeForMatch(title string) string {
	title = matchNoise.ReplaceAllString(strings.ToLower(title), " ")
	return strings.TrimSpace(strings.Join(strings.Fields(title), " "))
}

// getDateFromFileName extracts the message date from the video file name
func getDateFromFileName(filePath string) (catalog.DateOnly, error) {
	match := datePattern.FindStringSubmatch(filepath.Base(filePath))
	if match == nil {
		return catalog.DateOnly{}, fmt.Errorf(
			"cannot find a date in the file name %q. expected something like 2025-03-09",
			filepath.Base(filePath))
	}
	return catalog.ParseDateOnly(match[1])
}
