package catalog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVisibility(t *testing.T) {
	assert.True(t, IsVisibleInView(Public, Public))
	assert.True(t, IsVisibleInView(Public, Partner))
	assert.True(t, IsVisibleInView(Public, Private))

	assert.False(t, IsVisibleInView(Partner, Public))
	assert.True(t, IsVisibleInView(Partner, Partner))
	assert.True(t, IsVisibleInView(Partner, Private))

	assert.False(t, IsVisibleInView(Private, Public))
	assert.False(t, IsVisibleInView(Private, Partner))
	assert.True(t, IsVisibleInView(Private, Private))
}
