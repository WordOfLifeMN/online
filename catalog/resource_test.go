package catalog

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestOnlineResourceestSuite(t *testing.T) {
	suite.Run(t, new(OnlineResourceTestSuite))
}

type OnlineResourceTestSuite struct {
	suite.Suite
}

// Runs the test suite as a test
func (t *OnlineResourceTestSuite) TestEmptyString() {
	t.Equal(OnlineResource{}, *NewResourceFromString(""))
}

func (t *OnlineResourceTestSuite) TestNewWikiString() {
	t.Equal(
		OnlineResource{URL: "http://hi", Name: "Hi"},
		*NewResourceFromString("Hi|http://hi"))

	t.Equal(
		OnlineResource{URL: "http://hi", Name: "Hi"},
		*NewResourceFromString(" Hi|http://hi"))

	t.Equal(
		OnlineResource{URL: "http://hi", Name: "Hi"},
		*NewResourceFromString("Hi|http://hi "))

	t.Equal(
		OnlineResource{URL: "http://hi", Name: "Hi"},
		*NewResourceFromString("Hi | http://hi"))

	t.Equal(
		OnlineResource{URL: "http://hi.doc", Name: "Hi"},
		*NewResourceFromString("Hi | http://hi.doc"))

	t.Equal(
		OnlineResource{URL: "http://hi.doc", Name: "Hi", Metadata: map[string]string{"id": "hi"}},
		*NewResourceFromString(`{"id":"hi"}Hi|http://hi.doc`))

	t.Equal(
		OnlineResource{URL: "http://hi.doc", Name: "Hi", Metadata: map[string]string{"id": "hi"}},
		*NewResourceFromString(`Hi|{"id":"hi"}http://hi.doc`))

	t.Equal(
		OnlineResource{URL: "http://hi.doc", Name: "Hi", Metadata: map[string]string{"id": "hi"}},
		*NewResourceFromString(`Hi|http://hi.doc {"id":"hi"}`))

}

func (t *OnlineResourceTestSuite) TestNewMarkdownString() {
	t.Equal(
		OnlineResource{URL: "http://hi", Name: "Hi"},
		*NewResourceFromString("[Hi](http://hi)"))

	t.Equal(
		OnlineResource{URL: "http://hi", Name: "Hi"},
		*NewResourceFromString("[ Hi ](http://hi)"))

	t.Equal(
		OnlineResource{URL: "http://hi", Name: "Hi"},
		*NewResourceFromString("[Hi]( http://hi )"))

	t.Equal(
		OnlineResource{URL: "http://hi", Name: "Hi", Metadata: map[string]string{"id": "hi"}},
		*NewResourceFromString(`[Hi]{"id":"hi"}(http://hi)`))

}

func (t *OnlineResourceTestSuite) TestNewRawURL() {
	t.Equal(
		OnlineResource{URL: "http://hi.doc", Name: "hi"},
		*NewResourceFromString("http://hi.doc"))

	t.Equal(
		OnlineResource{URL: "http://hi.doc", Name: "hi"},
		*NewResourceFromString("http://hi.doc "))

	t.Equal(
		OnlineResource{URL: "http://hi.doc", Name: "hi"},
		*NewResourceFromString(" http://hi.doc"))
}

func (t *OnlineResourceTestSuite) TestNameFromURL() {
	t.Equal("hi", (&OnlineResource{URL: "http://path/hi.doc"}).GetNameFromURL())
	t.Equal("hi", (&OnlineResource{URL: "http://hi"}).GetNameFromURL())
	t.Equal("hi", (&OnlineResource{URL: "http://yt/hi/"}).GetNameFromURL())
	t.Equal("Dr. Doolittle", (&OnlineResource{URL: "http://Dr.%20Doolittle.pdf"}).GetNameFromURL())
	t.Equal("file for q+a, and \"discussion\"",
		(&OnlineResource{URL: "http://file%20for%20q%2Ba%2C%20and%20%22discussion%22.pdf"}).GetNameFromURL())

	// different space encodings
	t.Equal("hi there", (&OnlineResource{URL: "http://yt/hi+there.pdf"}).GetNameFromURL())
	t.Equal("hi there", (&OnlineResource{URL: "http://hi%20there.pdf"}).GetNameFromURL())
	t.Equal("hi there", (&OnlineResource{URL: "http://hi_there.pdf"}).GetNameFromURL())
}

func (t *OnlineResourceTestSuite) TestNewEmptyResources() {
	t.Len(NewResourcesFromString(""), 0)

	t.Len(NewResourcesFromString(" "), 0)

	t.Len(NewResourcesFromString(";;;"), 0)

	t.Len(NewResourcesFromString("; ; ;"), 0)
}

func (t *OnlineResourceTestSuite) TestNewResources() {
	t.Equal(
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

/*
 * Metadata
 */

func (t *OnlineResourceTestSuite) TestMetadata_WhenNone() {
	s, m := extractResourceMetadata(`https://youtu.be/999`)

	t.Equal(`https://youtu.be/999`, s)
	t.Nil(m)
}

func (t *OnlineResourceTestSuite) TestMetadata_WhenValidString() {
	s, m := extractResourceMetadata(`https://youtu.be/999 {"id":"999"}`)

	t.Equal(`https://youtu.be/999 `, s)
	t.Equal(map[string]string{"id": "999"}, m)
}

func (t *OnlineResourceTestSuite) TestMetadata_WhenValidInteger() {
	s, m := extractResourceMetadata(`https://youtu.be/999 {"id":999}`)

	t.Equal(`https://youtu.be/999 `, s)
	t.Equal(map[string]string{"id": "999"}, m)
}

func (t *OnlineResourceTestSuite) TestMetadata_WhenValidBoolean() {
	s, m := extractResourceMetadata(`https://youtu.be/999 {"id":true}`)

	t.Equal(`https://youtu.be/999 `, s)
	t.Equal(map[string]string{"id": "true"}, m)
}

func (t *OnlineResourceTestSuite) TestMetadata_WhenInvalidQuotes() {
	s, m := extractResourceMetadata(`https://youtu.be/999 {"id":"999}`)

	t.Equal(`https://youtu.be/999 {"id":"999}`, s)
	t.Nil(m)
}

func (t *OnlineResourceTestSuite) TestMetadata_WhenInvalidBrace() {
	s, m := extractResourceMetadata(`https://youtu.be/999 {"id":999`)

	t.Equal(`https://youtu.be/999 {"id":999`, s)
	t.Nil(m)
}

func (t *OnlineResourceTestSuite) TestMetadata() {
	t.Equal(
		OnlineResource{URL: "http://hi.doc", Name: "hi", Metadata: map[string]string{
			"id":     "18",
			"iframe": "https://rumble.com/hi",
		}},
		*NewResourceFromString(`http://hi.doc {"id":18, "iframe":"https://rumble.com/hi"}`))
}
