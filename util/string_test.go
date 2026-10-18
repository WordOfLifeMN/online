package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeForMatch(t *testing.T) {
	// case, punctuation and run-together spacing are all house style, not meaning
	assert.Equal(t, "power of prayer q a", NormalizeForMatch("Power of Prayer: Q&A"))
	assert.Equal(t, "power of prayer qa", NormalizeForMatch("Power  of   Prayer QA"))
	assert.Equal(t, "the importance of voting", NormalizeForMatch("The Importance of Voting"))
	assert.Equal(t, "", NormalizeForMatch("   "))
}
