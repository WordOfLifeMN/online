package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/WordOfLifeMN/online/util"
	"github.com/WordOfLifeMN/online/util/box"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// defaultAnthropicModel is the model used to generate titles and summaries.
//
// Compared against claude-opus-5 on a real 80 minute service, two runs each
// (2026-10-03). Both identified the speaker, his organisation, the seven points and
// the closing image, and neither would mislead a viewer. Opus wrote livelier copy,
// reaching for specifics - "the guy who never learned to make a bed" - where Haiku
// stayed abstract, but it also ran about 35% longer, picked a different title every
// run, and one run in two emitted an undecoded unicode escape and a stray newline
// into the description. Haiku was steadier and costs roughly a fifth as much.
//
// The summary is the part people actually read and the speaker overwrites the title
// anyway, so steadiness wins. Set 'anthropic-model' to try something else.
const defaultAnthropicModel = "claude-haiku-4-5"

// RiskNote is one advisory observation about a passage a platform reviewer might act
// on. It is never a verdict: it names the area, quotes the passage so the operator can
// check it in context, and explains the concern so an out-of-date understanding of
// platform policy is visible rather than silently applied.
type RiskNote struct {
	PolicyArea string `json:"policy_area"`
	Quote      string `json:"quote"`
	Reasoning  string `json:"reasoning"`
}

// policyAreaUnreliable is the policy area the model uses to report that the transcript
// is too garbled at some point to judge, rather than guessing at what was said.
const policyAreaUnreliable = "transcript-unreliable"

// messageSummary is the structured response from the model
type messageSummary struct {
	Title     string     `json:"title"`
	Summary   string     `json:"summary"`
	RiskNotes []RiskNote `json:"risk_notes"`
}

// audioSummarizeCmd represents the command to summarize the audio transcript to titles
// and descriptions
var audioSummarizeCmd = &cobra.Command{
	Use:   "summarize transcript-file|audio-file|video-file",
	Short: "Summarize the transcription of an audio file",
	Long: `Takes a transcription of an audio file and produces a suggested title and description.

The input file may be a transcript (.txt or .text), or an .mp3 or .mp4 file. Given a
video, the transcript kept beside it is used, falling back to the scratch directory.
Given an audio file, the transcript is looked for in the scratch directory, where
'audio transcribe' writes it.

The entire transcript is sent for analysis.

Requires Anthropic credentials: either ANTHROPIC_API_KEY in the environment or an
'ant auth login' profile.`,
	RunE: xscriptSummarize,
}

func init() {
	audioCmd.AddCommand(audioSummarizeCmd)

	audioSummarizeCmd.Args = cobra.MaximumNArgs(1)

	audioCmd.PersistentFlags().Bool("risk-notes", false,
		"Include advisory platform-risk notes (on by default for Faith & Freedom)")
	viper.BindPFlag("risk-notes", audioCmd.PersistentFlags().Lookup("risk-notes"))
}

func xscriptSummarize(cmd *cobra.Command, args []string) error {
	initLogging()

	var xscriptPath string

	// get the input file
	if len(args) == 1 {
		xscriptPath = args[0]
	}
	xscriptPath = PromtUserForInputFile(xscriptPath, ".mp4", ".mp3", ".text", ".txt")
	if xscriptPath == "" {
		fmt.Printf("Aborting")
		return nil
	}

	// resolve a video or audio file to its transcript. for a video that is the one kept
	// beside it, else whatever is still in scratch. reading the scratch copy here is
	// harmless where reusing it in 'audio' is not: this never transcribes, so it cannot
	// stand in the way of a transcript being regenerated.
	if strings.EqualFold(filepath.Ext(xscriptPath), ".mp4") {
		videoPath := xscriptPath
		xscriptPath = findExistingTranscript(videoPath)
		if xscriptPath == "" {
			xscriptPath = getScratchTranscriptPathFromVideoPath(videoPath)
		}
		if !util.IsFile(xscriptPath) {
			return fmt.Errorf("no transcript found for %s", filepath.Base(videoPath))
		}
	}
	if strings.EqualFold(filepath.Ext(xscriptPath), ".mp3") {
		xscriptPath = getTranscribePathFromAudioPath(xscriptPath)
	}
	if !util.DoesPathExist(xscriptPath) {
		return fmt.Errorf("file not found: %s", xscriptPath)
	}
	if util.IsDirectory(xscriptPath) {
		return fmt.Errorf("must be a file, not a directory: %s", xscriptPath)
	}

	// this subcommand works from a transcript alone, with no spreadsheet lookup, so
	// there is nobody to ask but the operator
	speaker := viper.GetString("speaker")
	if speaker == "" {
		speaker = PromptUserForSpeaker()
	}

	info := &MessageInfo{
		TranscriptPath: xscriptPath,
		SpeakerName:    speaker,
	}
	var err error
	if info, err = generateMessageSummary(info); err != nil {
		return err
	}

	fmt.Printf("\n\n%s\n", util.ToJSON(info))

	return nil
}

