package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// defaultScratchDir is where intermediate audio and transcript files are written when
// no 'scratch-dir' is configured. These files are deleted once they have served their
// purpose, so they deliberately do not live beside the source video.
const defaultScratchDir = "~/.wolm/scratch"

type MessageInfo struct {
	VideoPath      string
	AudioPath      string
	TranscriptPath string
	SpeakerName    string
	Title          string
	Summary        string
	RiskNotes      []RiskNote

	// the spreadsheet row describing this message, and the series it belongs to.
	// resolved up front, before the slow work, so that everything needing an answer
	// from the operator is asked while they are still at the keyboard.
	Message *catalog.CatalogMessage
	Seri    *catalog.CatalogSeri

	// the assembled packet, and why it could not be assembled if it could not. a
	// missing packet is not fatal: the generated title and summary are still useful.
	Packet      *UploadPacket
	PacketError error

	// times
	ExtractTime    util.StopWatch
	TranscribeTime util.StopWatch
	SummaryTime    util.StopWatch
}

// audioCmd represents the command to extract and process audio
var audioCmd = &cobra.Command{
	Use:   "audio",
	Short: "Prepare a message for publication",
	Long: `Process a service message into everything needed to publish it.

Given one or more video files, will do the following for each file:
1. Extract the audio to a temporary .mp3
2. Transcribe the audio to a temporary transcript
3. Send the transcript to Claude for a suggested title and description
4. Delete the temporary files

The audio and transcript are intermediates only - they are not published or
preserved. Pass --keep-intermediates to leave them on disk.`,
	RunE: audio,
}

func init() {
	rootCmd.AddCommand(audioCmd)

	rootCmd.PersistentFlags().String("speaker", "", "Name of the speaker")
	viper.BindPFlag("speaker", rootCmd.PersistentFlags().Lookup("speaker"))

	audioCmd.Flags().Bool("keep-intermediates", false,
		"Do not delete the extracted audio and transcript after processing")
	viper.BindPFlag("keep-intermediates", audioCmd.Flags().Lookup("keep-intermediates"))

	audioCmd.Flags().String("type", "",
		"Message type for the spreadsheet lookup (message, prayer, testimony, ...). Inferred from the file name if not given")
	viper.BindPFlag("type", audioCmd.Flags().Lookup("type"))

	audioCmd.Flags().Int("track", 0,
		"Track number, to pick between several messages sharing a date and type")
	viper.BindPFlag("track", audioCmd.Flags().Lookup("track"))
}

func audio(cmd *cobra.Command, args []string) error {
	initLogging()

	var infos []*MessageInfo

	var videoPaths []string
	if len(args) == 0 {
		// prompt user for video files until there are no more
		for {
			videoPath := getInputVideo("")
			if videoPath == "" {
				if len(videoPaths) == 0 {
					// no videos at all
					fmt.Println("No input files, exiting")
					return nil
				}
				// user is done inputting videos
				break
			}
			videoPaths = append(videoPaths, videoPath)
		}
	} else {
		// validate the video file arguments
		for _, arg := range args {
			videoPaths = append(videoPaths, getInputVideo(arg))
		}
	}

	// read the spreadsheet once for all the videos, rather than once per video
	cat := readCatalogForLookup(cmd.Context())

	// ask everything that needs an answer before starting the slow work, so the
	// operator is not called back to the keyboard twenty minutes into a transcription
	for _, videoPath := range videoPaths {
		infos = append(infos, newMessageInfo(videoPath, cat))
	}

	// process all the video files
	err := processAllVideos(infos)

	// output the results of all processing
	for index, info := range infos {
		printMessageInfo(index, info)
	}

	return err
}

