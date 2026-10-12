package cmd

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/WordOfLifeMN/online/catalog"
)

// datePattern extracts the leading ISO date from a file name
var datePattern = regexp.MustCompile(`([0-9]{4}-[0-9]{2}-[0-9]{2})`)

// The file name once supplied a message type, from a "p" next to the date, and a
// speaker, from an initial. Neither is read any more. The spreadsheet row is the record
// of what a message is and who preached it, and a second answer from the file name
// could only agree redundantly or disagree silently - which is what it did.
//
// The date is now the only part of a file name that means anything. The rest is
// tolerated, not interpreted:
//
//	<date>[<discriminator>][-<obsolete markup>] <title>.<ext>
//
//	2026-10-11 Servanthood.mp4
//	2026-10-11b-mp Learning at Moriah.mp4
//	2026-10-04-v The Works of God.mp4
//	2026-10-04 The Works of God-v.mp4
//
// The discriminator is a single letter that keeps two files of one date apart on disk.
// It is for the operator's filing, not for this program - which is exactly what the
// markup after it has now become too.

// leadingMarkers matches what sits between the date and the title: an optional
// discriminator letter, an optional hyphen and run of obsolete markup letters, then the
// gap before the title. Every part is optional, so a plain "2026-10-04 Title" works.
//
// Anchored, so it can only ever eat the markers. A title beginning with a short word -
// "2026-10-04-v A New Day" - keeps that word, because the gap is what ends the match
// and the letters before it have already been consumed.
var leadingMarkers = regexp.MustCompile(`^[A-Za-z]?(-[A-Za-z]+)?\s+`)

// trailingInitial matches the other convention, where the markup goes on the end:
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
