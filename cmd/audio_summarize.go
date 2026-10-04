package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/WordOfLifeMN/online/util"
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

The input file may be a transcript .text file or an .mp3 or .mp4 file. If you
provide an audio or video file, we will look for the transcript in the scratch
directory where the 'audio transcribe' command would have written it.

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
	xscriptPath = PromtUserForInputFile(xscriptPath, ".mp4", ".mp3", ".text")
	if xscriptPath == "" {
		fmt.Printf("Aborting")
		return nil
	}

	// resolve a video or audio file to the transcript it would have produced
	if strings.EqualFold(filepath.Ext(xscriptPath), ".mp4") {
		xscriptPath = getAudioPathFromVideoPath(xscriptPath)
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
	// the speaker comes from the file name or the operator
	speaker := getSpeakerInitialFromFileName(xscriptPath)
	if speaker == "" {
		speaker = PromptUserForSpeaker(guessSpeakerFromFileName(xscriptPath))
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
	xscript := strings.TrimSpace(string(xscriptBytes))
	if xscript == "" {
		return info, fmt.Errorf("transcript %s is empty", info.TranscriptPath)
	}

	if err := checkAnthropicCredentials(); err != nil {
		return info, err
	}

	riskNotes := wantRiskNotes(info)

	client := anthropic.NewClient(anthropicClientOptions()...)
	resp, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     anthropic.Model(getAnthropicModel()),
		MaxTokens: 16000,
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{
				Schema: summarySchema(riskNotes),
			},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewTextBlock(buildSummaryPrompt(info.SpeakerName, xscript, riskNotes)),
			),
		},
	})
	if err != nil {
		return info, fmt.Errorf("could not generate summary: %w", err)
	}

	// structured outputs guarantee the response validates against the schema, so the
	// text block is parseable JSON. There is deliberately no hand-rolled recovery here.
	var summary messageSummary
	for _, block := range resp.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			if err := json.Unmarshal([]byte(text.Text), &summary); err != nil {
				return info, fmt.Errorf("could not parse the model response: %w", err)
			}
			break
		}
	}

	info.Title = sanitizeModelText(summary.Title)
	info.Summary = sanitizeModelText(summary.Summary)
	info.RiskNotes = summary.RiskNotes

	return info, nil
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
			"type":        "string",
			"description": "A 3 sentence summary in a casual voice suitable for social media",
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

// buildSummaryPrompt assembles the prompt sent to the model
func buildSummaryPrompt(speakerName string, xscript string, riskNotes bool) string {
	var b strings.Builder

	fmt.Fprintf(&b, `I'm going to give you a transcript of a Christian church sermon that is delimited by triple quotes.
The speaker's name is %s.

You will suggest a single title and a single summary.

The title will be no longer than 6 words.
The summary should be 3 sentences in length and use a casual voice suitable for social media.
`, speakerName)

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

	fmt.Fprintf(&b, "\n\"\"\" %s \"\"\"\n", xscript)

	return b.String()
}

// printRiskNotes outputs the advisory platform-risk notes, clearly separated from the
// fields that get pasted into YouTube. These are never pasted anywhere.
func printRiskNotes(info *MessageInfo) {
	if !wantRiskNotes(info) {
		return
	}

	fmt.Printf("\n⚠ Platform risk notes (advisory - not for upload)\n")
	if len(info.RiskNotes) == 0 {
		fmt.Printf("  Nothing flagged.\n")
		return
	}

	for _, note := range info.RiskNotes {
		if note.PolicyArea == policyAreaUnreliable {
			fmt.Printf("  • Transcript unreliable: %s\n", note.Reasoning)
		} else {
			fmt.Printf("  • %s: %s\n", note.PolicyArea, note.Reasoning)
		}
		fmt.Printf("    \"%s\"\n", note.Quote)
	}
	fmt.Printf("  These are observations, not verdicts. Check each quote in context.\n")
}

// getSpeakerInitialFromFileName infers the speaker from an initial in the file name.
// Returns "" when the file name carries no initial, which is the caller's signal to
// look elsewhere.
//
//   - supported initials are V (Vern Peltz), M (Mary Peltz), J (Jim Isakson), I
//     (Igor Kondratyuk), A (Anthony Leong), T (Tania Kondratyuk)
//   - 2025-03-04 Message Title-[VMJIA].mp4
//   - 2025-03-04-[vmjia] Message Title.mp4
//   - 2025-03-04-p[vmjia] Message Title.mp4
//   - 2025-03-04p-[vmjia] Message Title.mp4
func getSpeakerInitialFromFileName(filePath string) string {
	names := map[string]string{
		"V": "Pastor Vern Peltz",
		"M": "Pastor Mary Peltz",
		"J": "Jim Isakson",
		"I": "Pastor Igor Kondratyuk",
		"T": "Pastor Tania Kondratyuk",
		"A": "Anthony Leong",
	}

	// test for each possible location of the initials
	ucFilePath := strings.ToUpper(filePath)
	for initials, n := range names {
		// initial at end of file name: "2025-03-09 Title-v.mp4"
		re := fmt.Sprintf("-%s\\....$", initials)
		if match, err := regexp.MatchString(re, ucFilePath); err == nil && match {
			return n
		}
		// initial following date with optional prayer "p": "2025-03-09-[p]v[p] Title.mp4"
		re = fmt.Sprintf("-[0-9][0-9]-[0-9][0-9]-P?%sP? ", initials)
		if match, err := regexp.MatchString(re, ucFilePath); err == nil && match {
			return n
		}
		// initial following prayer "p": "2025-03-09pv Title.mp4"
		re = fmt.Sprintf("-[0-9][0-9]-[0-9][0-9]P%s ", initials)
		if match, err := regexp.MatchString(re, ucFilePath); err == nil && match {
			return n
		}
	}

	return ""
}

// guessSpeakerFromFileName is the last-resort default for the speaker prompt, used
// only when neither the file name nor the spreadsheet says who spoke. A prayer is
// more often Mary, anything else more often Vern.
func guessSpeakerFromFileName(filePath string) string {
	if match, err := regexp.MatchString("-[0-9][0-9]p ", filePath); err == nil && match {
		return "Mary"
	}
	return "Vern"
}
