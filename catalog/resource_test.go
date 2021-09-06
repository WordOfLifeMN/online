package catalog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPanicsOnEmptyString(t *testing.T) {
	assert.Equal(t, OnlineResource{}, NewResourceFromString(""))
}

func TestNewWikiString(t *testing.T) {
	assert.Equal(t,
		OnlineResource{URL: "http://hi", Name: "Hi"},
		NewResourceFromString("Hi|http://hi"))

	assert.Equal(t,
		OnlineResource{URL: "http://hi", Name: "Hi"},
		NewResourceFromString(" Hi|http://hi"))

	assert.Equal(t,
		OnlineResource{URL: "http://hi", Name: "Hi"},
		NewResourceFromString("Hi|http://hi "))

	assert.Equal(t,
		OnlineResource{URL: "http://hi", Name: "Hi"},
		NewResourceFromString("Hi | http://hi"))

	assert.Equal(t,
		OnlineResource{URL: "http://hi.doc", Name: "Hi"},
		NewResourceFromString("Hi | http://hi.doc"))

}

func TestNewMarkdownString(t *testing.T) {
	assert.Equal(t,
		OnlineResource{URL: "http://hi", Name: "Hi"},
		NewResourceFromString("[Hi](http://hi)"))

	assert.Equal(t,
		OnlineResource{URL: "http://hi", Name: "Hi"},
		NewResourceFromString("[ Hi ](http://hi)"))

	assert.Equal(t,
		OnlineResource{URL: "http://hi", Name: "Hi"},
		NewResourceFromString("[Hi]( http://hi )"))

}

func TestNewRawURL(t *testing.T) {
	assert.Equal(t,
		OnlineResource{URL: "http://hi.doc", Name: "hi"},
		NewResourceFromString("http://hi.doc"))

	assert.Equal(t,
		OnlineResource{URL: "http://path/hi.doc", Name: "hi"},
		NewResourceFromString("http://path/hi.doc"))

	assert.Equal(t,
		OnlineResource{URL: "http://hi", Name: "hi"},
		NewResourceFromString("http://hi"))

	assert.Equal(t,
		OnlineResource{URL: "http://yt/hi/", Name: "hi"},
		NewResourceFromString("http://yt/hi/"))

	assert.Equal(t,
		OnlineResource{URL: "http://yt/hi+there.pdf", Name: "hi there"},
		NewResourceFromString("http://yt/hi+there.pdf"))

	assert.Equal(t,
		OnlineResource{URL: "http://hi%20there.pdf", Name: "hi there"},
		NewResourceFromString("http://hi%20there.pdf"))

	assert.Equal(t,
		OnlineResource{URL: "http://Dr.%20Doolittle.pdf", Name: "Dr. Doolittle"},
		NewResourceFromString("http://Dr.%20Doolittle.pdf"))

	assert.Equal(t,
		OnlineResource{
			URL:  "http://file%20for%20q%2Ba%2C%20and%20%22discussion%22.pdf",
			Name: "file for q+a, and \"discussion\"",
		},
		NewResourceFromString("http://file%20for%20q%2Ba%2C%20and%20%22discussion%22.pdf"))

}

func TestNewEmptyResources(t *testing.T) {
	assert.Len(t, NewResourcesFromString(""), 0)

	assert.Len(t, NewResourcesFromString(" "), 0)

	assert.Len(t, NewResourcesFromString(";;;"), 0)

	assert.Len(t, NewResourcesFromString("; ; ;"), 0)
}

func TestNewResources(t *testing.T) {
	assert.Equal(t,
		[]OnlineResource{
			{
				Name: "one",
				URL:  "http://one.pdf",
			},
			{
				Name: "2",
				URL:  "http://two.pdf",
			},
			{
				Name: "Video",
				URL:  "http://youtu.be/12368",
			},
		},
		NewResourcesFromString("http://one.pdf; [2](http://two.pdf); Video|http://youtu.be/12368"),
	)
}
