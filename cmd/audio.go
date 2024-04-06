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
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/WordOfLifeMN/online/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type MessageInfo struct {
	VideoPath       string
	AudioPath       string
	AudioURL        string
	TranscriptPath  string
	SpeakerName     string
	SpeakerPronouns string // "he/him", "she/her"
	Title           string
	Summary         string

	// times
	ExtractTime    util.StopWatch
	UploadTime     util.StopWatch
	TranscribeTime util.StopWatch
	SummaryTime    util.StopWatch
}

// audioCmd represents the command to extract and process audio
var audioCmd = &cobra.Command{
	Use:   "audio",
	Short: "Extract, upload, and process audio",
	Long: `Process audio from a service message.

Given one or more video files, will do the following for each file:
1. Extract the audio and save it as *.mp3
2. Upload the audio to s3://wordoflife.mn.audio/year
3. Transcribe the audio with Whisper to xscript/*.txt
4. Send the transcript to ChatGPT to get a suggested title and summary`,
	RunE: audio,
}

func init() {
	rootCmd.AddCommand(audioCmd)

	rootCmd.PersistentFlags().String("speaker", "", "Name of the speaker")
	viper.BindPFlag("speaker", rootCmd.PersistentFlags().Lookup("speaker"))

	rootCmd.PersistentFlags().String("pronouns", "he/him", "Pronouns of the speaker, like he/him, she/her, they/them")
	viper.BindPFlag("pronouns", rootCmd.PersistentFlags().Lookup("pronouns"))
}

func audio(cmd *cobra.Command, args []string) error {
	initLogging()

	var infos []*MessageInfo

	if len(args) == 0 {
		// prompt user for video files until there are no more
		for {
			videoPath := getInputVideo("")
			if videoPath == "" {
				if len(infos) == 0 {
					// no videos at all
					fmt.Println("No input files, exiting")
					return nil
				}
				// user is done inputting videos
				break
			}

			info := MessageInfo{
				VideoPath:       videoPath,
				SpeakerName:     viper.GetString("speaker"),
				SpeakerPronouns: viper.GetString("pronoun"),
			}
			if info.SpeakerName == "" {
				info.SpeakerName, info.SpeakerPronouns = getSpeakerFromFileName(videoPath)
			}

			infos = append(infos, &info)
		}
	} else {
		// validate the video file arguments
		for _, arg := range args {
			arg = getInputVideo(arg)

			info := MessageInfo{
				VideoPath:       arg,
				SpeakerName:     viper.GetString("speaker"),
				SpeakerPronouns: viper.GetString("pronoun"),
			}
			if info.SpeakerName == "" {
				info.SpeakerName, info.SpeakerPronouns = getSpeakerFromFileName(arg)
			}
			infos = append(infos, &info)
		}
	}

	// process all the video files
	//	err := processAllVideosSequentially(infos)
	err := processAllVideosInEditingPriority(infos)

	// output the results of all processing
	for index, info := range infos {
		printMessageInfo(index, info)
	}

	return err
}

// processAllVideosSequentially processes each video sequentially,
// updating the message information records as it goes
func processAllVideosSequentially(infos []*MessageInfo) error {
	var errs []error
	for _, info := range infos {
		if err := processOneAudio(info); err != nil {
			errs = append(errs, err)
		}
	}

	// collect the errors to return later
	return errors.Join(errs...)
}

// processOneAudio handles the minimal processing of one audio file.
// If the audio path doesn't exist, will extract the audio from the video.
// If the transcript doesn't exist, will transcribe the audio.
// Will send the audio transcript to ChatGPT.
// All information will be recorded in the passed in info record.
func processOneAudio(info *MessageInfo) error {
	var err error

	if info.VideoPath == "" {
		return fmt.Errorf("no video file was provided to extract audio from. aborting")
	}

	// extract the audio from the video if needed
	info.AudioPath = getAudioPathFromVideoPath(info.VideoPath)
	if !util.IsFile(info.AudioPath) {
		info.ExtractTime = util.NewStopWatch()
		info.AudioPath, err = extractAudioFromVideo(info.VideoPath)
		info.ExtractTime.Stop()
		if err != nil {
			return err
		}

		// upload the audio to S3
		info.UploadTime = util.NewStopWatch()
		info.AudioURL, err = uploadAudioToS3(info.AudioPath)
		info.UploadTime.Stop()
		if err != nil {
			return err
		}
	}
	info.AudioURL = getAudioHTTPURL(info.AudioPath)

	// transcribe the audio file if needed
	info.TranscriptPath = getTranscribePathFromAudioPath(info.AudioPath, ".txt")
	if !util.IsFile(info.TranscriptPath) {
		info.TranscribeTime = util.NewStopWatch()
		xscripts, err := transcribeAudio(info.AudioPath)
		info.TranscribeTime.Stop()
		if err != nil {
			return err
		}
		info.TranscriptPath = xscripts[0]
	}

	// generate the message summary
	info.SummaryTime = util.NewStopWatch()
	xscriptSample, err := xscriptExtractSample(info.TranscriptPath, 12_000)
	if err != nil {
		return err
	}
	if _, err = generateMessageSummary(xscriptSample, info); err != nil {
		return err
	}
	info.SummaryTime.Stop()

	return nil
}

