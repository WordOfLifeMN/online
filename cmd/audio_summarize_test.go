package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/util/box"
	"github.com/stretchr/testify/assert"
)

// the 62-word example the prompt shows, which follows every rule
const goodSummary = exampleSummary

// the example must itself pass the rules it is held up as following
func TestExampleSummaryFollowsTheRules(t *testing.T) {
	assert.Empty(t, summaryProblems(exampleSummary, []string{"Pastor Mary Peltz"}))
	assert.Equal(t, 62, len(strings.Fields(exampleSummary)))
}

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

	assert.Contains(t, prompt, "two or three short sentences, about 55 words")
	assert.Contains(t, prompt, "never be more than 65 words")
	assert.Contains(t, prompt, "<example>\n"+exampleSummary+"\n</example>")
	assert.Contains(t, prompt, "not its content or its speaker")
	assert.Contains(t, prompt, `"Pastor Vern Peltz" and "Pastor Mary Peltz"`)
	assert.Contains(t, prompt, "The first time it mentions each speaker")
	assert.Contains(t, prompt, "including any title such as \"Pastor\"")
	assert.Contains(t, prompt, "may refer to them more briefly")
	assert.Contains(t, prompt,
		`Acceptable shorter forms for "Pastor Vern Peltz" include "Pastor", "Pastor Vern" or "Vern".`)
	assert.Contains(t, prompt,
		`Acceptable shorter forms for "Pastor Mary Peltz" include "Pastor", "Pastor Mary" or "Mary".`)
	assert.Contains(t, prompt, `""" the transcript """`)
}

// the instructions follow the transcript, so they are fresh when the model writes:
// ahead of ten thousand words of sermon, the word limit was being ignored
func TestBuildSummaryPrompt_InstructionsFollowTheTranscript(t *testing.T) {
	prompt := buildSummaryPrompt("Pastor Mary Peltz", "the transcript", true)

	transcript := strings.Index(prompt, `""" the transcript """`)
	for _, instruction := range []string{
		"The speaker's name is",
		"The title will be",
		"never be more than 65 words",
		"<example>",
		"must name the speaker",
		"note any passages",
	} {
		at := strings.Index(prompt, instruction)
		assert.Greater(t, at, transcript, "%q comes after the transcript", instruction)
	}
}

func TestShortSpeakerForms(t *testing.T) {
	assert.Equal(t, []string{"Pastor", "Pastor Mary", "Mary"}, shortSpeakerForms("Pastor Mary Peltz"))
	assert.Equal(t, []string{"Mary"}, shortSpeakerForms("Mary Peltz"), "no title, so first name only")
	assert.Empty(t, shortSpeakerForms("Mary"), "a single name has nothing shorter")
}

// with nobody to name, the prompt asks for no name
func TestBuildSummaryPrompt_NoSpeaker(t *testing.T) {
	prompt := buildSummaryPrompt("", "the transcript", false)
	assert.NotContains(t, prompt, "The speaker's name is")
	assert.NotContains(t, prompt, "must name the speaker")
	assert.Contains(t, prompt, "never be more than 65 words")
}

func TestSummarySchema_DescribesTheLimit(t *testing.T) {
	properties := summarySchema(false)["properties"].(map[string]any)
	description := properties["summary"].(map[string]any)["description"].(string)
	assert.Contains(t, description, "two or three short sentences, about 55 words")
	assert.Contains(t, description, "never more than 65")
	assert.Contains(t, description, "naming each speaker in full")
}

// a correction says how much to cut, not just what the limit is
func TestCorrectionPrompt_TooLong(t *testing.T) {
	long := goodSummary + " " + strings.Repeat("word ", 38) // 100 words
	prompt := correctionPrompt(long, []string{"Pastor Mary Peltz"})

	assert.Contains(t, prompt, "It is 100 words, and must be at most 65. Cut at least 35 words, aiming for about 55.")
	assert.NotContains(t, prompt, "does not name", "the name is there")
	assert.Contains(t, prompt, "same title and a corrected summary of two or three short sentences")
}

func TestCorrectionPrompt_NameMissing(t *testing.T) {
	short := strings.Replace(goodSummary, "Pastor Mary Peltz", "Pastor Mary", 1)
	prompt := correctionPrompt(short, []string{"Pastor Mary Peltz"})

	assert.Contains(t, prompt, `It does not name "Pastor Mary Peltz" in full.`)
	assert.NotContains(t, prompt, "Cut at least", "it is short enough")
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

func TestPrintSummaryInfo(t *testing.T) {
	var buf bytes.Buffer
	printSummaryInfo(&buf, &MessageInfo{
		TranscriptPath: "C:/Videos/2026-10-04-pm Pay Attention.txt",
		VideoPath:      "C:/Videos/2026-10-04-pm Pay Attention.mp4",
		SpeakerName:    "Pastor Mary Peltz",
		Title:          "Pay Attention to God's Breadcrumbs",
		Summary:        goodSummary,
		Message:        &catalog.CatalogMessage{Name: "Pay Attention", Description: "The old, long description."},
	})
	out := buf.String()

	assert.Contains(t, out, "╭─ Summary ")
	assert.Contains(t, out, "│ Transcript : 2026-10-04-pm Pay Attention.txt ")
	assert.Contains(t, out, "│ Video file : 2026-10-04-pm Pay Attention.mp4 ")
	assert.Contains(t, out, "│ Speaker    : Pastor Mary Peltz ")
	assert.Contains(t, out, "│ Spreadsheet: Pay Attention")

	// the title and description are bare copy lines, as in the 'audio' box
	assert.Contains(t, out, "\nPay Attention to God's Breadcrumbs\n")
	assert.Contains(t, out, "\n"+goodSummary+"\n")

	assert.Contains(t, out, "├─ Description in the sheet now ")
	assert.Contains(t, out, "│ The old, long description. ")
	assert.NotContains(t, out, "Summary check", "a summary with no problems has no check")
	assert.True(t, strings.HasSuffix(out, "╯\n"), "the box is closed")
}

// a transcript given directly has no video and may have no row: those lines are left out
func TestPrintSummaryInfo_TranscriptOnly(t *testing.T) {
	var buf bytes.Buffer
	printSummaryInfo(&buf, &MessageInfo{
		TranscriptPath: "message.text",
		SpeakerName:    "Pastor Vern Peltz",
		Title:          "A Title",
		Summary:        "A summary.",
	})
	out := buf.String()

	assert.NotContains(t, out, "Video file")
	assert.NotContains(t, out, "Spreadsheet")
	assert.NotContains(t, out, "in the sheet now")
}
