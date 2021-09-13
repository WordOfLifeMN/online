package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringHash(t *testing.T) {
	assert.Equal(t, "ODA2MjgxMjk0", ComputeHash("Jabberwocky"))
	assert.Equal(t, "ODA2MjgxMjk0", ComputeHash("Jabberwocky"))
	assert.Equal(t, "MjY5Mzk3NDg2", ComputeHash("JabberwockY"))
}
