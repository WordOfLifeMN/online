/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/WordOfLifeMN/online/util"
	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type MessageInfo struct {
	SpeakerName     string
	speakerPronouns string // "male" or "female"
	Title           string
	Summary         string
}

// audioSummarizeCmd represents the command to summarize the audio transcript to titles
// and descriptions
var audioSummarizeCmd = &cobra.Command{
	Use:   "summarize transcript-file|audio-file|video-file",
	Short: "Summarize the transcription of an audio file",
	Long: `Takes a transcription of an audio file and produces a suggested title and description.

The input file may be a transcripted .txt file or an .mp3 or .mp4 file. If you
provide an audio or video file, we will look for the transcript in the xscript
directory where the 'audio transcribe' command would have written it.

Requires an OpenAI API key be available in the configuration file.`,
	RunE: xscriptSummarize,
}

func init() {
	audioCmd.AddCommand(audioSummarizeCmd)

	audioSummarizeCmd.Args = cobra.MaximumNArgs(1)
}

func xscriptSummarize(cmd *cobra.Command, args []string) error {
	initLogging()

	var xscriptPath string

	// get the input video file
	if len(args) == 1 {
		xscriptPath = args[0]
	}
	xscriptPath = PromtUserForInputFile(xscriptPath, ".mp4", ".mp3", ".txt")
	if xscriptPath == "" {
		fmt.Printf("Aborting")
		return nil
	}

	if strings.EqualFold(filepath.Ext(xscriptPath), ".mp4") {
		xscriptPath = getAudioPathFromVideoPath(xscriptPath)
	}
	if strings.EqualFold(filepath.Ext(xscriptPath), ".mp3") {
		xscriptPath = getTranscribePathFromAudioPath(xscriptPath, ".txt")
	}
	if !util.DoesPathExist(xscriptPath) {
		return fmt.Errorf("file not found: %s", xscriptPath)
	}
	if util.IsDirectory(xscriptPath) {
		return fmt.Errorf("must be a file, not a directory: %s", xscriptPath)
	}
	// fmt.Printf("TODO(km) transcription file: %s\n", xscriptPath)
	// fmt.Printf("TODO(km) OpenAI key: %s\n", viper.GetString("openai-key"))

	xscriptSample, err := xscriptExtractSample(xscriptPath, 12_000)
	if err != nil {
		return err
	}
	speakerName, speakerPronouns := getXScriptSpeaker(xscriptPath)
	// fmt.Printf("TODO(km) XScript speaker %s\n XScript sample: %s\n",speakerName, xscriptSample)

	// TODO(km)
	info := &MessageInfo{
		SpeakerName:     speakerName,
		speakerPronouns: speakerPronouns,
	}
	if info, err = generateMessageInfo(xscriptSample, info); err != nil {
		return err
	}

	fmt.Printf("\n\n%s\n", util.ToJSON(info))

	return nil
}

// generateMessageInfo takes a transcript and some basic message information,
// and fills out the rest of the message information: title and summary
func generateMessageInfo(xscript string, info *MessageInfo) (*MessageInfo, error) {
	client := openai.NewClient(viper.GetString("openai-key"))
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role: openai.ChatMessageRoleUser,
					// Content: "I'm going to give you a transcript of a Christian church sermon " +
					// 	"that is delimited by triple quotes." +
					// 	"The speaker's name is " + info.SpeakerName +
					// 	" and they use the " + info.speakerPronouns + " gender. " +
					// 	"You will suggest a title no longer than 6 words " +
					// 	"You will also suggest a three sentence summary suitable for social media. " +
					// 	"You will output the results formatted as a JSON object " +
					// 	"containing two fields named \"title\" and \"summary\"\n\n" +
					// 	"\"\"\"" + xscript + "\"\"\"",
					Content: fmt.Sprintf(`
I'm going to give you a transcript of a Christian church sermon that is delimited by triple quotes.
The speaker's name is %s and use the pronouns %s.

You will suggest a single title and a single summary.

The title will be no longer than 6 words.
The summary will be three sentences and use a casual voice suitable for social media. 

You will output the results formatted as a JSON object containing two fields named "title" and "summary"

""" %s """
`, info.SpeakerName, info.speakerPronouns, xscript),
				},
			},
		},
	)
	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return info, err
	}

	fmt.Println(resp.Choices[0].Message.Content)
	fmt.Printf("\n\n%s\n", util.ToJSON(resp))

	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), info)
	return info, err
}

