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
	CORE_HealthMatters             Ministry = "core_health"
	CORE_HopeDealers               Ministry = "core_hope"
	CORE_RecoveryClasses           Ministry = "core_recovery"
	CORE_CounselingClasses         Ministry = "core_counseling"
	TheBridgeOutreach              Ministry = "tbo"
	AskThePastor                   Ministry = "ask-pastor"
	FaithAndFreedom                Ministry = "faith-freedom"
)

var AllMinistries = []Ministry{WordOfLife, TheBridgeOutreach,
	CenterOfRelationshipExperience, CORE_HealthMatters, CORE_HopeDealers, CORE_RecoveryClasses, CORE_CounselingClasses,
	AskThePastor,
	FaithAndFreedom}

func NewMinistryFromString(s string) Ministry {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)

	switch s {
	case "wol":
		return WordOfLife
	case "core":
		return CenterOfRelationshipExperience
	case "core:health", "core:healthmatters", "core: health", "core: health matters":
		return CORE_HealthMatters
	case "core:hope", "core:hopedealers", "core: hope", "core: hope dealers":
		return CORE_HopeDealers
	case "core:recovery", "core:recoveryclasses", "core: recovery", "core: recovery classes":
		return CORE_RecoveryClasses
	case "core:counseling", "core:counselingclasses", "core: counseling", "core: counseling classes":
		return CORE_CounselingClasses
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

func (ministry Ministry) Description() string {
	switch ministry {
	case WordOfLife:
		return "Word of Life"
	case CenterOfRelationshipExperience:
		return "C.O.R.E."
	case CORE_HealthMatters:
		return "CORE: Health Matters"
	case CORE_HopeDealers:
		return "CORE: Hope Dealers"
	case CORE_RecoveryClasses:
		return "CORE: Recovery Classes"
	case CORE_CounselingClasses:
		return "CORE: Counseling Classes"
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
