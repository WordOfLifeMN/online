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
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/WordOfLifeMN/online/util"
	"github.com/spf13/cobra"
)

// audioCmd represents the command to extract and process audio
var audioCmd = &cobra.Command{
	Use:   "audio",
	Short: "Extract, upload, and process audio",
	Long:  `Process audio from a service message`,
	RunE:  audio,
}

func init() {
	rootCmd.AddCommand(audioCmd)
}

func audio(cmd *cobra.Command, args []string) error {
	initLogging()

	fmt.Println("Use 'online help audio' for help")

	return nil
}

// getInputVideo finds an appropriate video file for processing. The input
// should be the command line argument. If it is empty, then the user will
// be prompted to enter a path. The path will be verified to be an .mp4 and
// exist.
//
// If the return string is empty then the user cancelled the operation.
func getInputVideo(videoPath string) string {
	return PromtUserForInputFile(videoPath, ".mp4")
}

// getInputAudio finds an appropriate audio file for processing. The input
// should be the command line argument. If it is empty, then the user will
// be prompted to enter a path. The path will be verified to be an .mp3 and
// exist.
//
// If the return string is empty then the user cancelled the operation.
func getInputAudio(audioPath string) string {
	return PromtUserForInputFile(audioPath, ".mp3")
}

// PromtUserForInputFile finds an appropriate file for processing.
// The input path should be the command line argument, and will be used as the default.
// If it is empty, then the user will be prompted to enter a path.
// The path will be verified to be the right requiredExt and exist.
// When prompting, the fileType will be the type of file requested
//
// If the return string is empty then the user cancelled the operation.
func PromtUserForInputFile(path string, allowedExts ...string) string {
	var err error

	// ensure all extensions start with a dot
	for i, ext := range allowedExts {
		allowedExts[i] = "." + strings.TrimPrefix(ext, ".")
	}

	reader := bufio.NewReader(os.Stdin)

	// get the input video file
	for filePath := strings.Trim(path, "\"' \r\n"); ; filePath = "" {
		if filePath == "" {
			fmt.Println("Enter path or drag the file to use:")
			filePath, err = reader.ReadString('\n')
			filePath = strings.Trim(filePath, "\"' \r\n")
			if err != nil {
				// report error and try again
				fmt.Println(err.Error())
				continue
			}
			if filePath == "" {
				// user canceled operation
				return ""
			}
		}

		// verify this is the right type of file
		allowed := false
		for _, ext := range allowedExts {
			if strings.EqualFold(filepath.Ext(filePath), ext) {
				allowed = true
				break
			}
		}
		if !allowed {
			fmt.Printf("Input file must be one of %v.\nPlease try again\n", allowedExts)
			continue
		}

		// verify this file exists
		if !util.DoesPathExist(filePath) {
			fmt.Printf("Unable to find file: %s\nPlease try to drag the file into this window\n",
				filePath)
			continue
		}
		if util.IsDirectory(filePath) {
			fmt.Printf("Input must be a file, not a directory\n")
			continue
		}

		return filePath
	}
}

func getAudioPathFromVideoPath(videoPath string) string {
	return videoPath[:len(videoPath)-4] + ".mp3"
}

func getTranscribePathFromAudioPath(audioPath string, textExt string) string {
	textExt = "." + strings.TrimPrefix(textExt, ".")
	audioName := filepath.Base(audioPath)
	return fmt.Sprintf("%s/xscript/%s",
		filepath.Dir(audioPath), audioName[:len(audioName)-4]+textExt)
}

// deleteExistingFile deletes an existing file if it exists.
// If prompt is true, then the user will be asked whether
// they want to overwrite it before deleting it
func deleteExistingFile(audioPath string, prompt bool) error {
	if !util.DoesPathExist(audioPath) {
		// could not find file, so file already doesn't exist
		return nil
	}

	// audio does exist
	if prompt {
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("File %s exists.\nDo you want to overwrite it [Y/n]?",
			audioPath)
		a, _ := reader.ReadString('\n')
		a = strings.Trim(a, "\"' \r\n")
		if a == "" {
			a = "y"
		}
		if a != "y" {
			return fmt.Errorf("file %s exists", audioPath)
		}
	}

	// delete the file
	if err := os.Remove(audioPath); err != nil {
		return fmt.Errorf("unable to delete %s: %s", audioPath, err)
	}

	return nil
}

func PromptUserForSpeakerAndGender(defaultSpeaker string) (string, string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Who is the speaker [Default:%s/Vern(v)/Mary(m)/Other]?\n", defaultSpeaker)
	name, _ := reader.ReadString('\n')
	name = strings.Trim(name, "\"' \r\n")

	if name == "" {
		name = defaultSpeaker
	}
	switch strings.ToUpper(name) {
	case "V", "VERN":
		return "Pastor Vern Peltz", "he/him"
	case "M", "MARY":
		return "Pastor Mary Peltz", "she/her"
	default:
		return name, "they/them"
	}
}
