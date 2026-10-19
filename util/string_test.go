package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeForMatch(t *testing.T) {
	// case, punctuation and run-together spacing are all house style, not meaning
	assert.Equal(t, "power of prayer q a", NormalizeForMatch("Power of Prayer: Q&A"))
	assert.Equal(t, "power of prayer qa", NormalizeForMatch("Power  of   Prayer QA"))
	assert.Equal(t, "", NormalizeForMatch("   "))

	// articles come and go between the sheet and a file name without changing the title
	assert.Equal(t, "importance of voting", NormalizeForMatch("The Importance of Voting"))
	assert.Equal(t, "time to build in house of god", NormalizeForMatch("A Time to Build in the House of God"))
	assert.Equal(t, "end of age", NormalizeForMatch("An End of the Age"))
	assert.Equal(t, "importance of voting", NormalizeForMatch("(The) Importance, of Voting"))

	// only whole words: the A in Q&A stays, as do words that merely contain one
	assert.Equal(t, "power of prayer q a", NormalizeForMatch("Power of Prayer: Q&A"))
	assert.Equal(t, "theology anchor", NormalizeForMatch("Theology Anchor"))
	assert.Equal(t, "", NormalizeForMatch("The"))
}
