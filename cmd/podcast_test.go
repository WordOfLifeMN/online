package cmd

import (
	"bytes"
	"testing"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/stretchr/testify/assert"
)

func TestEmptyPodcast(t *testing.T) {
	assert := assert.New(t)

	cmd := podcastCmdStruct{}

	data := map[string]interface{}{
		"Title":         "TITLE",
		"Description":   "DESC",
		"CopyrightYear": 1000,
	}

	b := bytes.Buffer{}
	err := cmd.printPodcast(data, &b)
	assert.NoError(err)

	s := b.String()
	assert.Contains(s, "<title>TITLE<")
	assert.Contains(s, "<description>DESC<")
	assert.Contains(s, "Copyright 1000 ")
	assert.NotContains(s, "<item>")
}

func TestSingleMessagePodcast(t *testing.T) {
	assert := assert.New(t)

	cmd := podcastCmdStruct{}

	data := map[string]interface{}{
		"Title":         "TITLE",
		"Description":   "DESC",
		"CopyrightYear": 1000,
		"Messages": []catalog.CatalogMessage{
			{
				Date:        catalog.MustParseDateOnly("2001-02-03"),
				Name:        "MSG-TITLE",
				Description: "MSG-DESC",
				Audio:       "http://AUDIO.mp3",
			},
		},
	}

	b := bytes.Buffer{}
	err := cmd.printPodcast(data, &b)
	assert.NoError(err)

	s := b.String()
	t.Logf("Single MessageString:\n%s", s)

	assert.Contains(s, "<title>TITLE<")
	assert.Contains(s, "<description>DESC<")
	assert.Contains(s, "Copyright 1000 ")
	assert.Contains(s, "<item>")
	assert.Contains(s, "<title>MSG-TITLE<")
	assert.Contains(s, "<description>MSG-TITLE (Feb 3, 2001)<")
	assert.Contains(s, "<pubDate>Sat, 3 Feb 2001 10:00:00 CDT<")
	assert.Contains(s, `url="http://AUDIO.mp3"`)
	assert.Contains(s, `length="-1"`) // because audio file does not exist
}

func TestSingleMessagePodcastCharacters(t *testing.T) {
	assert := assert.New(t)

	cmd := podcastCmdStruct{}

	data := map[string]interface{}{
		"Title":         "T'LE",
		"Description":   "DESC & DETAILS",
		"CopyrightYear": 1000,
		"Messages": []catalog.CatalogMessage{
			{
				Date:        catalog.MustParseDateOnly("2001-02-03"),
				Name:        "MSG<TITLE>",
				Description: `MSG "DESC"`,
				Audio:       "http://AUDIO.mp3",
			},
		},
	}

	b := bytes.Buffer{}
	err := cmd.printPodcast(data, &b)
	assert.NoError(err)

	s := b.String()
	t.Logf("Single MessageString:\n%s", s)

	assert.Contains(s, "<title>T&#39;LE<")
	assert.Contains(s, "<description>DESC &amp; DETAILS<")
	assert.Contains(s, "<title>MSG&lt;TITLE&gt;<")
	assert.Contains(s, "<description>MSG&lt;TITLE&gt; (Feb 3, 2001)<")
}

// TODO: test special characters in title/description like ' < and >
