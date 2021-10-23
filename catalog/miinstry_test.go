package catalog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsing(t *testing.T) {
	assert.Equal(t, WordOfLife, NewMinistryFromString("wol"))
	assert.Equal(t, WordOfLife, NewMinistryFromString("WOL"))

	assert.Equal(t, CenterOfRelationshipExperience, NewMinistryFromString("core"))
	assert.Equal(t, CenterOfRelationshipExperience, NewMinistryFromString("CORE"))

	assert.Equal(t, TheBridgeOutreach, NewMinistryFromString("tbo"))
	assert.Equal(t, TheBridgeOutreach, NewMinistryFromString("TBO"))

	assert.Equal(t, AskThePastor, NewMinistryFromString("ask pastor"))
	assert.Equal(t, AskThePastor, NewMinistryFromString("askthepastor"))
	assert.Equal(t, AskThePastor, NewMinistryFromString("AskThePastor"))

	assert.Equal(t, FaithAndFreedom, NewMinistryFromString("faith-freedom"))
	assert.Equal(t, FaithAndFreedom, NewMinistryFromString("FaithAndFreedom"))

	assert.Equal(t, UnknownMinistry, NewMinistryFromString("Open Heavens"))
}

func TestConvertToString(t *testing.T) {
	assert.Equal(t, "wol", string(WordOfLife))
	assert.Equal(t, "core", string(CenterOfRelationshipExperience))
	assert.Equal(t, "tbo", string(TheBridgeOutreach))
	assert.Equal(t, "ask-pastor", string(AskThePastor))
	assert.Equal(t, "faith-freedom", string(FaithAndFreedom))
	assert.Equal(t, "unknown", string(UnknownMinistry))
}

func TestMinistryString(t *testing.T) {
	assert.Equal(t, "Word of Life", WordOfLife.Description())
	assert.Equal(t, "C.O.R.E.", CenterOfRelationshipExperience.Description())
	assert.Equal(t, "The Bridge Outreach", TheBridgeOutreach.Description())
	assert.Equal(t, "Ask the Pastor", AskThePastor.Description())
	assert.Equal(t, "Faith & Freedom", FaithAndFreedom.Description())
	assert.Equal(t, "(Unknown Ministry)", UnknownMinistry.Description())
}
