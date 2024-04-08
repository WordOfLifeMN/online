package catalog

import (
	"fmt"
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
	testCases := []struct {
		resourceString string
		expectedName   string
		expectedURL    string
	}{
		{"Hi|http://hi", "Hi", "http://hi"},
		{" Hi|http://hi", "Hi", "http://hi"},
		{"Hi|http://hi ", "Hi", "http://hi"},
		{"Hi | http://hi", "Hi", "http://hi"},
		{"Hi | http://hi", "Hi", "http://hi"},
		{"Hi | http://hi.doc", "Hi", "http://hi.doc"},
		{"http://hi|Hi", "Hi", "http://hi"},
		{" http://hi|Hi", "Hi", "http://hi"},
		{"http://hi|Hi ", "Hi", "http://hi"},
		{"http://hi | Hi", "Hi", "http://hi"},
	}

	for index, tc := range testCases {
		expected := OnlineResource{
			URL:  tc.expectedURL,
			Name: tc.expectedName,
		}
		actual := NewResourceFromString(tc.resourceString)
		if t.NotNil(actual) {
			t.Equal(expected, *actual, fmt.Sprintf("Test case #%d failed", index))
		}
	}
}
func (t *OnlineResourceTestSuite) TestNewWikiString_WithMetadata() {
	testCases := []struct {
		resourceString   string
		expectedName     string
		expectedURL      string
		expectedMetadata map[string]string
	}{
		{`{"id":"hi"}Hi|http://hi.doc`, "Hi", "http://hi.doc", map[string]string{"id": "hi"}},
		{`Hi|{"id":"hi"}http://hi.doc`, "Hi", "http://hi.doc", map[string]string{"id": "hi"}},
		{`Hi|http://hi.doc {"id":"hi"}`, "Hi", "http://hi.doc", map[string]string{"id": "hi"}},
		{`[Hi]{"id":"hi"}(http://hi)`, "Hi", "http://hi", map[string]string{"id": "hi"}},
	}

	for index, tc := range testCases {
		expected := OnlineResource{
			URL:      tc.expectedURL,
			Name:     tc.expectedName,
			Metadata: tc.expectedMetadata,
		}
		actual := NewResourceFromString(tc.resourceString)
		if t.NotNil(actual) {
			t.Equal(expected, *actual, fmt.Sprintf("Test case #%d failed", index))
		}
	}
}

func (t *OnlineResourceTestSuite) TestNewMarkdownString() {
	testCases := []struct {
		resourceString string
		expectedName   string
		expectedURL    string
	}{
		{"[Hi](http://hi)", "Hi", "http://hi"},
		{"[ Hi ](http://hi)", "Hi", "http://hi"},
		{"[Hi]( http://hi )", "Hi", "http://hi"},
	}

	for index, tc := range testCases {
		expected := OnlineResource{
			URL:  tc.expectedURL,
			Name: tc.expectedName,
		}
		actual := NewResourceFromString(tc.resourceString)
		if t.NotNil(actual) {
			t.Equal(expected, *actual, fmt.Sprintf("Test case #%d failed", index))
		}
	}
}

func (t *OnlineResourceTestSuite) TestNewRawURL() {
	testCases := []struct {
		resourceString string
		expectedName   string
		expectedURL    string
	}{
		{"http://hi.doc", "hi", "http://hi.doc"},
		{" http://hi.doc", "hi", "http://hi.doc"},
		{"http://hi.doc ", "hi", "http://hi.doc"},
	}

	for index, tc := range testCases {
		expected := OnlineResource{
			URL:  tc.expectedURL,
			Name: tc.expectedName,
		}
		actual := NewResourceFromString(tc.resourceString)
		if t.NotNil(actual) {
			t.Equal(expected, *actual, fmt.Sprintf("Test case #%d failed", index))
		}
	}
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
