package ytpkg

import (
	"testing"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/stretchr/testify/assert"
)

func TestChannelForMinistry(t *testing.T) {
	expected := map[catalog.Ministry]Channel{
		catalog.WordOfLife:                     ChannelWOL,
		catalog.AskThePastor:                   ChannelWOL,
		catalog.CenterOfRelationshipExperience: ChannelWOL,
		catalog.CORE_HealthMatters:             ChannelWOL,
		catalog.CORE_HopeDealers:               ChannelWOL,
		catalog.CORE_RecoveryClasses:           ChannelWOL,
		catalog.CORE_CounselingClasses:         ChannelWOL,
		catalog.FaithAndFreedom:                ChannelFaithAndFreedom,
		catalog.TheBridgeOutreach:              ChannelTBO,
	}

	for ministry, channel := range expected {
		assert.Equal(t, channel, ChannelForMinistry(ministry), "ministry %s", string(ministry))
	}

	// every ministry the catalog knows about must map somewhere
	for _, ministry := range catalog.AllMinistries {
		assert.NotEqual(t, ChannelUnknown, ChannelForMinistry(ministry),
			"ministry %s has no channel", string(ministry))
	}
	assert.Equal(t, ChannelUnknown, ChannelForMinistry(catalog.UnknownMinistry))
}

func TestIsCoreMinistry(t *testing.T) {
	for _, ministry := range []catalog.Ministry{
		catalog.CenterOfRelationshipExperience,
		catalog.CORE_HealthMatters,
		catalog.CORE_HopeDealers,
		catalog.CORE_RecoveryClasses,
		catalog.CORE_CounselingClasses,
	} {
		assert.True(t, IsCoreMinistry(ministry), "ministry %s", string(ministry))
	}

	for _, ministry := range []catalog.Ministry{
		catalog.WordOfLife,
		catalog.AskThePastor,
		catalog.FaithAndFreedom,
		catalog.TheBridgeOutreach,
	} {
		assert.False(t, IsCoreMinistry(ministry), "ministry %s", string(ministry))
	}
}

func TestChannelKey(t *testing.T) {
	assert.Equal(t, "wol", ChannelWOL.Key())
	assert.Equal(t, "faith-freedom", ChannelFaithAndFreedom.Key())
	assert.Equal(t, "tbo", ChannelTBO.Key())
	assert.Equal(t, "", ChannelUnknown.Key())
}
