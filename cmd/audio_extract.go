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

// audioExtractCmd represents the command to extract audio from video
var audioExtractCmd = &cobra.Command{
	Use:   "extract video-file",
	Short: "Extract audio track from video",
	Long: `Extracts the audio as an mp3 file from the mp4 video file.

The input file must be .mp4 and the output will be generated as an .mp3 file
in the same directory as the input. If the output file already exsits, you will
be prompted to overwrite it.

After extraction, the audio file will be uploaded to AWS S3's wordoflife.mn.audio
bucket as s3://wordoflife.mn.audio/{year}/{mp3-file-name} .

Requires 'ffmpeg' and 'aws' be installed and accessible on the path.`,
	RunE: audioExtract,
}

func init() {
	audioCmd.AddCommand(audioExtractCmd)

	audioExtractCmd.Args = cobra.MaximumNArgs(1)
}

func audioExtract(cmd *cobra.Command, args []string) error {
	initLogging()

	var err error
	var videoPath string

	// get the input video file
	if len(args) == 1 {
		videoPath = args[0]
	}
	videoPath = getInputVideo(videoPath)
	if videoPath == "" {
		fmt.Printf("Aborting")
		return nil
	}

	audioPath, err := extractAudioFromVideo(videoPath)
	if err != nil {
		return err
	}

	if _, err = uploadAudioToS3(audioPath); err != nil {
		return err
	}
	return nil
}

func extractAudioFromVideo(videoPath string) (string, error) {
	audioPath := getAudioPathFromVideoPath(videoPath)
	if err := deleteExistingFile(audioPath, true); err != nil {
		return "", err
	}

	// compute trim length based on file name
	trimLen := 9.95
	if strings.Contains(audioPath, " FF ") {
		trimLen = 9.9
	} else if strings.Contains(audioPath, " CORE ") {
		trimLen = 30.0
	}

	// output status
	fmt.Printf("Extracting: %s\n", filepath.Base(audioPath))
	fmt.Printf("      from: %s\n", filepath.Base(videoPath))
	fmt.Printf("  trimming: %0.1fs\n", trimLen)

	cmd := exec.Command("ffmpeg",
		"-hide_banner",
		"-loglevel", "warning",
		"-stats",
		"-i", videoPath,
		"-ss", fmt.Sprintf("%f", trimLen),
		audioPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Print(cmd.String())
	if err := cmd.Run(); err != nil {
		fmt.Printf("Unable to extract audio: %s\n", err)
		return "", err
	}

	return audioPath, nil
}

// uploadAudioToS3 uploads the provided audio to the S3 bucket
// and returns the HTTP URL for the uploaded file.
func uploadAudioToS3(audioPath string) (string, error) {
	if !util.DoesPathExist(audioPath) {
		return "", fmt.Errorf("cannot find file %s", audioPath)
	}

	// compute all the file references
	s3URL := getAudioS3URL(audioPath)
	url := getAudioHTTPURL(audioPath)

	// fmt.Printf("Uploading: %s\n", audioPath)
	// fmt.Printf("       to: %s\n", s3URL)
	fmt.Printf("╭───────────────────────────────────────────────────────────────────────────────────┄┄\n")
	fmt.Printf("│ Public HTML reference for audio file\n")
	fmt.Printf("%s\n", url)
	fmt.Printf("╰───────────────────────────────────────────────────────────────────────────────────┄┄\n")

	cmd := exec.Command("aws", "s3", "cp", audioPath, s3URL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Unable to upload audio to S3: %s\n", err)
		return "", err
	}

	return url, nil
}

func getAudioS3URL(audioPath string) string {
	audioName := filepath.Base(audioPath)
	s3Bucket := "wordoflife.mn.audio"
	return fmt.Sprintf("s3://%s/%s/%s", s3Bucket, audioName[0:4], audioName)
}

func getAudioHTTPURL(audioPath string) string {
	audioName := filepath.Base(audioPath)
	s3Bucket := "wordoflife.mn.audio"
	return fmt.Sprintf("https://s3.us-west-2.amazonaws.com/%s/%s/%s",
		s3Bucket, audioName[0:4], strings.ReplaceAll(audioName, " ", "+"))
}