// anthropicClientOptions supplies the API key to the SDK client.
//
// The key is read from the 'anthropic-api-key' configuration value in
// ~/.wolm/online-config.yaml and handed straight to the client, rather than being
// exported into the environment. That matters for two reasons:
//
//   - A user-wide ANTHROPIC_API_KEY would shadow any OAuth profile on this machine,
//     including the one Claude Code uses. The church's key must not end up
//     authenticating anything but this application.
//   - Setting it with os.Setenv would also hand it to every child process we spawn -
//     ffmpeg and faster-whisper would both inherit it for no reason.
//
// Returning no options lets the SDK fall back to its normal resolution
// (ANTHROPIC_API_KEY, then an 'ant auth login' profile), so a machine configured that
// way still works.
//
// There is deliberately no command line flag for the key: a flag would expose it in
// shell history and in the process list.
func anthropicClientOptions() []option.RequestOption {
	if key := viper.GetString("anthropic-api-key"); key != "" {
		return []option.RequestOption{option.WithAPIKey(key)}
	}
	return nil
}

// checkAnthropicCredentials reports a useful error when nothing has been configured,
// rather than letting the SDK fail with an unauthenticated request after the audio has
// already been extracted and transcribed
func checkAnthropicCredentials() error {
	if viper.GetString("anthropic-api-key") != "" {
		return nil
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "" || os.Getenv("ANTHROPIC_AUTH_TOKEN") != "" {
		return nil
	}

	return fmt.Errorf(`no Anthropic credentials configured.

Add the church's API key to %s:

    anthropic-api-key: sk-ant-...

The key is read from there and handed to this process only. Do NOT set
ANTHROPIC_API_KEY as a user-wide environment variable - it would override the
credentials Claude Code and other tools use on this machine`,
		viper.ConfigFileUsed())
}

// getAnthropicModel returns the model to use for summarization
func getAnthropicModel() string {
	if model := viper.GetString("anthropic-model"); model != "" {
		return model
	}
	return defaultAnthropicModel
}

// wantRiskNotes reports whether advisory platform-risk notes should be requested for
// this message. They are on by default for Faith & Freedom - the ministry that
// sometimes produces content that would not survive on YouTube - and off elsewhere,
// because weekly risk notes on ordinary preaching would be noise.
func wantRiskNotes(info *MessageInfo) bool {
	if viper.GetBool("risk-notes") {
		return true
	}
	return isFaithAndFreedomPath(info.VideoPath) || isFaithAndFreedomPath(info.TranscriptPath)
}

// isFaithAndFreedomPath reports whether a file name marks a Faith & Freedom message.
// The " FF " marker is the same convention the audio trim length already relies on.
func isFaithAndFreedomPath(path string) bool {
	return strings.Contains(strings.ToUpper(path), " FF ")
}

// generateMessageSummary sends the transcript to Claude and fills in the title,
// summary, and - when requested - advisory platform-risk notes.
//
// The whole transcript is sent. The sampling this used to do (a ~12,000 token slice
// taken from the middle) existed only to fit GPT-3.5-turbo's context window, and it
// discarded the opening and closing of the message - the two passages that say most
// about what the message is actually about.
func generateMessageSummary(info *MessageInfo) (*MessageInfo, error) {
	if info.TranscriptPath == "" {
		return info, fmt.Errorf("no transcript to summarize")
	}

	xscriptBytes, err := os.ReadFile(info.TranscriptPath)
	if err != nil {
		return info, fmt.Errorf("cannot read transcript %s: %w", info.TranscriptPath, err)
	}
	// the header records how the transcript was made, which is no business of the model
	xscript := strings.TrimSpace(stripTranscriptHeader(string(xscriptBytes)))
	if xscript == "" {
		return info, fmt.Errorf("transcript %s is empty", info.TranscriptPath)
	}

	if err := checkAnthropicCredentials(); err != nil {
		return info, err
	}

	riskNotes := wantRiskNotes(info)
	speakers := summarySpeakers(info.SpeakerName)

	client := anthropic.NewClient(anthropicClientOptions()...)
	conversation := []anthropic.MessageParam{
		anthropic.NewUserMessage(
			anthropic.NewTextBlock(buildSummaryPrompt(info.SpeakerName, xscript, riskNotes)),
		),
	}

	summary, reply, err := requestSummary(&client, conversation, riskNotes)
	if err != nil {
		return info, err
	}

	// a summary that breaks the rules for new descriptions is sent back, with what is
	// wrong, to be corrected - up to maxSummaryCorrections times. A failed correction
	// request keeps the last summary rather than losing it.
	problems := summaryProblems(sanitizeModelText(summary.Summary), speakers)
	for attempt := 0; attempt < maxSummaryCorrections && len(problems) > 0; attempt++ {
		log.Printf("Summary breaks the rules (%s); asking for a correction", strings.Join(problems, "; "))
		conversation = append(conversation,
			anthropic.NewAssistantMessage(anthropic.NewTextBlock(reply)),
			anthropic.NewUserMessage(anthropic.NewTextBlock(
				correctionPrompt(sanitizeModelText(summary.Summary), speakers))),
		)
		corrected, correctedReply, err := requestSummary(&client, conversation, riskNotes)
		if err != nil {
			log.Printf("Summary correction failed: %s", err)
			break
		}
		summary, reply = corrected, correctedReply
		problems = summaryProblems(sanitizeModelText(summary.Summary), speakers)
	}

	info.Title = sanitizeModelText(summary.Title)
	info.Summary = sanitizeModelText(summary.Summary)
	info.RiskNotes = summary.RiskNotes
	info.SummaryProblems = problems

	return info, nil
}

// requestSummary sends a conversation to the model and parses its structured reply,
// returning the reply's text as well, so that it can be sent back in a follow-up
func requestSummary(
	client *anthropic.Client,
	conversation []anthropic.MessageParam,
	riskNotes bool,
) (messageSummary, string, error) {
	resp, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     anthropic.Model(getAnthropicModel()),
		MaxTokens: 16000,
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{
				Schema: summarySchema(riskNotes),
			},
		},
		Messages: conversation,
	})
	if err != nil {
		return messageSummary{}, "", fmt.Errorf("could not generate summary: %w", err)
	}

	// structured outputs guarantee the response validates against the schema, so the
	// text block is parseable JSON. There is deliberately no hand-rolled recovery here.
	var summary messageSummary
	for _, block := range resp.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			if err := json.Unmarshal([]byte(text.Text), &summary); err != nil {
				return messageSummary{}, "", fmt.Errorf("could not parse the model response: %w", err)
			}
			return summary, text.Text, nil
		}
	}
	return messageSummary{}, "", fmt.Errorf("the model returned no summary")
}

