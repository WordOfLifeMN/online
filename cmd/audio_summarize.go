package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/WordOfLifeMN/online/util"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// defaultAnthropicModel is the model used to generate titles and summaries when none
// is configured. The task is small - a transcript in, a six-word title and three
// sentences out - so a cheaper model may well do as well; set 'anthropic-model' to
// try one.
const defaultAnthropicModel = "claude-opus-5"

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

	info := &MessageInfo{
		TranscriptPath: xscriptPath,
		SpeakerName:    getSpeakerFromFileName(xscriptPath),
	}
	var err error
	if info, err = generateMessageSummary(info); err != nil {
		return err
	}

	fmt.Printf("\n\n%s\n", util.ToJSON(info))

	return nil
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

	riskNotes := wantRiskNotes(info)

	client := anthropic.NewClient()
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

	info.Title = summary.Title
	info.Summary = summary.Summary
	info.RiskNotes = summary.RiskNotes

	return info, nil
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

// getSpeakerFromFileName attempts to infer the speaker name from the file name,
// and prompts the user if necessary
//   - supported initials are V (Vern Peltz), M (Mary Peltz), J (Jim Isakson), I
//     (Igor Kondratyuk), A (Anthony Leong), T (Tania Kondratyuk)
//   - 2025-03-04 Message Title-[VMJIA].mp4
//   - 2025-03-04-[vmjia] Message Title.mp4
//   - 2025-03-04-p[vmjia] Message Title.mp4
//   - 2025-03-04p-[vmjia] Message Title.mp4
func getSpeakerFromFileName(filePath string) string {
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

	defaultSpeaker := "Vern"
	if match, err := regexp.MatchString("-[0-9][0-9]p ", filePath); err == nil && match {
		defaultSpeaker = "Mary"
	}
	return PromptUserForSpeaker(defaultSpeaker)
}
