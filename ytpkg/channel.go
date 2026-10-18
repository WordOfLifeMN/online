// Package ytpkg is the ytPackage: one description of a YouTube video (Message) or
// playlist (Series), whether it was read from YouTube or built from the spreadsheet.
//
// Nothing here touches the network. Every string that reaches YouTube is built in
// format.go, so that a change to the house style is a change to that one file.
package ytpkg

import "github.com/WordOfLifeMN/online/catalog"

// Channel identifies which of the three channels a message belongs on
type Channel string

const (
	ChannelWOL             Channel = "WOL"
	ChannelFaithAndFreedom Channel = "Faith & Freedom"
	ChannelTBO             Channel = "TBO"
	ChannelUnknown         Channel = ""
)

// ChannelForMinistry maps a ministry to the channel that hosts it. The WOL channel
// hosts three ministries; the other two have a channel each.
func ChannelForMinistry(ministry catalog.Ministry) Channel {
	switch ministry {
	case catalog.WordOfLife,
		catalog.AskThePastor,
		catalog.CenterOfRelationshipExperience,
		catalog.CORE_HealthMatters,
		catalog.CORE_HopeDealers,
		catalog.CORE_RecoveryClasses,
		catalog.CORE_CounselingClasses:
		return ChannelWOL
	case catalog.FaithAndFreedom:
		return ChannelFaithAndFreedom
	case catalog.TheBridgeOutreach:
		return ChannelTBO
	}
	return ChannelUnknown
}

// IsCoreMinistry reports whether a ministry belongs to the CORE family. This is
// deliberately not derived from Ministry.Description(), which returns "C.O.R.E." for
// the parent ministry but "CORE: Health Matters" for the sub-ministries - reusing it
// would produce two different markers.
func IsCoreMinistry(ministry catalog.Ministry) bool {
	switch ministry {
	case catalog.CenterOfRelationshipExperience,
		catalog.CORE_HealthMatters,
		catalog.CORE_HopeDealers,
		catalog.CORE_RecoveryClasses,
		catalog.CORE_CounselingClasses:
		return true
	}
	return false
}

// Key is the channel's short lower-case name, used for its token file and its entry in
// the youtube-channel-ids configuration: "wol", "faith-freedom", "tbo"
func (c Channel) Key() string {
	switch c {
	case ChannelWOL:
		return "wol"
	case ChannelFaithAndFreedom:
		return "faith-freedom"
	case ChannelTBO:
		return "tbo"
	}
	return ""
}
