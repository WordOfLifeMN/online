package catalog

import "strings"

// visibility of series and messages
type View string

const (
	UnknownView View = "unknown" // undetermined view
	Raw         View = "raw"     // unedited, raw footage - assumed to be private
	Public      View = "public"  // available to anyone
	Partner     View = "partner" // available to covenant partners
	Private     View = "private" // not to be displayed online to anyone
)

// describes whether the visibility level (1st param) is visible at the specified view (2nd
// param). So visibilityMap[Public][Private] is true (a public message is visible in a private
// view), but visibilityMap[Private][Public] is false (a private message is not visible in a
// public view)
var visibilityMap = map[View]map[View]bool{
	UnknownView: {
		UnknownView: true,
		Raw:         false,
		Public:      false,
		Partner:     false,
		Private:     false,
	},
	Raw: {
		UnknownView: false,
		Raw:         true,
		Public:      false,
		Partner:     false,
		Private:     false,
	},
	Public: {
		UnknownView: false,
		Raw:         true,
		Public:      true,
		Partner:     true,
		Private:     true,
	},
	Partner: {
		UnknownView: false,
		Raw:         true,
		Public:      false,
		Partner:     true,
		Private:     true,
	},
	Private: {
		UnknownView: false,
		Raw:         true,
		Public:      false,
		Partner:     false,
		Private:     true,
	},
}

func NewViewFromString(s string) View {
	switch strings.ToLower(s) {
	case "public":
		return Public
	case "protected":
		return Partner
	case "partner":
		return Partner
	case "private":
		return Private
	case "raw":
		return Raw
	}
	return UnknownView
}

// IsVisibleInView determines if the specified visibility should be visible at a specific view
// level
func IsVisibleInView(visibility View, view View) bool {
	return visibilityMap[visibility][view]
}