// processAllVideosPriority handles processing all the videos in a way that makes best
// use of editing time. It first does all the extraction and uploading, outputting the
// relevant links, then does the transcoding and summarization later since that takes
// the most time
func processAllVideosInEditingPriority(infos []*MessageInfo) error {
	var err error

	// do all the audio extraction
	for _, info := range infos {
		if info.VideoPath == "" {
			return fmt.Errorf("no video file was provided to extract audio from. aborting")
		}

		// extract the audio from the video if needed
		info.AudioPath = getAudioPathFromVideoPath(info.VideoPath)
		info.AudioURL = getAudioHTTPURL(info.AudioPath)
		if !util.IsFile(info.AudioPath) {
			info.ExtractTime = util.NewStopWatch()
			info.AudioPath, err = extractAudioFromVideo(info.VideoPath)
			info.ExtractTime.Stop()
			if err != nil {
				return err
			}

			// upload the audio to S3
			info.UploadTime = util.NewStopWatch()
			info.AudioURL, err = uploadAudioToS3(info.AudioPath)
			info.UploadTime.Stop()
			if err != nil {
				return err
			}
		} else {
			// just print the URL
			fmt.Printf("╭───────────────────────────────────────────────────────────────────────────────────┄┄\n")
			fmt.Printf("│ Public HTML reference for audio file\n")
			fmt.Printf("%s\n", info.AudioURL)
			fmt.Printf("╰───────────────────────────────────────────────────────────────────────────────────┄┄\n")
		}
	}

	// now transcribe and summarize everything

	for _, info := range infos {
		// transcribe the audio file if needed
		info.TranscriptPath = getTranscribePathFromAudioPath(info.AudioPath, ".txt")
		if !util.IsFile(info.TranscriptPath) {
			info.TranscribeTime = util.NewStopWatch()
			xscripts, err := transcribeAudio(info.AudioPath)
			info.TranscribeTime.Stop()
			if err != nil {
				return err
			}
			info.TranscriptPath = xscripts[0]
		}

		// generate the message summary
		info.SummaryTime = util.NewStopWatch()
		xscriptSample, err := xscriptExtractSample(info.TranscriptPath, 12_000)
		if err != nil {
			return err
		}
		if _, err = generateMessageSummary(xscriptSample, info); err != nil {
			return err
		}
		info.SummaryTime.Stop()
	}

	return nil
}

func printMessageInfo(index int, info *MessageInfo) {
	fmt.Printf("Message #%d\n", index+1)
	fmt.Printf("Video file   : %s\n", filepath.Base(info.VideoPath))
	fmt.Printf("Audio file   : %s\n", filepath.Base(info.AudioPath))
	fmt.Printf("Transcription: %s\n", "xscript\\"+filepath.Base(info.TranscriptPath))
	fmt.Printf("╭───────────────────────────────────────────────────────────────────────────────────┄┄\n")
	fmt.Printf("│ Audio URL:\n")
	fmt.Printf("%s\n", info.AudioURL)
	fmt.Printf("│ Speaker  : %s\n", info.SpeakerName)
	fmt.Printf("│ Title    : %s\n", info.Title)
	fmt.Printf("│ Summary  :\n")
	fmt.Printf("%s\n", info.Summary)
	fmt.Printf("╰───────────────────────────────────────────────────────────────────────────────────┄┄\n")
	log.Printf("Timeline: Extract = %s, Upload = %s, Transcribe = %s, Summarize = %s\n",
		info.ExtractTime.Elapsed(), info.UploadTime.Elapsed(), info.TranscribeTime.Elapsed(),
		info.SummaryTime.Elapsed())
	fmt.Println()
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
