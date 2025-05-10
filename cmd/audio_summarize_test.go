package cmd

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestAudioSummarizeCmdTestSuite(t *testing.T) {
	suite.Run(t, new(AudioSummarizeCmdTestSuite))
}

type AudioSummarizeCmdTestSuite struct {
	suite.Suite
}

func (t *AudioSummarizeCmdTestSuite) TestFindStartOfSentence() {
	s := "First sentence. Is not! It is, isn't it? Pretty sure."
	//    ^0            ^14     ^22              ^39          ^52

	t.Equal(0, findStartOfSentence(s, -2))
	t.Equal(0, findStartOfSentence(s, 0))
	t.Equal(0, findStartOfSentence(s, 1))
	t.Equal(0, findStartOfSentence(s, 13))
	t.Equal(0, findStartOfSentence(s, 14))

	t.Equal(16, findStartOfSentence(s, 15))
	t.Equal(16, findStartOfSentence(s, 21))
	t.Equal(16, findStartOfSentence(s, 22))

	t.Equal(24, findStartOfSentence(s, 23))
	t.Equal(24, findStartOfSentence(s, 38))
	t.Equal(24, findStartOfSentence(s, 39))

	t.Equal(41, findStartOfSentence(s, 41))
	t.Equal(41, findStartOfSentence(s, 51))
	t.Equal(41, findStartOfSentence(s, 52))

	t.Equal(53, findStartOfSentence(s, 53))
	t.Equal(53, findStartOfSentence(s, 999))

	r := "this is just raw wo?rds with no sen.tence structure"

	t.Equal(0, findStartOfSentence(r, 0))
	t.Equal(0, findStartOfSentence(r, 1))
	t.Equal(0, findStartOfSentence(r, 30))
	t.Equal(51, findStartOfSentence(r, 80))
	t.Equal(51, findStartOfSentence(r, 999))
}

func (t *AudioSummarizeCmdTestSuite) TestFindStartOfWord() {
	s := "the word is bird"
	//    ^0  ^4   ^9 ^12

	t.Equal(0, findStartOfWord(s, -2))
	t.Equal(0, findStartOfWord(s, 0))
	t.Equal(0, findStartOfWord(s, 1))
	t.Equal(0, findStartOfWord(s, 3))

	t.Equal(4, findStartOfWord(s, 4))
	t.Equal(4, findStartOfWord(s, 7))
	t.Equal(4, findStartOfWord(s, 8))

	t.Equal(9, findStartOfWord(s, 9))
	t.Equal(9, findStartOfWord(s, 10))
	t.Equal(9, findStartOfWord(s, 11))

	t.Equal(12, findStartOfWord(s, 12))
	t.Equal(12, findStartOfWord(s, 13))
	t.Equal(12, findStartOfWord(s, 15))

	t.Equal(16, findStartOfWord(s, 16))
	t.Equal(16, findStartOfWord(s, 999))
}

func (t *AudioSummarizeCmdTestSuite) TestFindPreviousSentenceStart() {
	s := "First sentence. Is not! It is, isn't it? Pretty sure."
	//    ^0            ^14     ^22              ^39          ^52

	t.Equal(0, findPreviousSentenceStart(s, -2, 5))
	t.Equal(0, findPreviousSentenceStart(s, 4, 5))
	t.Equal(0, findPreviousSentenceStart(s, 5, 5))

	t.Equal(24, findPreviousSentenceStart(s, 28, 5))
	t.Equal(24, findPreviousSentenceStart(s, 29, 5))
	t.Equal(27, findPreviousSentenceStart(s, 30, 5)) // sentence start outside window

	t.Equal(48, findPreviousSentenceStart(s, 49, 5))
	t.Equal(48, findPreviousSentenceStart(s, 52, 5))

	t.Equal(53, findPreviousSentenceStart(s, 90, 5))
}

func (t *AudioSummarizeCmdTestSuite) TestExtractSampleFromMiddle() {
	s := "left1. left2. left3. left4. middle. rite1. rite2. rite3. rite4."

	t.Equal("middle.", extractSampleFromMiddle(s, 2))
	t.Equal("left4. middle.", extractSampleFromMiddle(s, 4))
	t.Equal("left3. left4. middle. rite1.", extractSampleFromMiddle(s, 6))
	t.Equal("left2. left3. left4. middle. rite1. rite2.", extractSampleFromMiddle(s, 10))
	t.Equal("left1. left2. left3. left4. middle. rite1. rite2. rite3. rite4.", extractSampleFromMiddle(s, 16))
	t.Equal("left1. left2. left3. left4. middle. rite1. rite2. rite3. rite4.", extractSampleFromMiddle(s, 128))
}

func (t *AudioSummarizeCmdTestSuite) TestGetSpeakerFromFileName() {
	for i, tc := range []struct {
		FileName string
		Speaker  string
		Pronoun  string
	}{
		{"2025-03-09 Msg-M.mp4", "Pastor Mary Peltz", "she/her"},
		{"2025-03-09 Msg-m.mp4", "Pastor Mary Peltz", "she/her"},
		{"2025-03-09-m Msg.mp4", "Pastor Mary Peltz", "she/her"},
		{"2025-03-09-pm Msg.mp4", "Pastor Mary Peltz", "she/her"},
		{"2025-03-09-mp Msg.mp4", "Pastor Mary Peltz", "she/her"},
		{"2025-03-09pm Msg.mp4", "Pastor Mary Peltz", "she/her"},
		{"2025-03-09-MP Msg.mp4", "Pastor Mary Peltz", "she/her"},

		{"2025-03-09-v Msg.mp4", "Pastor Vern Peltz", "he/him"},
		{"2025-03-09-m Msg.mp4", "Pastor Mary Peltz", "she/her"},
		{"2025-03-09-i Msg.mp4", "Pastor Igor Kondratyuk", "he/him"},
		{"2025-03-09-t Msg.mp4", "Tania Kondratyuk", "she/her"},
		{"2025-03-09-a Msg.mp4", "Anthony Leong", "he/him"},
		{"2025-03-09-j Msg.mp4", "Jim Isakson", "he/him"},

		{"2025-03-09-pt Msg.mp4", "Tania Kondratyuk", "she/her"},
	} {
		speaker, pronouns := getSpeakerFromFileName(tc.FileName)
		t.Equal(tc.Speaker, speaker, "Case %d failed: %s", i+1, tc.FileName)
		t.Equal(tc.Pronoun, pronouns, "Case %d failed: %s", i+1, tc.FileName)
	}
}