// maxSummaryWords is the most words a generated summary may have. The new website
// shows descriptions in a larger font, and one should fill about half a phone screen.
//
// The limit is for summaries generated from now on. Existing descriptions, in the
// sheet or on YouTube, are left as they are.
const maxSummaryWords = 65

// targetSummaryWords is the length the prompt asks for. It sits below the limit on
// purpose: a model treats a stated maximum as a rough size rather than a ceiling, so
// asking for 65 got 80 to 100. Aiming lower leaves the limit as a backstop.
const targetSummaryWords = 55

// maxSummaryCorrections is how many times a summary that breaks the rules is sent back
// to be corrected before it is kept as it is, with the problems shown
const maxSummaryCorrections = 2

// exampleSummary shows the model the length and voice wanted: 62 words in three short
// sentences, naming the speaker in full first and more briefly after. An example
// anchors length far better than a number does.
const exampleSummary = "Pastor Mary Peltz challenges us to pay attention to God's voice and let Him " +
	"question everything we think we know. Using Abraham's journey to Moriah, she shows how " +
	"growth that stretches us leads to a clear, God-given destiny we then surrender to Him. In " +
	"finances, relationships, and purpose, she calls us to partner with God, planting faith and " +
	"calling forth the harvest."

// summarySpeakers lists the speakers a summary must name, each exactly as the sheet
// gives it, honorific included: "Pastor Vern Peltz". SpeakerString joins several with
// a comma.
func summarySpeakers(speakerName string) []string {
	var speakers []string
	for name := range strings.SplitSeq(speakerName, ",") {
		if name = strings.TrimSpace(name); name != "" {
			speakers = append(speakers, name)
		}
	}
	return speakers
}