// newMessageInfo creates the record for one video: finds its spreadsheet row, then
// resolves the speaker. Both may ask the operator a question, which is why this runs
// before any extraction or transcription.
func newMessageInfo(videoPath string, cat *catalog.Catalog) *MessageInfo {
	info := MessageInfo{VideoPath: videoPath}

	info.Message, info.Seri, info.PacketError = findMessageForVideo(videoPath, cat)
	if info.PacketError != nil {
		log.Printf("No spreadsheet row for %s: %s", filepath.Base(videoPath), info.PacketError)
	}

	info.SpeakerName = resolveSpeaker(videoPath, info.Message)

	return &info
}

// readCatalogForLookup loads the spreadsheet so messages can be looked up. A failure
// here is not fatal - the transcript, title and summary are still worth producing - so
// it warns and returns nil rather than stopping.
func readCatalogForLookup(ctx context.Context) *catalog.Catalog {
	cat, err := readOnlineContentFromInput(ctx)
	if err != nil {
		fmt.Printf("WARNING: could not read the spreadsheet: %s\n", err)
		fmt.Printf("         continuing without it - no upload packet will be assembled\n")
		return nil
	}
	if err := cat.Initialize(); err != nil {
		fmt.Printf("WARNING: could not initialize the catalog: %s\n", err)
		return nil
	}
	return cat
}

// findMessageForVideo locates the spreadsheet row describing a video, asking the
// operator to choose when the date and type match more than one
func findMessageForVideo(
	videoPath string,
	cat *catalog.Catalog,
) (*catalog.CatalogMessage, *catalog.CatalogSeri, error) {
	if cat == nil {
		return nil, nil, fmt.Errorf("the spreadsheet was not available")
	}

	date, err := getDateFromFileName(videoPath)
	if err != nil {
		return nil, nil, err
	}

	msgType := catalog.NewMessageTypeFromString(viper.GetString("type"))
	if msgType == catalog.UnknownType {
		msgType = getMessageTypeFromFileName(videoPath)
	}

	msg, seri, err := cat.FindMessage(catalog.MessageLookup{
		Date:  date,
		Type:  msgType,
		Track: viper.GetInt("track"),
	})

	// several rows share this date and type. the track number often cannot tell them
	// apart - a message and its Q&A, or several interviews on one date, all carry
	// track 0 - so ask rather than guess.
	var ambiguous *catalog.AmbiguousLookupError
	if errors.As(err, &ambiguous) {
		chosen := promptUserForMessageChoice(ambiguous)
		if chosen == nil {
			return nil, nil, err
		}
		return chosen, cat.FindSeriesForMessage(chosen), nil
	}
	if err != nil {
		return nil, nil, err
	}

	return msg, seri, nil
}

// resolveSpeaker determines who spoke, in order of authority:
//
//  1. an explicit --speaker flag
//  2. an initial in the file name, which is unambiguous when present
//  3. otherwise ask, defaulting to whatever the spreadsheet says
//
// Before the spreadsheet was consulted this early, step 3 defaulted to a guess based
// on whether the file name looked like a prayer. The spreadsheet knows the answer, so
// it supplies the default now and accepting it is a single keystroke.
func resolveSpeaker(videoPath string, msg *catalog.CatalogMessage) string {
	if speaker := viper.GetString("speaker"); speaker != "" {
		return speaker
	}

	if speaker := getSpeakerInitialFromFileName(videoPath); speaker != "" {
		return speaker
	}

	defaultSpeaker := ""
	if msg != nil {
		defaultSpeaker = msg.SpeakerString()
	}
	if defaultSpeaker == "" {
		defaultSpeaker = guessSpeakerFromFileName(videoPath)
	}

	return PromptUserForSpeaker(defaultSpeaker)
}

// processAllVideos processes each video in turn, updating the message information
// records as it goes.
//
// NOTE: this used to be split into two passes ("editing priority") so that the audio
// could be extracted and uploaded for every video first, getting the public S3 URLs
// printed as early as possible while the slow transcription ran afterwards. With no
// uploads there is no early output to hurry, so a single straightforward pass is both
// simpler and equivalent.
func processAllVideos(infos []*MessageInfo) error {
	var errs []error
	for _, info := range infos {
		if err := processOneAudio(info); err != nil {
			errs = append(errs, err)
		}
	}

	// collect the errors to return later
	return errors.Join(errs...)
}

