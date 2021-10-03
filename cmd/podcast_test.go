package cmd

import (
	"bytes"
	"testing"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/stretchr/testify/suite"
)

type PodcastCmdTestSuite struct {
	suite.Suite
}

// Runs the test suite as a test
func TestPodcastCmdTestSuite(t *testing.T) {
	suite.Run(t, new(PodcastCmdTestSuite))
}

func (t *PodcastCmdTestSuite) TestEmptyPodcast() {
	cmd := podcastCmdStruct{}

	data := map[string]interface{}{
		"Title":         "TITLE",
		"Description":   "DESC",
		"CopyrightYear": 1000,
	}

	b := bytes.Buffer{}
	err := cmd.printPodcast(data, &b)
	t.NoError(err)

	s := b.String()
	t.Contains(s, "<title>TITLE<")
	t.Contains(s, "<description>DESC<")
	t.Contains(s, "Copyright 1000 ")
	t.NotContains(s, "<item>")
}

func (t *PodcastCmdTestSuite) TestSingleMessagePodcast() {
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
	t.NoError(err)

	s := b.String()
	t.T().Logf("Single MessageString:\n%s", s)

	t.Contains(s, "<title>TITLE<")
	t.Contains(s, "<description>DESC<")
	t.Contains(s, "Copyright 1000 ")
	t.Contains(s, "<item>")
	t.Contains(s, "<title>MSG-TITLE<")
	t.Contains(s, "<description>MSG-TITLE (Feb 3, 2001)<")
	t.Contains(s, "<pubDate>Sat, 3 Feb 2001 10:00:00 CDT<")
	t.Contains(s, `url="http://AUDIO.mp3"`)
	t.Contains(s, `length="-1"`) // because audio file does not exist
}

func (t *PodcastCmdTestSuite) TestSingleMessagePodcastCharacters() {
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
	t.NoError(err)

	s := b.String()
	t.T().Logf("Single MessageString:\n%s", s)

	t.Contains(s, "<title>T&#39;LE<")
	t.Contains(s, "<description>DESC &amp; DETAILS<")
	t.Contains(s, "<title>MSG&lt;TITLE&gt;<")
	t.Contains(s, "<description>MSG&lt;TITLE&gt; (Feb 3, 2001)<")
}