// summaryProblems lists the ways a generated summary breaks the rules for new
// descriptions: more than maxSummaryWords words, or a speaker not named in full,
// honorific included, at least once.
//
// That the full name is the first mention, as the prompt asks, is not checked: telling
// which later words refer to which speaker is not worth the guesswork.
func summaryProblems(summary string, speakers []string) []string {
	var problems []string

	if n := len(strings.Fields(summary)); n > maxSummaryWords {
		problems = append(problems, fmt.Sprintf("it is %d words long; the limit is %d", n, maxSummaryWords))
	}

	lower := strings.ToLower(summary)
	for _, speaker := range speakers {
		if !strings.Contains(lower, strings.ToLower(speaker)) {
			problems = append(problems, fmt.Sprintf("it does not name %q in full", speaker))
		}
	}

	return problems
}

// correctionPrompt asks the model to fix a summary that broke the rules. It says
// exactly what to do - how many words to cut, which name to write out - because "the
// limit is 65" alone was met with a summary trimmed by a word or two.
func correctionPrompt(summary string, speakers []string) string {
	var b strings.Builder
	b.WriteString("That summary breaks the rules:\n")

	if n := len(strings.Fields(summary)); n > maxSummaryWords {
		fmt.Fprintf(&b, "- It is %d words, and must be at most %d. Cut at least %d words, "+
			"aiming for about %d. Drop whole points rather than squeezing every point into fewer words.\n",
			n, maxSummaryWords, n-maxSummaryWords, targetSummaryWords)
	}

	lower := strings.ToLower(summary)
	for _, speaker := range speakers {
		if !strings.Contains(lower, strings.ToLower(speaker)) {
			fmt.Fprintf(&b, "- It does not name %q in full. Write the name exactly as given, "+
				"including the title, the first time you mention them.\n", speaker)
		}
	}

	b.WriteString("\nReply again with the same title and a corrected summary of two or three short sentences.\n")
	return b.String()
}

// literalUnicodeEscape matches a \uXXXX sequence that survived as literal text rather
// than being decoded into the character it names
var literalUnicodeEscape = regexp.MustCompile(`\\u[0-9a-fA-F]{4}`)

// sanitizeModelText cleans a generated title or description before it goes anywhere
// near a YouTube field.
//
// Two things have been observed coming back from the model, both on claude-opus-5:
//
//   - A literal — in the text. The model escaped the backslash in its own JSON,
//     so correct parsing yields the seven characters rather than an em dash. Pasted
//     into a description that is visible garbage.
//   - Newlines inside the summary. The description is assembled with blank lines
//     separating the summary from the resource links, so a summary that breaks its
//     own lines scrambles that layout.
//
// Neither is something the caller should have to think about, and neither is worth
// risking in front of a congregation, so both are fixed here rather than hoping the
// model behaves.
func sanitizeModelText(s string) string {
	// turn any literal \uXXXX back into the character it names
	s = literalUnicodeEscape.ReplaceAllStringFunc(s, func(match string) string {
		code, err := strconv.ParseInt(match[2:], 16, 32)
		if err != nil {
			return match
		}
		return string(rune(code))
	})

	// collapse every run of whitespace, newlines included, into a single space
	return strings.Join(strings.Fields(s), " ")
}

