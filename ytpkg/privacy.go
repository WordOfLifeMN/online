package ytpkg

import (
	"fmt"

	"github.com/WordOfLifeMN/online/catalog"
)

// Privacy is a YouTube video or playlist privacy setting
type Privacy string

const (
	PrivacyPublic   Privacy = "public"
	PrivacyUnlisted Privacy = "unlisted"
	PrivacyPrivate  Privacy = "private"
)

// PrivacyForView maps the catalog's visibility onto the privacy a new item is given, or
// that an existing one is corrected to when its live privacy contradicts the sheet.
// Private and partner both become unlisted. Raw is refused.
func PrivacyForView(view catalog.View) (Privacy, error) {
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

// IsPrivacyConsistent reports whether a live privacy already agrees with the sheet, and
// so should be kept rather than corrected:
//
//	sheet public  <-> YouTube public
//	sheet private <-> YouTube private or unlisted
//	sheet partner <-> YouTube unlisted
//
// A video someone deliberately made private for a private message stays private.
func IsPrivacyConsistent(view catalog.View, live Privacy) bool {
	switch view {
	case catalog.Public:
		return live == PrivacyPublic
	case catalog.Private:
		return live == PrivacyPrivate || live == PrivacyUnlisted
	case catalog.Partner:
		return live == PrivacyUnlisted
	}
	return false
}

// ResolvePrivacy returns the privacy an item should have: its live privacy when that is
// consistent with the sheet, otherwise the mapping for a new item. An empty live
// privacy means a new item.
func ResolvePrivacy(view catalog.View, live Privacy) (Privacy, error) {
	want, err := PrivacyForView(view)
	if err != nil {
		return "", err
	}
	if live != "" && IsPrivacyConsistent(view, live) {
		return live, nil
	}
	return want, nil
}
