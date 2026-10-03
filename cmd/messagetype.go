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