// summarySchema builds the JSON schema constraining the model's response
func summarySchema(riskNotes bool) map[string]any {
	properties := map[string]any{
		"title": map[string]any{
			"type":        "string",
			"description": "A title for the message, no longer than 6 words",
		},
		"summary": map[string]any{
			"type": "string",
			"description": fmt.Sprintf("A summary of two or three short sentences, about %d words "+
				"and never more than %d, in a casual voice suitable for social media, naming each "+
				"speaker in full on first mention", targetSummaryWords, maxSummaryWords),
		},
	}
	required := []string{"title", "summary"}

	if riskNotes {
		properties["risk_notes"] = map[string]any{
			"type":        "array",
			"description": "Passages a platform reviewer might act on. Empty if there are none.",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"policy_area": map[string]any{
						"type": "string",
						"description": "The area of concern, e.g. medical claims, elections, " +
							"hate speech. Use '" + policyAreaUnreliable + "' when the transcript " +
							"is too garbled to tell what was said.",
					},
					"quote": map[string]any{
						"type":        "string",
						"description": "The passage, quoted verbatim from the transcript",
					},
					"reasoning": map[string]any{
						"type":        "string",
						"description": "One line on why this passage could draw review",
					},
				},
				"required":             []string{"policy_area", "quote", "reasoning"},
				"additionalProperties": false,
			},
		}
		required = append(required, "risk_notes")
	}

	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

// shortSpeakerForms lists the shorter ways a summary may refer to a speaker after
// naming them in full: for "Pastor Mary Peltz", "Pastor", "Pastor Mary" or simply
// "Mary". Spelled out because, left to choose, the model reached for the surname
// alone - "Peltz emphasizes..." - which is not how the church speaks of its pastors.
//
// The first word is taken as a title only when there are three or more, so a name
// without one - "Mary Peltz" - offers just the first name.
func shortSpeakerForms(speaker string) []string {
	words := strings.Fields(speaker)
	switch {
	case len(words) >= 3:
		title, first := words[0], words[1]
		return []string{title, title + " " + first, first}
	case len(words) == 2:
		return []string{words[0]}
	}
	return nil
}

// joinQuoted quotes each item and joins them as a list: "a", "b" or "c"
func joinQuoted(items []string, conjunction string) string {
	quoted := make([]string, len(items))
	for index, item := range items {
		quoted[index] = fmt.Sprintf("%q", item)
	}
	if len(quoted) == 1 {
		return quoted[0]
	}
	return strings.Join(quoted[:len(quoted)-1], ", ") + " " + conjunction + " " + quoted[len(quoted)-1]
}

