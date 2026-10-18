package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/util"
	"github.com/WordOfLifeMN/online/util/box"
	"github.com/WordOfLifeMN/online/ytpkg"
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

	// how the generated summary still breaks the rules for new descriptions after being
	// sent back once to be corrected; shown so it can be edited before it is pasted
	SummaryProblems []string

	// the spreadsheet row describing this message, and the series it belongs to.
	// resolved up front, before the slow work, so that everything needing an answer
	// from the operator is asked while they are still at the keyboard.
	Message *catalog.CatalogMessage
	Seri    *catalog.CatalogSeri

	// the assembled package, and why it could not be assembled if it could not. a
	// missing package is not fatal: the generated title and summary are still useful.
	Packet      *ytpkg.Message
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
2. Transcribe the audio, keeping the transcript beside the video as a .txt
3. Send the transcript to Claude for a suggested title and description

A video that already has a .txt transcript beside it skips steps 1 and 2. To
transcribe it again, delete the .txt.

Transcripts are kept but never published. The .mp3 is an intermediate: it is kept
in the scratch directory for 24 hours so that re-transcribing a message reuses it
instead of extracting it again, then swept on the next run. Pass
--keep-intermediates to disable the sweep.`,
	RunE: audio,
}

func init() {
	rootCmd.AddCommand(audioCmd)

	rootCmd.PersistentFlags().String("speaker", "", "Name of the speaker")
	viper.BindPFlag("speaker", rootCmd.PersistentFlags().Lookup("speaker"))

	audioCmd.Flags().Bool("keep-intermediates", false,
		"Do not sweep old audio and transcript files from the scratch directory")
	viper.BindPFlag("keep-intermediates", audioCmd.Flags().Lookup("keep-intermediates"))

	audioCmd.Flags().Int("track", 0,
		"Track number, to pick between several messages sharing a date without being asked")
	viper.BindPFlag("track", audioCmd.Flags().Lookup("track"))
}

func audio(cmd *cobra.Command, args []string) error {
	initLogging()

	// housekeeping runs however this exits, including an early return or a failure
	defer cleanUpScratchDir()

	// read the spreadsheet once up front, before any questions, so that the questions
	// about each video can be answered from it
	cat := readCatalogForLookup(cmd.Context())

	printAudioBanner(os.Stdout)

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
	err := processAllVideos(infos, cat)

	// output the results of all processing
	for index, info := range infos {
		printMessageInfo(os.Stdout, index, info)
	}

	// then offer to publish each message that has a package. a message that is already
	// on YouTube is synced instead. this comes last, so that everything above is on
	// screen whatever happens here.
	publishErr := publishAll(cmd.Context(), cat, infos)

	return errors.Join(err, publishErr)
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

// printAudioBanner explains the input loop before the first prompt. Each paragraph is
// one line of text and the box wraps it, so the banner fits whatever width the box is.
func printAudioBanner(w io.Writer) {
	fmt.Fprintln(w)

	b := box.New(w)
	b.Header("Prepare messages for YouTube")
	b.Line("Drag an edited video into this window and press Enter. You will be asked " +
		"about that video before being asked for the next one.")
	b.Blank()
	b.Line("Press Enter on an empty line when there are no more videos.")
	b.Footer()

	fmt.Fprintln(w)
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

	info.SpeakerName = resolveSpeaker(info.Message)

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

	msg, seri, err := cat.FindMessage(catalog.MessageLookup{
		Date:  date,
		Track: viper.GetInt("track"),
	})

	// several rows share this date. the track number often cannot tell them apart - a
	// message and its Q&A, or several interviews on one date, all carry track 0 - so
	// ask rather than guess.
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
//  3. otherwise ask
//
// Asking is skipped entirely when the row names a speaker. Confirming it was worth
// doing when the prompt predated the lookup and the only alternative was a guess, but a
// question whose default is right every time teaches the operator to hit Enter without
// reading it.
//
// Taking the row also gets the name right where it is published. The summary must name
// the speaker in full, honorific included, and the row's name is the one the sheet and
// the website use.
//
// The file name is not consulted. Its speaker initial used to sit between the row and
// the prompt, which only mattered when the row was silent, and which could contradict
// the row when it was not.
func resolveSpeaker(msg *catalog.CatalogMessage) string {
	if speaker := viper.GetString("speaker"); speaker != "" {
		return speaker
	}

	if msg != nil {
		if speaker := msg.SpeakerString(); speaker != "" {
			return speaker
		}
	}

	return PromptUserForSpeaker()
}

// processAllVideos processes each video in turn, updating the message information
// records as it goes.
//
// NOTE: this used to be split into two passes ("editing priority") so that the audio
// could be extracted and uploaded for every video first, getting the public S3 URLs
// printed as early as possible while the slow transcription ran afterwards. With no
// uploads there is no early output to hurry, so a single straightforward pass is both
// simpler and equivalent.
func processAllVideos(infos []*MessageInfo, cat *catalog.Catalog) error {
	// a nil *Catalog inside the interface would not compare equal to nil, so only a
	// catalog that was actually read is handed on
	var finder ytpkg.SeriesFinder
	if cat != nil {
		finder = cat
	}

	var errs []error
	for _, info := range infos {
		if err := processOneAudio(info, finder); err != nil {
			errs = append(errs, err)
		}
	}

	// collect the errors to return later
	return errors.Join(errs...)
}

// processOneAudio handles the processing of one message.
// If a transcript is kept beside the video, it is used and nothing is extracted or
// transcribed. Otherwise the audio is extracted (unless a scratch copy remains), then
// transcribed, and the transcript is kept beside the video.
// Will send the transcript to Claude for a title and summary.
// All information will be recorded in the passed in info record. cat finds the series
// the message's playlists and booklets come from, and may be nil.
//
// The extracted audio stays in the scratch directory until it is swept, so a failed
// step can be retried without extracting it again.
func processOneAudio(info *MessageInfo, cat ytpkg.SeriesFinder) error {
	var err error

	if info.VideoPath == "" {
		return fmt.Errorf("no video file was provided to extract audio from. aborting")
	}

	if path := findExistingTranscript(info.VideoPath); path != "" {
		// announced rather than logged: deleting this file is how a transcript is
		// regenerated, so the operator needs to see that it was used
		info.TranscriptPath = path
		fmt.Printf("Using existing transcript: %s\n", filepath.Base(path))
	} else {
		if err := extractAndTranscribe(info); err != nil {
			return err
		}
	}

	// generate the message summary
	info.SummaryTime = util.NewStopWatch()
	_, err = generateMessageSummary(info)
	info.SummaryTime.Stop()
	if err != nil {
		return err
	}

	// assemble the upload packet from the row found before processing started. a
	// missing row is not a reason to throw away the summary we just generated, so the
	// error is recorded and shown rather than returned.
	if info.Message != nil {
		// the row's type, not the one inferred from the file name, for the same reason
		// the speaker comes from the row: it is the record of what this actually is
		thumbnail := getThumbnailPath(info.VideoPath, info.Message.Type)
		info.Packet, info.PacketError = ytpkg.NewMessageFromCatalog(
			info.Message, cat, info.Title, info.Summary, thumbnail)
	}

	return nil
}

// extractAndTranscribe makes a new transcript for a message and keeps it beside the
// video, recording the paths and times in info.
func extractAndTranscribe(info *MessageInfo) error {
	var err error

	// extract the audio from the video if needed.
	//
	// NOTE: this reuse check requires a non-empty file. A failed run leaves its
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

	// a transcript left in scratch is never reused - only the one beside the video is,
	// so that deleting it regenerates it. Delete the stale copy without asking:
	// otherwise transcribeAudio would ask whether to overwrite it, in the middle of the
	// slow work after the operator has walked away, and "no" would reuse it after all.
	if err := deleteExistingFile(getTranscribePathFromAudioPath(info.AudioPath), false); err != nil {
		return err
	}

	info.TranscribeTime = util.NewStopWatch()
	info.TranscriptPath, err = transcribeAudio(info.AudioPath)
	info.TranscribeTime.Stop()
	if err != nil {
		return err
	}

	info.TranscriptPath = preserveTranscript(info.TranscriptPath, info.VideoPath)
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
	return rankCandidatesByTitle(getTitleFromFileName(videoPath), candidates)
}

// rankCandidatesByTitle orders candidates by how closely each spreadsheet name
// resembles a title, closest first, and reports whether the winner is a clear one. An
// empty title offers no opinion: the candidates keep their order and there is no
// winner.
func rankCandidatesByTitle(
	title string,
	candidates []catalog.CatalogMessage,
) ([]catalog.CatalogMessage, bool) {
	target := util.NormalizeForMatch(title)
	if target == "" {
		// nothing to compare against, so there is no opinion to offer
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
			distance: levenshtein.ComputeDistance(target, util.NormalizeForMatch(candidates[index].Name)),
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
	return promptUserForChoice(
		ambiguous.Candidates,
		getTitleFromFileName(videoPath),
		fmt.Sprintf("%d spreadsheet rows are dated %s:",
			len(ambiguous.Candidates), ambiguous.Lookup.Date.String()))
}

// promptUserForChoice asks which of several spreadsheet rows describes a video, under
// a heading that says why there are several. The rows are ranked against title, and
// the best match is marked and offered as the default when it is a clear one. Returns
// nil if the operator declines to choose.
func promptUserForChoice(
	candidates []catalog.CatalogMessage,
	title string,
	heading string,
) *catalog.CatalogMessage {
	reader := getStdin()
	candidates, hasDefault := rankCandidatesByTitle(title, candidates)

	fmt.Printf("\n%s\n", heading)
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

// thumbnailExtensions are the image types recognised as a thumbnail
var thumbnailExtensions = []string{".jpg", ".jpeg", ".png"}

// getThumbnailPath returns the local thumbnail to upload with the video. Thumbnails go
// to YouTube with the video; they are not uploaded anywhere else.
//
// The search runs from most specific to least, so artwork made for this message always
// beats a generic fallback:
//
//  1. an image beside the video sharing its name - artwork for this one message
//  2. any other image in that directory with "thumb" in the name - the series
//     thumbnail, shared by every message in the series folder
//  3. a generic fallback from 'thumb-dir', named for the message type
//
// Returns "" when none of those exist, which the packet renders by omitting the line
// rather than by claiming a thumbnail that is not there.
func getThumbnailPath(videoPath string, msgType catalog.MessageType) string {
	if path := thumbnailNamedForVideo(videoPath); path != "" {
		return path
	}
	if path := thumbnailInVideoDir(videoPath); path != "" {
		return path
	}
	return fallbackThumbnail(msgType)
}

// thumbnailNamedForVideo finds an image sharing the video's name
func thumbnailNamedForVideo(videoPath string) string {
	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	for _, ext := range thumbnailExtensions {
		if util.IsFile(base + ext) {
			return base + ext
		}
	}
	return ""
}

// thumbnailInVideoDir finds the series thumbnail: an image in the video's directory
// with "thumb" in its name, which covers every message in that series folder. One
// image is dropped in once and each message in the series picks it up, so a series
// does not need artwork produced per message.
//
// It ranks below a name match because a message with its own artwork should use it;
// the series thumbnail is the fallback for the rest of the folder.
//
// Sorted, so a folder holding more than one such image picks the same file every run
// instead of depending on the order the file system happens to return.
func thumbnailInVideoDir(videoPath string) string {
	entries, err := os.ReadDir(filepath.Dir(videoPath))
	if err != nil {
		return ""
	}

	var found []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.Contains(strings.ToLower(name), "thumb") {
			continue
		}
		if slices.Contains(thumbnailExtensions, strings.ToLower(filepath.Ext(name))) {
			found = append(found, name)
		}
	}
	if len(found) == 0 {
		return ""
	}

	slices.Sort(found)
	return filepath.Join(filepath.Dir(videoPath), found[0])
}

// fallbackThumbnail returns the generic artwork for a message type from 'thumb-dir'.
//
// A type with no artwork of its own falls back to the message artwork rather than to
// nothing, since every type that is not a prayer is a message as far as the cover image
// is concerned. Returns "" when 'thumb-dir' is not configured - this step is the only
// one that consults it, so an unset key simply means there is no generic fallback.
func fallbackThumbnail(msgType catalog.MessageType) string {
	dir := viper.GetString("thumb-dir")
	if dir == "" {
		return ""
	}
	dir = util.NormalizePath(dir)

	for _, t := range []catalog.MessageType{msgType, catalog.Message} {
		path := filepath.Join(dir, thumbnailFileName(t))
		if util.IsFile(path) {
			return path
		}
	}

	return ""
}

// thumbnailFileName renders the generic artwork name for a message type, matching how
// the files are named on disk: "WOL Thumbnail - Message.jpg".
func thumbnailFileName(msgType catalog.MessageType) string {
	words := strings.Split(string(msgType), "-")
	for index, word := range words {
		if word != "" {
			words[index] = strings.ToUpper(word[:1]) + word[1:]
		}
	}

	return fmt.Sprintf("WOL Thumbnail - %s.jpg", strings.Join(words, "-"))
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

// printMessageInfo writes everything known about one message as a single closed box,
// followed by a blank line
func printMessageInfo(w io.Writer, index int, info *MessageInfo) {
	b := box.New(w)

	b.Header(fmt.Sprintf("Message #%d", index+1))
	b.Labeled("Video file: ", filepath.Base(info.VideoPath))
	b.Labeled("Speaker   : ", info.SpeakerName)

	// the order matters. PacketError is set before processing whenever there is no
	// spreadsheet row, so testing it ahead of the generated title would report a
	// message that also failed processing as unmatched, with nothing under it.
	switch {
	case info.Packet != nil:
		info.Packet.Print(b)

	case info.Title == "" && info.Summary == "":
		// processing failed before there was anything to assemble. the error itself
		// is reported by the caller, so do not invent a second, emptier one here.
		b.Section("Not processed")
		b.Line("Processing failed - see the error below.")
		if info.PacketError != nil {
			b.Labeled("Reason: ", info.PacketError.Error())
		}

	default:
		// the message was processed but could not be matched to a spreadsheet row
		b.Section("No upload packet")
		if info.PacketError != nil {
			b.Labeled("Reason: ", info.PacketError.Error())
		}
		b.Section("Titles")
		b.Line("Title")
		b.Copy(info.Title)
		b.Section("Description")
		b.Copy(info.Summary)
	}

	printSummaryProblems(b, info)
	printRiskNotes(b, info)
	printTimeline(b, info)
	b.Footer()
	fmt.Fprintln(w)
}

// printTimeline writes how long each step took, with --verbose only. Durations are
// right aligned so the column reads down, and rounded to the second because nobody
// waiting on a transcription cares about the milliseconds.
func printTimeline(b *box.Box, info *MessageInfo) {
	if !viper.GetBool("verbose") {
		return
	}

	summarized := info.Title != "" || info.Summary != ""

	b.Section("Timeline")
	var total time.Duration
	for _, step := range []struct {
		name  string
		watch util.StopWatch
	}{
		{"Extract   : ", info.ExtractTime},
		{"Transcribe: ", info.TranscribeTime},
		{"Summarize : ", info.SummaryTime},
	} {
		b.Labeled(step.name, fmt.Sprintf("%6s", formatStepDuration(step.watch, summarized)))
		if !step.watch.StartTime.IsZero() {
			total += step.watch.Elapsed()
		}
	}
	b.Labeled("Total     : ", fmt.Sprintf("%6s", formatDuration(total)))
}

// formatStepDuration renders how long one step took. A step that never started either
// had its earlier output reused, when the message went on to be summarized, or was
// never reached because processing stopped before it.
func formatStepDuration(w util.StopWatch, summarized bool) string {
	if w.StartTime.IsZero() {
		if summarized {
			return "reused"
		}
		return "-"
	}
	return formatDuration(w.Elapsed())
}

// formatDuration renders a duration to the second: "24s", "2m08s", "1h02m03s"
func formatDuration(d time.Duration) string {
	seconds := int(d.Round(time.Second) / time.Second)
	hours, minutes := seconds/3600, seconds/60%60
	seconds %= 60

	switch {
	case hours > 0:
		return fmt.Sprintf("%dh%02dm%02ds", hours, minutes, seconds)
	case minutes > 0:
		return fmt.Sprintf("%dm%02ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
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

// cleanDroppedPath turns whatever a terminal inserts for a dragged file back into the
// path:
//
//	"C:\Videos\Pay Attention.mp4"      cmd.exe, or a quoted path typed by hand
//	'C:\Videos\Pay Attention.mp4'      a single-quoted path
//	& 'C:\Videos\God''s Word.mp4'      PowerShell, VSCode's terminal included
//
// PowerShell offers the file as a command to run, hence the "& ", and doubles any
// apostrophe inside its single quotes, which is undone here.
func cleanDroppedPath(s string) string {
	s = strings.TrimSpace(s)

	if rest, ok := strings.CutPrefix(s, "&"); ok {
		s = strings.TrimSpace(rest)
	}

	if len(s) >= 2 && strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'") {
		return strings.ReplaceAll(s[1:len(s)-1], "''", "'")
	}
	return strings.Trim(s, "\"' ")
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
	for filePath := cleanDroppedPath(path); ; filePath = "" {
		if filePath == "" {
			fmt.Println("Enter path or drag the file to use:")
			filePath, err = reader.ReadString('\n')
			filePath = cleanDroppedPath(filePath)
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

// PromptUserForSpeaker asks who spoke. There is no default: it is only reached when
// nothing - not the flag, not the spreadsheet row - knows the answer, and offering a
// guess as the Enter key is how the wrong name gets accepted without being read.
//
// The v/m shortcuts are kept. They are the operator typing an abbreviation at a
// prompt, not the program inferring a person from a file name.
func PromptUserForSpeaker() string {
	reader := getStdin()
	fmt.Printf("Who is the speaker [Vern(v)/Mary(m)/Other]?\n")
	name, _ := reader.ReadString('\n')
	name = strings.Trim(name, "\"' \r\n")

	switch strings.ToUpper(name) {
	case "V", "VERN":
		return "Pastor Vern Peltz"
	case "M", "MARY":
		return "Pastor Mary Peltz"
	default:
		return name
	}
}
