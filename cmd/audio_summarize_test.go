package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/WordOfLifeMN/online/util/box"
	"github.com/stretchr/testify/assert"
)

// the 62-word example from the TODO list, which follows every rule
const goodSummary = "Pastor Mary Peltz challenges us to pay attention to God's voice and let Him " +
	"question everything we think we know. Using Abraham's journey to Moriah, she shows how " +
	"growth that stretches us leads to a clear, God-given destiny we then surrender to Him. In " +
	"finances, relationships, and purpose, she calls us to partner with God, planting faith and " +
	"calling forth the harvest."

func TestSummarySpeakers(t *testing.T) {
	assert.Equal(t, []string{"Pastor Mary Peltz"}, summarySpeakers("Pastor Mary Peltz"))
	assert.Equal(t, []string{"Pastor Vern Peltz", "Pastor Mary Peltz"},
		summarySpeakers("Pastor Vern Peltz, Pastor Mary Peltz"))
	assert.Empty(t, summarySpeakers(""))
	assert.Empty(t, summarySpeakers(" , "))
}

func TestSummaryProblems_GoodSummary(t *testing.T) {
	assert.Empty(t, summaryProblems(goodSummary, []string{"Pastor Mary Peltz"}))

	// a later, shorter reference is fine once the full name has appeared
	assert.Empty(t, summaryProblems(strings.Replace(goodSummary, "she calls", "Pastor Mary calls", 1),
		[]string{"Pastor Mary Peltz"}))

	// case is not held against it
	assert.Empty(t, summaryProblems(strings.ToLower(goodSummary), []string{"Pastor Mary Peltz"}))
}

func TestSummaryProblems_TooLong(t *testing.T) {
	long := goodSummary + " " + strings.Repeat("word ", 10)
	assert.Equal(t, []string{"it is 72 words long; the limit is 65"},
		summaryProblems(long, []string{"Pastor Mary Peltz"}))

	// exactly at the limit is fine
	assert.Empty(t, summaryProblems(strings.Repeat("word ", 65), nil))
}

// the honorific is part of the name: dropping it on the only mention is a problem
func TestSummaryProblems_SpeakerNotNamedInFull(t *testing.T) {
	withoutHonorific := strings.Replace(goodSummary, "Pastor Mary Peltz", "Mary Peltz", 1)
	assert.Equal(t, []string{`it does not name "Pastor Mary Peltz" in full`},
		summaryProblems(withoutHonorific, []string{"Pastor Mary Peltz"}))

	// each of several speakers is needed
	assert.Equal(t, []string{`it does not name "Pastor Vern Peltz" in full`},
		summaryProblems(goodSummary, []string{"Pastor Vern Peltz", "Pastor Mary Peltz"}))

	// with no speaker known there is nothing to require
	assert.Empty(t, summaryProblems("A summary naming nobody.", nil))
}

func TestBuildSummaryPrompt(t *testing.T) {
	prompt := buildSummaryPrompt("Pastor Vern Peltz, Pastor Mary Peltz", "the transcript", false)

	assert.Contains(t, prompt, "no longer than 65 words")
	assert.NotContains(t, prompt, "3 sentences", "the sentence count is gone")
	assert.Contains(t, prompt, `"Pastor Vern Peltz" and "Pastor Mary Peltz"`)
	assert.Contains(t, prompt, "The first time it mentions each speaker")
	assert.Contains(t, prompt, "including any title such as \"Pastor\"")
	assert.Contains(t, prompt, "may refer to them more briefly")
	assert.Contains(t, prompt, `""" the transcript """`)
}

// with nobody to name, the prompt asks for no name
func TestBuildSummaryPrompt_NoSpeaker(t *testing.T) {
	prompt := buildSummaryPrompt("", "the transcript", false)
	assert.NotContains(t, prompt, "The speaker's name is")
	assert.NotContains(t, prompt, "must name the speaker")
	assert.Contains(t, prompt, "no longer than 65 words")
}

func TestSummarySchema_DescribesTheLimit(t *testing.T) {
	properties := summarySchema(false)["properties"].(map[string]any)
	description := properties["summary"].(map[string]any)["description"].(string)
	assert.Contains(t, description, "at most 65 words")
	assert.Contains(t, description, "naming each speaker in full")
	assert.NotContains(t, description, "3 sentence")
}

func TestCorrectionPrompt(t *testing.T) {
	prompt := correctionPrompt([]string{"it is 80 words long; the limit is 65"})
	assert.Contains(t, prompt, "- it is 80 words long; the limit is 65\n")
	assert.Contains(t, prompt, "same title and a corrected summary")
}

func TestPrintSummaryProblems(t *testing.T) {
	var buf bytes.Buffer
	b := box.New(&buf)

	printSummaryProblems(b, &MessageInfo{})
	assert.Empty(t, buf.String(), "nothing to say, nothing shown")

	printSummaryProblems(b, &MessageInfo{SummaryProblems: []string{`it does not name "Pastor Mary Peltz" in full`}})
	out := buf.String()
	assert.Contains(t, out, "├─ Summary check ")
	assert.Contains(t, out, `│ • it does not name "Pastor Mary Peltz" in full `)
	assert.Contains(t, out, "Edit it before pasting")
}