// processOneAudio handles the processing of one message.
// If the audio doesn't exist, will extract the audio from the video.
// If the transcript doesn't exist, will transcribe the audio.
// Will send the transcript to Claude for a title and summary.
// All information will be recorded in the passed in info record.
//
// On success the intermediate audio and transcript are deleted. On failure they are
// left in place so the failed step can be retried without redoing the work before it.
func processOneAudio(info *MessageInfo) error {
	var err error

	if info.VideoPath == "" {
		return fmt.Errorf("no video file was provided to extract audio from. aborting")
	}

	// extract the audio from the video if needed.
	//
	// NOTE: these reuse checks require a non-empty file. A failed run leaves its
	// intermediates behind on purpose so the failed step can be retried, but a tool
	// that reports success while writing nothing leaves a zero-length file, and
	// treating that as "already done" skips the step on every future run.
	info.AudioPath = getAudioPathFromVideoPath(info.VideoPath)
	if !util.IsNonEmptyFile(info.AudioPath) {
		info.ExtractTime = util.NewStopWatch()
		info.AudioPath, err = extractAudioFromVideo(info.VideoPath)
		info.ExtractTime.Stop()
		if err != nil {
			return err
		}
	} else {
		log.Printf("Reusing existing audio %s", info.AudioPath)
	}

	// transcribe the audio file if needed
	info.TranscriptPath = getTranscribePathFromAudioPath(info.AudioPath)
	if !util.IsNonEmptyFile(info.TranscriptPath) {
		info.TranscribeTime = util.NewStopWatch()
		info.TranscriptPath, err = transcribeAudio(info.AudioPath)
		info.TranscribeTime.Stop()
		if err != nil {
			return err
		}
	} else {
		log.Printf("Reusing existing transcript %s", info.TranscriptPath)
	}

	// generate the message summary
	info.SummaryTime = util.NewStopWatch()
	if _, err = generateMessageSummary(info); err != nil {
		return err
	}
	info.SummaryTime.Stop()

	// assemble the upload packet from the row found before processing started. a
	// missing row is not a reason to throw away the summary we just generated, so the
	// error is recorded and shown rather than returned.
	if info.Message != nil {
		info.Packet, info.PacketError = NewUploadPacket(
			info.Message, info.Seri, info.Summary, getThumbnailPath(info.VideoPath))
	}

	// everything succeeded, so the intermediates have served their purpose
	cleanUpIntermediates(info)

	return nil
}

// promptUserForMessageChoice asks which of several equally-matching spreadsheet rows
// describes the video being processed. Returns nil if the operator declines to choose.
func promptUserForMessageChoice(ambiguous *catalog.AmbiguousLookupError) *catalog.CatalogMessage {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("\n%d spreadsheet rows match %s / %s:\n",
		len(ambiguous.Candidates),
		ambiguous.Lookup.Date.String(),
		string(ambiguous.Lookup.Type))
	for index := range ambiguous.Candidates {
		fmt.Printf("  %d. %s\n", index+1, ambiguous.Candidates[index].DescribeForChoice())
	}

	for {
		fmt.Printf("Which one is this video [1-%d, or blank to skip]? ",
			len(ambiguous.Candidates))
		answer, err := reader.ReadString('\n')
		if err != nil && strings.TrimSpace(answer) == "" {
			// stdin closed or unreadable - treat as a skip rather than looping forever
			fmt.Println()
			return nil
		}
		answer = strings.Trim(answer, "\"' \r\n")
		if answer == "" {
			return nil
		}

		choice, err := strconv.Atoi(answer)
		if err != nil || choice < 1 || choice > len(ambiguous.Candidates) {
			fmt.Printf("Please enter a number between 1 and %d.\n", len(ambiguous.Candidates))
			continue
		}

		return &ambiguous.Candidates[choice-1]
	}
}

