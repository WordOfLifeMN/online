package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/util"
	"github.com/agnivade/levenshtein"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// defaultScratchDir is where intermediate audio and transcript files are written when
// no 'scratch-dir' is configured. They are working files rather than content, so they
// deliberately do not live beside the source video, and they are swept once they are
// older than scratchRetention.
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

The audio and transcript are intermediates - they are never published. They are
kept in the scratch directory for 24 hours so that re-running a message reuses
them instead of extracting and transcribing it again, then swept on the next run.
Pass --keep-intermediates to disable the sweep.`,
	RunE: audio,
}

func init() {
	rootCmd.AddCommand(audioCmd)

	rootCmd.PersistentFlags().String("speaker", "", "Name of the speaker")
	viper.BindPFlag("speaker", rootCmd.PersistentFlags().Lookup("speaker"))

	audioCmd.Flags().Bool("keep-intermediates", false,
		"Do not sweep old audio and transcript files from the scratch directory")
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

	// housekeeping runs however this exits, including an early return or a failure
	defer cleanUpScratchDir()

	// read the spreadsheet once up front, before any questions, so that the questions
	// about each video can be answered from it
	cat := readCatalogForLookup(cmd.Context())

	printAudioBanner()

	// Gather the videos and ask about each one as it is entered. Everything that needs
	// an answer is asked before the slow work starts, so the operator is not called
	// back to the keyboard twenty minutes into a transcription - but the questions
	// about a video must immediately follow that video, or there is no way to tell
	// which answer belongs to which file.
	var infos []*MessageInfo

	if len(args) == 0 {
		for {
			videoPath := getInputVideo("")
			if videoPath == "" {
				if len(infos) == 0 {
					// no videos at all
					fmt.Println("No input files, exiting")
					return nil
				}
				// user is done entering videos
				break
			}
			infos = append(infos, newMessageInfo(videoPath, cat))
		}
	} else {
		// validate the video file arguments
		for _, arg := range args {
			infos = append(infos, newMessageInfo(getInputVideo(arg), cat))
		}
	}

	// process all the video files
	err := processAllVideos(infos)

	// output the results of all processing
	for index, info := range infos {
		printMessageInfo(index, info)
	}

	return err
}

// stdinReader is the one buffered reader over standard input.
//
// Every prompt must share it. A bufio.Reader reads ahead in blocks, so a second
// reader over the same file descriptor finds that the first one has already consumed
// the bytes it wanted. With a terminal this is usually invisible, because input
// arrives a line at a time; with redirected input the first prompt swallows the whole
// stream and every later prompt sees EOF.
var stdinReader *bufio.Reader

// getStdin returns the shared reader, creating it on first use. It is created lazily
// rather than at startup so that a test can replace os.Stdin and call resetStdin.
func getStdin() *bufio.Reader {
	if stdinReader == nil {
		stdinReader = bufio.NewReader(os.Stdin)
	}
	return stdinReader
}

// resetStdin drops the shared reader so the next prompt picks up the current
// os.Stdin. For tests.
func resetStdin() {
	stdinReader = nil
}

// printAudioBanner explains the input loop before the first prompt
func printAudioBanner() {
	fmt.Printf("\n")
	fmt.Printf("╭───────────────────────────────────────────────────────────────────────────────────┄┄\n")
	fmt.Printf("│ Prepare messages for YouTube\n")
	fmt.Printf("│\n")
	fmt.Printf("│ Drag an edited video into this window and press Enter. You will be asked\n")
	fmt.Printf("│ about that video before being asked for the next one.\n")
	fmt.Printf("│\n")
	fmt.Printf("│ Press Enter on an empty line when there are no more videos.\n")
	fmt.Printf("╰───────────────────────────────────────────────────────────────────────────────────┄┄\n")
	fmt.Printf("\n")
}

// newMessageInfo creates the record for one video: finds its spreadsheet row, then
// resolves the speaker. Both may ask the operator a question, which is why this runs
// before any extraction or transcription - and why it runs immediately after the
// video is entered, so the questions stay attached to the file they are about.
func newMessageInfo(videoPath string, cat *catalog.Catalog) *MessageInfo {
	info := MessageInfo{VideoPath: videoPath}

	// name the video the questions are about, so a run covering several services
	// cannot leave the operator guessing which answer applies to which file
	fmt.Printf("\n── %s\n", filepath.Base(videoPath))

	info.Message, info.Seri, info.PacketError = findMessageForVideo(videoPath, cat)
	if info.PacketError != nil {
		log.Printf("No spreadsheet row for %s: %s", filepath.Base(videoPath), info.PacketError)
		fmt.Printf("   No spreadsheet row: %s\n", info.PacketError)
	} else {
		// show the row the answers are coming from, so the speaker default below is
		// not an unexplained suggestion
		fmt.Printf("   Spreadsheet: %s\n", info.Message.DescribeForChoice())
	}

	info.SpeakerName = resolveSpeaker(videoPath, info.Message)

	return &info
}

// readCatalogForLookup loads the spreadsheet so messages can be looked up. A failure
// here is not fatal - the transcript, title and summary are still worth producing - so
// it warns and returns nil rather than stopping.
func readCatalogForLookup(ctx context.Context) *catalog.Catalog {
	// this takes a couple of seconds and is otherwise silent without --verbose
	fmt.Printf("Reading the spreadsheet...\n")

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

	// an operator-supplied --type is an assertion and is matched exactly. one read off
	// the file name is only a hint, and the lookup trusts it accordingly.
	msgType := catalog.NewMessageTypeFromString(viper.GetString("type"))
	typeIsHint := msgType == catalog.UnknownType
	if typeIsHint {
		msgType = getMessageTypeFromFileName(videoPath)
	}

	msg, seri, err := cat.FindMessage(catalog.MessageLookup{
		Date:       date,
		Type:       msgType,
		Track:      viper.GetInt("track"),
		TypeIsHint: typeIsHint,
	})

	// several rows share this date and type. the track number often cannot tell them
	// apart - a message and its Q&A, or several interviews on one date, all carry
	// track 0 - so ask rather than guess.
	var ambiguous *catalog.AmbiguousLookupError
	if errors.As(err, &ambiguous) {
		chosen := promptUserForMessageChoice(ambiguous, videoPath)
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
//  1. an explicit --speaker flag, which is the operator overriding everything
//  2. the spreadsheet row, which is the record of who actually preached
//  3. an initial in the file name, when there is no row to consult
//  4. otherwise ask, defaulting to a guess from the file name
//
// The spreadsheet outranks the file name initial, and asking is skipped entirely when
// the row names a speaker. Confirming it was worth doing when the prompt predated the
// lookup and the only alternative was a guess, but a question whose default is right
// every time teaches the operator to hit Enter without reading it.
//
// Taking the row also keeps the summary and the title consistent. The title is built
// from the row's speakers, so resolving a different name here - which a stale or wrong
// initial in the file name could do - would have the summary introduce one person and
// the title credit another.
func resolveSpeaker(videoPath string, msg *catalog.CatalogMessage) string {
	if speaker := viper.GetString("speaker"); speaker != "" {
		return speaker
	}

	if msg != nil {
		if speaker := msg.SpeakerString(); speaker != "" {
			return speaker
		}
	}

	if speaker := getSpeakerInitialFromFileName(videoPath); speaker != "" {
		return speaker
	}

	return PromptUserForSpeaker(guessSpeakerFromFileName(videoPath))
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

	return nil
}

// rankCandidatesByFileName orders candidates by how closely each spreadsheet name
// resembles the title in the video file name, closest first, and reports whether the
// winner is a clear one.
//
// The title in the file name is the one thing that distinguishes two videos recorded
// on the same date, and until now it was the one thing the lookup ignored. It is still
// not trusted to decide alone: the two sides disagree about leading articles,
// abbreviations and punctuation, so edit distance orders the list and preselects a
// default while the operator still sees every candidate.
//
// "Clear" means strictly better than the runner up. On a tie there is no default and
// nothing is marked, because a best-match arrow that is only a coin toss is worse than
// no arrow - it invites a confirming keystroke it has not earned.
func rankCandidatesByFileName(
	videoPath string,
	candidates []catalog.CatalogMessage,
) ([]catalog.CatalogMessage, bool) {
	target := normalizeForMatch(getTitleFromFileName(videoPath))
	if target == "" {
		// nothing but markers in the file name, so there is no opinion to offer
		return slices.Clone(candidates), false
	}

	// scored rather than a map keyed by name, because two rows on one date can share a
	// name - a message and its second service, say
	type scored struct {
		msg      catalog.CatalogMessage
		distance int
	}

	ranked := make([]scored, len(candidates))
	for index := range candidates {
		ranked[index] = scored{
			msg:      candidates[index],
			distance: levenshtein.ComputeDistance(target, normalizeForMatch(candidates[index].Name)),
		}
	}

	// stable, so equally distant candidates keep their spreadsheet order
	slices.SortStableFunc(ranked, func(a, b scored) int {
		return a.distance - b.distance
	})

	out := make([]catalog.CatalogMessage, len(ranked))
	for index := range ranked {
		out[index] = ranked[index].msg
	}

	return out, len(ranked) > 1 && ranked[0].distance < ranked[1].distance
}

// promptUserForMessageChoice asks which of several equally-matching spreadsheet rows
// describes the video being processed. Returns nil if the operator declines to choose.
func promptUserForMessageChoice(
	ambiguous *catalog.AmbiguousLookupError,
	videoPath string,
) *catalog.CatalogMessage {
	reader := getStdin()
	candidates, hasDefault := rankCandidatesByFileName(videoPath, ambiguous.Candidates)

	fmt.Printf("\n%d spreadsheet rows match %s / %s:\n",
		len(candidates),
		ambiguous.Lookup.Date.String(),
		ambiguous.Lookup.DescribeType())
	for index := range candidates {
		fmt.Printf("  %d. %s", index+1, candidates[index].DescribeForChoice())
		if index == 0 && hasDefault {
			fmt.Printf("   <- best match")
		}
		fmt.Println()
	}

	// the skip key moves off blank when blank has been given a meaning, so that
	// accepting the best match and declining to choose stay distinct answers
	prompt := fmt.Sprintf("Which one is this video [1-%d, or blank to skip]? ", len(candidates))
	if hasDefault {
		prompt = fmt.Sprintf("Which one is this video [1-%d, Enter for 1, s to skip]? ",
			len(candidates))
	}

	for {
		fmt.Print(prompt)
		answer, err := reader.ReadString('\n')
		if err != nil && strings.TrimSpace(answer) == "" {
			// stdin closed or unreadable - treat as a skip rather than looping forever
			fmt.Println()
			return nil
		}
		answer = strings.Trim(answer, "\"' \r\n")

		if answer == "" {
			if hasDefault {
				return &candidates[0]
			}
			return nil
		}
		if hasDefault && strings.EqualFold(answer, "s") {
			return nil
		}

		choice, err := strconv.Atoi(answer)
		if err != nil || choice < 1 || choice > len(candidates) {
			fmt.Printf("Please enter a number between 1 and %d.\n", len(candidates))
			continue
		}

		return &candidates[choice-1]
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

// scratchRetention is how long an extracted audio file or transcript is kept before
// being swept. Long enough that re-running a message later the same day reuses the
// work rather than redoing it, short enough that the directory does not grow without
// limit - the audio for one service is around 75MB.
const scratchRetention = 24 * time.Hour

// scratchExtensions are the files this application creates in the scratch directory.
// Only these are ever deleted, so anything else put there by hand is left alone.
var scratchExtensions = []string{".mp3", ".text"}

// cleanUpScratchDir sweeps intermediates older than the retention period.
//
// It runs when the command exits rather than after each message, and it works on age
// rather than on what this run happened to produce. That means a failed or
// interrupted run leaves its audio and transcript behind, and re-running the same
// message picks them up instead of extracting and transcribing again - which for an
// eighty minute service is several minutes saved. Files from this run are minutes
// old, so they are never swept by the run that created them.
//
// Suppressed entirely by --keep-intermediates.
func cleanUpScratchDir() {
	if viper.GetBool("keep-intermediates") {
		log.Printf("Keeping intermediates: --keep-intermediates was given")
		return
	}

	dir := getScratchDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		// nothing to sweep if the directory was never created
		log.Printf("Not sweeping %s: %s", dir, err)
		return
	}

	cutoff := time.Now().Add(-scratchRetention)
	var deleted int
	var freed int64

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !slices.Contains(scratchExtensions, ext) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			// recent enough to be worth reusing, including everything this run made
			log.Printf("Keeping recent intermediate %s", entry.Name())
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if err := os.Remove(path); err != nil {
			// not fatal, we just leave a file behind
			log.Printf("WARNING: could not delete %s: %s", path, err)
			continue
		}
		log.Printf("Swept %s", entry.Name())
		deleted++
		freed += info.Size()
	}

	if deleted > 0 {
		fmt.Printf("Cleaned %d intermediate file(s) over %s old from %s (%.0f MB)\n",
			deleted, scratchRetention, dir, float64(freed)/(1024*1024))
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

	reader := getStdin()

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
		reader := getStdin()
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
	reader := getStdin()
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