// buildSummaryPrompt assembles the prompt sent to the model.
//
// The transcript comes first and the instructions after it. A sermon transcript runs to
// ten thousand words or more, and instructions placed ahead of it are a long way back
// by the time the model starts writing - the summaries kept overrunning the word limit
// stated there. Only a line saying what is coming precedes the transcript.
func buildSummaryPrompt(speakerName string, xscript string, riskNotes bool) string {
	var b strings.Builder

	speakers := summarySpeakers(speakerName)

	b.WriteString("I'm going to give you a transcript of a Christian church sermon, delimited by " +
		"triple quotes. After it come instructions for what to write about it.\n")

	fmt.Fprintf(&b, "\n\"\"\" %s \"\"\"\n", xscript)

	if len(speakers) > 0 {
		fmt.Fprintf(&b, "\nThe speaker's name is %s.\n", speakerName)
	}

	fmt.Fprintf(&b, `
Suggest a single title and a single summary of this sermon.

The title will be no longer than 6 words.

The summary is for social media, in a casual voice. Write two or three short sentences, about %d words in total. It must never be more than %d words: that is a hard limit, and a longer summary is rejected. Leave detail out rather than cramming it in.

Here is an example summary of a different message. Match its length and voice, not its content or its speaker:
<example>
%s
</example>
`, targetSummaryWords, maxSummaryWords, exampleSummary)

	if len(speakers) > 0 {
		quoted := make([]string, len(speakers))
		for index, speaker := range speakers {
			quoted[index] = fmt.Sprintf("%q", speaker)
		}
		fmt.Fprintf(&b, `
The summary must name the speaker. The first time it mentions each speaker, it must use their full name exactly as written here, including any title such as "Pastor": %s. After that it may refer to them more briefly.
`, strings.Join(quoted, " and "))

		for _, speaker := range speakers {
			if forms := shortSpeakerForms(speaker); len(forms) > 0 {
				fmt.Fprintf(&b, "Acceptable shorter forms for %q include %s.\n", speaker, joinQuoted(forms, "or"))
			}
		}
	}

	if riskNotes {
		b.WriteString(`
You will also note any passages that a YouTube reviewer might act on, so a human can
decide where to publish this message.

These notes are observations, not verdicts, and a human makes the final call:
- Report only what is actually in the transcript. Quote each passage verbatim so the
  reviewer can check it in context.
- Explain your reasoning in one line, so that a judgment resting on an out-of-date
  understanding of platform policy is visible rather than silently applied.
- Do not score the message, do not recommend whether to upload it, and do not suggest
  another platform.
- Remember that a transcript loses tone and framing. A claim being quoted in order to
  be refuted reads the same as one being asserted. If you cannot tell which it is, say so.
- If the transcript is too garbled at some point to tell what was actually said, report
  that with the policy area "` + policyAreaUnreliable + `" rather than guessing.
- Precision matters far more than recall here. Ordinary preaching - including ordinary
  political and moral argument from a religious viewpoint - is not a concern and should
  not be reported. If nothing genuinely stands out, return an empty list. Do not pad
  the list with marginal observations to appear thorough.
`)
	}

	return b.String()
}

// printSummaryProblems writes how the generated summary breaks the rules for new
// descriptions, when it still does after a correction. It is kept rather than thrown
// away, so the operator can edit it before pasting.
func printSummaryProblems(b *box.Box, info *MessageInfo) {
	if len(info.SummaryProblems) == 0 {
		return
	}

	b.WarningSection("Summary check")
	b.WarningLine("The generated summary breaks the rules for new descriptions:")
	for _, problem := range info.SummaryProblems {
		b.WarningLabeled("• ", problem)
	}
	b.WarningLine("Edit it before pasting it anywhere, or run the message again for another.")
}

// printRiskNotes writes the advisory platform-risk notes as a section of their own,
// clearly separated from the fields that get pasted into YouTube. These are never
// pasted anywhere, so unlike those fields they are bordered and wrapped.
//
// The title has no "⚠": its display width varies between terminals, and one that draws
// it double width pushes the right border of that line out of line with the rest.
func printRiskNotes(b *box.Box, info *MessageInfo) {
	if !wantRiskNotes(info) {
		return
	}

	// only the title is coloured: the notes are advisory, and a whole section of
	// yellow would claim more alarm than they carry
	b.WarningSection("Platform risk notes (advisory - not for upload)")
	if len(info.RiskNotes) == 0 {
		b.Line("Nothing flagged.")
		return
	}

	for _, note := range info.RiskNotes {
		if note.PolicyArea == policyAreaUnreliable {
			b.Labeled("• ", "Transcript unreliable: "+note.Reasoning)
		} else {
			b.Labeled("• ", note.PolicyArea+": "+note.Reasoning)
		}
		b.Labeled("  ", fmt.Sprintf("\"%s\"", note.Quote))
	}
	b.Line("These are observations, not verdicts. Check each quote in context.")
}

// The file name once named the speaker too, with an initial next to the date or at the
// end - "-v" Vern, "-m" Mary, "-i" Igor, and so on. The spreadsheet row now supplies
// the speaker, and it is the record of who actually preached, so the initial had
// nothing left to contribute but a second answer that could disagree with the first.
//
// It was also a lookup table of six people in a program that is published for a church
// whose staff changes. Whoever preaches next does not need a code point allocated.
//
// Initials still appear in file names and the title parser skips over them - see
// leadingMarkers in messagetype.go. They just no longer mean anything.
