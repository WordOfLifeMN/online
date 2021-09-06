package catalog

import "strings"

// visibility of series and messages
type View string

const (
	Raw     View = "raw"     // undetermined or unedited
	Public  View = "public"  // available to anyone
	Partner View = "partner" // available to covenant partners
	Private View = "private" // not to be displayed online to anyone
)

func NewViewFromString(s string) View {
	switch strings.ToLower(s) {
	case "public":
		return Public
	case "protected":
	case "partner":
		return Partner
	case "private":
		return Private
	}
	return Raw
}