// getThumbnailPath returns the local thumbnail to upload with the video, if one sits
// beside the video file. Thumbnails go to YouTube with the video; they are not
// uploaded anywhere else.
func getThumbnailPath(videoPath string) string {
	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	for _, ext := range []string{".jpg", ".jpeg", ".png"} {
		if util.IsFile(base + ext) {
			return base + ext
		}
	}
	return ""
}

// cleanUpIntermediates deletes the extracted audio and the transcript. These are
// working files, not content. Suppressed by --keep-intermediates.
func cleanUpIntermediates(info *MessageInfo) {
	if viper.GetBool("keep-intermediates") {
		log.Printf("Keeping intermediates: %s, %s", info.AudioPath, info.TranscriptPath)
		return
	}

	for _, path := range []string{info.AudioPath, info.TranscriptPath} {
		if path == "" || !util.IsFile(path) {
			continue
		}
		if err := os.Remove(path); err != nil {
			// not fatal - the message was processed successfully, we just left a file behind
			log.Printf("WARNING: could not delete intermediate %s: %s", path, err)
			continue
		}
		log.Printf("Deleted intermediate %s", path)
	}
}

func printMessageInfo(index int, info *MessageInfo) {
	fmt.Printf("Message #%d\n", index+1)
	fmt.Printf("Video file: %s\n", filepath.Base(info.VideoPath))
	fmt.Printf("Speaker   : %s\n", info.SpeakerName)

	switch {
	case info.Packet != nil:
		info.Packet.Print()

	case info.PacketError != nil:
		// the message was processed but could not be matched to a spreadsheet row
		fmt.Printf("\nCould not assemble an upload packet:\n  %s\n", info.PacketError)
		fmt.Printf("\nGenerated title and summary (not matched to a spreadsheet row):\n")
		fmt.Printf("╭───────────────────────────────────────────────────────────────────────────────────┄┄\n")
		fmt.Printf("│ Title\n%s\n", info.Title)
		fmt.Printf("│ Summary\n%s\n", info.Summary)
		fmt.Printf("╰───────────────────────────────────────────────────────────────────────────────────┄┄\n")

	default:
		// processing failed before there was anything to assemble. the error itself
		// is reported by the caller, so do not invent a second, emptier one here.
		fmt.Printf("\nNot processed - see the error below.\n")
	}

	printRiskNotes(info)
	log.Printf("Timeline: Extract = %s, Transcribe = %s, Summarize = %s\n",
		info.ExtractTime.Elapsed(), info.TranscribeTime.Elapsed(), info.SummaryTime.Elapsed())
	fmt.Println()
}

// getScratchDir returns the directory for intermediate files
func getScratchDir() string {
	dir := viper.GetString("scratch-dir")
	if dir == "" {
		dir = defaultScratchDir
	}
	return util.NormalizePath(dir)
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

// getAudioPathFromVideoPath returns the scratch path of the audio extracted from the
// given video
func getAudioPathFromVideoPath(videoPath string) string {
	name := filepath.Base(videoPath)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return filepath.Join(getScratchDir(), name+".mp3")
}

// getTranscribePathFromAudioPath returns the scratch path of the transcript generated
// from the given audio
func getTranscribePathFromAudioPath(audioPath string) string {
	name := filepath.Base(audioPath)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return filepath.Join(getScratchDir(), name+".text")
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

func PromptUserForSpeaker(defaultSpeaker string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Who is the speaker [Default:%s/Vern(v)/Mary(m)/Other]?\n", defaultSpeaker)
	name, _ := reader.ReadString('\n')
	name = strings.Trim(name, "\"' \r\n")

	if name == "" {
		name = defaultSpeaker
	}
	switch strings.ToUpper(name) {
	case "V", "VERN":
		return "Pastor Vern Peltz"
	case "M", "MARY":
		return "Pastor Mary Peltz"
	default:
		return name
	}
}
