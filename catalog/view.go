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
