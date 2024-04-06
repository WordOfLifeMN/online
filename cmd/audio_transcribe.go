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
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/WordOfLifeMN/online/util"
	"github.com/spf13/cobra"
)

// audioTranscribeCmd represents the command to transcribe audio to text
var audioTranscribeCmd = &cobra.Command{
	Use:   "transcribe audio-file",
	Short: "Transcribe the audio track to text",
	Long: `Takes an already extracted audio track and transcribes it to English text.

The input file must be .mp3 and the output will be generated as both a text and
a vtt file in the 'xscript' sub-directory.

The resulting text files will be uploaded to AWS S3 to the wordoflife.mn.audio bucket
as 
- s3://wordoflife.mn.audio/{year}/xscript/{txt-file-name}
- s3://wordoflife.mn.audio/{year}/xscript/{vtt-file-name}

Requires 'whisper' and 'aws' be installed and accessible on the path.`,
	RunE: audioTranscribe,
}

type xscriptInfo struct {
	path    string
	s3URL   string
	httpURL string
}

func init() {
	audioCmd.AddCommand(audioTranscribeCmd)

	audioTranscribeCmd.Args = cobra.MaximumNArgs(1)
}

func audioTranscribe(cmd *cobra.Command, args []string) error {
	initLogging()

	var err error
	var audioPath string

	// get the input video file
	if len(args) == 1 {
		audioPath = args[0]
	}
	audioPath = getInputAudio(audioPath)
	if audioPath == "" {
		fmt.Printf("Aborting")
		return nil
	}

	textPaths, err := transcribeAudio(audioPath)
	if err != nil {
		return err
	}
	if len(textPaths) == 0 {
		return fmt.Errorf("transcribing %s returned no output text", audioPath)
	}

	if err = uploadTranscriptionsToS3(textPaths); err != nil {
		return err
	}
	return nil
}

// transcribeAudio uses whisper to transcribe the audio and returns the .txt and .vtt
// files
func transcribeAudio(audioPath string) ([]string, error) {
	// delete any existing transcription files and collect the names
	var xscriptPaths []string
	for _, ext := range []string{".txt", ".vtt", ".srt", ".tsv", ".json"} {
		xscriptPaths = append(xscriptPaths, getTranscribePathFromAudioPath(audioPath, ext))
	}
	for i, p := range xscriptPaths {
		if err := deleteExistingFile(p, i == 0); err != nil {
			if strings.HasSuffix(err.Error(), "exists") {
				// user doesn't want to overwrite. assume the existing transcript
				// is valid and return no errors
				return xscriptPaths[0:2], nil
			}
			return nil, err
		}
	}

	// output status
	fmt.Printf("Transcribing: %s\n", filepath.Base(audioPath))
	fmt.Printf("     to .txt: %s\n", "xscript\\"+filepath.Base(xscriptPaths[0]))
	fmt.Printf("    and .vtt: %s\n", "xscript\\"+filepath.Base(xscriptPaths[1]))

	cmd := exec.Command("whisper",
		"--output_dir", filepath.Dir(xscriptPaths[0]),
		"--fp16", "False",
		"--model", "tiny",
		// "--model", "small",
		// "--model", "medium",
		"--output_format", "all",
		"--language", "English",
		audioPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Print(cmd.String())
	if err := cmd.Run(); err != nil {
		fmt.Printf("Unable to transcribe audio: %s\n", err)
		return nil, err
	}

	// delete the files we don't use
	for _, file := range xscriptPaths[2:] {
		os.Remove(file)
	}

	return xscriptPaths[0:2], nil
}

func uploadTranscriptionsToS3(xscriptPaths []string) error {
	s3Bucket := "wordoflife.mn.audio"

	xscripts := []xscriptInfo{}
	for _, p := range xscriptPaths {
		if !util.DoesPathExist(p) {
			return fmt.Errorf("cannot find file %s", p)
		}

		// compute all the file references
		year := filepath.Base(p)[0:4]
		xscripts = append(xscripts, xscriptInfo{
			path:  p,
			s3URL: fmt.Sprintf("s3://%s/%s/xscript/%s", s3Bucket, year, filepath.Base(p)),
			httpURL: fmt.Sprintf("https://s3.us-west-2.amazonaws.com/%s/%s/xscript/%s",
				s3Bucket, year, strings.ReplaceAll(filepath.Base(p), " ", "+")),
		})
	}

	// write the expectations
	for _, info := range xscripts {
		fmt.Printf("Uploading: %s\n", filepath.Base(info.path))
		fmt.Printf("       to: %s\n", "xscript\\"+filepath.Base(info.s3URL))
	}
	fmt.Printf("╭───────────────────────────────────────────────────────────────────────────────────┄┄\n")
	fmt.Printf("│ Public HTML reference for transcription files\n")
	for _, info := range xscripts {
		fmt.Printf("%s\n", info.httpURL)
	}
	fmt.Printf("╰───────────────────────────────────────────────────────────────────────────────────┄┄\n")

	for _, info := range xscripts {
		cmd := exec.Command("aws", "s3", "cp", info.path, info.s3URL)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("Unable to upload file to S3: %s\n", err)
			return err
		}
	}

	return nil
}