// CURL
// 	curl https://api.openai.com/v1/chat/completions \
//   -H "Content-Type: application/json" \
//   -H "Authorization: Bearer $OPENAI_API_KEY" \
//   -d '{
//   "model": "gpt-3.5-turbo",
//   "messages": [
//     {
//       "role": "user",
//       "content": "tell me a limerick about gorillas"
//     },
//     {
//       "role": "assistant",
//       "content": "There once was a big gorilla named Lou,\nWho loved to swing and play peek-a-boo.\nHe was strong and so hulky,\nBut really quite funky,\nAnd he always stole the show at the zoo."
//     }
//   ],
//   "temperature": 1,
//   "max_tokens": 256,
//   "top_p": 1,
//   "frequency_penalty": 0,
//   "presence_penalty": 0
// }'

// xscriptExtractSample extracts a sample from the sermon transcript consisting
// of about the requested number of tokens. The sample will be extracted from the
// middle of the transcript and will attempt to contain whole sentences
func xscriptExtractSample(xscriptPath string, tokenCount int) (string, error) {
	// read the entire transcript as a string
	xscriptBytes, err := os.ReadFile(xscriptPath)
	if err != nil {
		return "", err
	}
	xscript := string(xscriptBytes)

	// extract the sample from the middle of the transcript
	return extractSampleFromMiddle(xscript, tokenCount), nil
}

// extractSampleFromMiddle extracts a sample from the string consisting
// of about the requested number of tokens. The sample will be extracted from the
// middle of the string and will attempt to contain whole sentences
func extractSampleFromMiddle(s string, tokenCount int) string {
	// a token is approximately 4 characters
	charCount := tokenCount * 4

	// we're going to find roughly the middle of the transcript, so
	// 1. start at the middle of the transcript
	// 2. back up about 1/2 the characters
	// 3. look backwards for the start of a sentence
	// 4. advance the total number of characters
	// 5. look backwards for the end of a sentence

	// start at the middle of the transcript
	start := len(s) / 2

	// initial estimate is around this center point
	start -= charCount / 2
	end := start + charCount

	// look for the start of a sentence about half the tokens back
	start = findPreviousSentenceStart(s, start, 128)

	// find the start of the sentence at the end
	end = findPreviousSentenceStart(s, end, 64)

	return strings.TrimSpace(s[start:end])
}

// findPreviousSentenceStart starts at the pos in string s and backs up to find the
// start of a sentence. It will not back up more than window runes. Sentences end with
// periods, queries, or bangs
func findPreviousSentenceStart(s string, pos, window int) int {
	sentStart := findStartOfSentence(s, pos)
	if pos-sentStart > window {
		return findStartOfWord(s, pos)
	}
	return sentStart
}

// findStartOfSentence returns the index of the first character of the sentence that p
// is currently in.
func findStartOfSentence(str string, initPos int) int {
	// we want an array of runes to test
	s := []rune(str)

	if initPos >= len(s) {
		return len(s)
	}

	// iterate backward looking for the punctuation at the end of a sentence.
	// Punctuation at the end of a sentence is a dot, bang, or query that is
	// preceded by non-whitespace, and followed by whitespace
	for p := initPos; ; p-- {
		if p < 1 {
			return 0
		}
		c := s[p]
		if c != '.' && c != '?' && c != '!' {
			continue
		}

		if p < initPos && !unicode.IsSpace(s[p-1]) && unicode.IsSpace(s[p+1]) {
			// found the end of a sentence, iterate forward to find the start of the next sentence
			for p++; p < len(s) && unicode.IsSpace(s[p]); p++ {
			}
			return p
		}
	}
}

func findStartOfWord(str string, initPos int) int {
	s := []rune(str)

	if initPos >= len(s) {
		return len(s)
	}

	for p := initPos; ; p-- {
		if p < 1 {
			return 0
		}
		if unicode.IsSpace(s[p-1]) {
			return p
		}
	}
}

// getXScriptSpeaker attempts to infer the speaker name from the file name,
// and prompts the user if necessary
func getXScriptSpeaker(xscriptPath string) (name, gender string) {
	// look for the magic tags "-v" or "-m" at the end of the file name for vern and mary,
	switch {
	case strings.Contains(strings.ToUpper(xscriptPath), "-V."):
		return "Pastor Vern Peltz", "he/him"
	case strings.Contains(strings.ToUpper(xscriptPath), "-M."):
		return "Pastor Mary Peltz", "she/her"
	}

	defaultSpeaker := "Vern"
	if match, err := regexp.MatchString("[0-9][0-9]p ", xscriptPath); err == nil && match {
		defaultSpeaker = "Mary"
	}
	return PromptUserForSpeakerAndGender(defaultSpeaker)
}
