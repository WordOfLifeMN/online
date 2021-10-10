package catalog

import (
	"strings"
)

// ministries
type Ministry string

const (
	UnknownMinistry                Ministry = "unknown"
	WordOfLife                     Ministry = "wol"
	CenterOfRelationshipExperience Ministry = "core"
	TheBridgeOutreach              Ministry = "tbo"
	AskThePastor                   Ministry = "ask-pastor"
	FaithAndFreedom                Ministry = "faith-freedom"
)

func NewMinistryFromString(s string) Ministry {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)

	switch s {
	case "wol":
		return WordOfLife
	case "core":
		return CenterOfRelationshipExperience
	case "tbo":
		return TheBridgeOutreach
	case "ask pastor", "ask the pastor", "askthepastor", string(AskThePastor):
		return AskThePastor
	case "faith-freedom", "faithandfreedom":
		return FaithAndFreedom
	}

	// log.Printf("WARNING: Encountered unknown ministry '%s'", s)
	return UnknownMinistry
}

func (ministry Ministry) String() string {
	switch ministry {
	case WordOfLife:
		return "Word of Life"
	case CenterOfRelationshipExperience:
		return "C.O.R.E."
	case TheBridgeOutreach:
		return "The Bridge Outreach"
	case AskThePastor:
		return "Ask the Pastor"
	case FaithAndFreedom:
		return "Faith & Freedom"
	default:
		return "(Unknown Ministry)"
	}
}
