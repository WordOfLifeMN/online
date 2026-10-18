package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/WordOfLifeMN/online/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// defaultWhisperModel is the transcription model used when none is configured.
//
// Measured on a real 80 minute service (CUDA, 2026-10-03), word error rate taken
// against large-v3 over content words only:
//
//	model             time    speed   contentWER
//	tiny.en          2m28s      39x   EMPTY OUTPUT - reports success, writes nothing
//	base.en          2m04s      39x    8.5%
//	small            3m06s      26x    7.0%
//	small.en         4m04s      20x    6.7%
//	distil-large-v3  6m27s      13x    6.9%
//	medium           6m40s      12x    6.1%
//	large-v3-turbo   8m09s      10x    5.6%
//	large-v3        17m59s       4x    reference
//
// No model comes within 5% of large-v3, but that threshold turned out to be the wrong
// test. Feeding each transcript to the summariser produced the same title and the same
// three sentences - same speaker, same seven points, same closing image - all the way
// down to base.en. The differences the error rate is counting are filler words,
// contractions and the occasional mishearing, none of which survive summarisation.
//
// small is the default because it is bundled (no download), takes about three minutes
// for a full service, and leaves quality headroom for a guest speaker or poorer audio
// than this sample. base.en would save roughly a minute at the bottom of the measured
// quality range; large-v3-turbo is the best accuracy per minute if fidelity ever
// matters more than waiting.
//
// tiny.en must not be used. It wrote an empty transcript while reporting success on
// both a 67 second excerpt and the full 80 minute service.
const defaultWhisperModel = "small"

// whisperExe is the faster-whisper binary. Overridable with the 'whisper-exe' config key.
const defaultWhisperExe = "C:/Users/WordofLifeMNMedia/bin/Faster-Whisper-XXL_r245.4_windows/Faster-Whisper-XXL/faster-whisper-xxl.exe"

// audioTranscribeCmd represents the command to transcribe audio to text
var audioTranscribeCmd = &cobra.Command{
	Use:   "transcribe audio-file",
	Short: "Transcribe the audio track to text",
	Long: `Takes an already extracted audio track and transcribes it to English text.

The input file must be .mp3 and the transcript is written to the scratch directory,
where it is swept after 24 hours. An .mp3 does not know where its video is, so this
command does not keep the transcript beside the video - run 'audio' on the video for
that.

The model can be set with the 'whisper-model' configuration key (default "` + defaultWhisperModel + `").

Requires 'faster-whisper' be installed and accessible.`,
	RunE: audioTranscribe,
}

func init() {
	audioCmd.AddCommand(audioTranscribeCmd)

	audioTranscribeCmd.Args = cobra.MaximumNArgs(1)
}

func audioTranscribe(cmd *cobra.Command, args []string) error {
	initLogging()

	var audioPath string

	// get the input audio file
	if len(args) == 1 {
		audioPath = args[0]
	}
	audioPath = getInputAudio(audioPath)
	if audioPath == "" {
		fmt.Printf("Aborting")
		return nil
	}

	xscriptPath, err := transcribeAudio(audioPath)
	if err != nil {
		return err
	}

	fmt.Printf("Transcribed to %s\n", xscriptPath)
	return nil
}

// getWhisperModel returns the transcription model to use
func getWhisperModel() string {
	if model := viper.GetString("whisper-model"); model != "" {
		return model
	}
	return defaultWhisperModel
}

// getWhisperExe returns the path to the faster-whisper executable
func getWhisperExe() string {
	if exe := viper.GetString("whisper-exe"); exe != "" {
		return exe
	}
	return defaultWhisperExe
}

// transcribeAudio uses faster-whisper to transcribe the audio and returns the path to
// the resulting text file in the scratch directory, with a header line recording the
// model at the top. Only text output is produced - the .vtt, .srt, .tsv and .json
// formats existed to publish transcripts online, which we no longer do.
//
// Keeping the transcript beside its video is the caller's job (see preserveTranscript):
// this works from an .mp3, which does not know where its video is.
func transcribeAudio(audioPath string) (string, error) {
	xscriptPath := getTranscribePathFromAudioPath(audioPath)

	if err := deleteExistingFile(xscriptPath, true); err != nil {
		if strings.HasSuffix(err.Error(), "exists") {
			// user doesn't want to overwrite. assume the existing transcript is valid
			return xscriptPath, nil
		}
		return "", err
	}

	// make sure the scratch directory exists
	if err := os.MkdirAll(filepath.Dir(xscriptPath), os.FileMode(0777)); err != nil {
		return "", fmt.Errorf("cannot create scratch directory %s: %w", filepath.Dir(xscriptPath), err)
	}

	// output status
	fmt.Printf("Transcribing: %s\n", filepath.Base(audioPath))
	fmt.Printf("       model: %s\n", getWhisperModel())
	fmt.Printf("          to: %s\n", filepath.Base(xscriptPath))

	cmd := exec.Command(
		getWhisperExe(),
		"--output_format", "text",
		"--output_dir", filepath.Dir(xscriptPath),
		"--model", getWhisperModel(),
		"--language", "en",
		audioPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Print(cmd.String())
	if err := cmd.Run(); err != nil {
		// faster-whisper exits non-zero even on success, so this is not fatal on its
		// own - the output file is the real test. Log it for diagnosis.
		log.Printf("faster-whisper exited with: %s", err)
	}

	if _, err := os.Stat(xscriptPath); err != nil {
		fmt.Printf("Unable to transcribe audio: output file not found\n")
		return "", fmt.Errorf("unable to transcribe audio: output file %s not found", xscriptPath)
	}

	// An empty transcript means the model reported success but recognised no speech.
	// Catch it here rather than letting a zero-length file sit in the scratch
	// directory, where it looks like completed work and suppresses every future
	// attempt. The usual cause is a model too small for the material.
	if !util.IsNonEmptyFile(xscriptPath) {
		os.Remove(xscriptPath)
		return "", fmt.Errorf(
			"transcribing %s with model %q produced an empty transcript.\n"+
				"the model recognised no speech. try a larger one, for example:\n"+
				"    whisper-model: small",
			filepath.Base(audioPath), getWhisperModel())
	}

	// record how the transcript was made while that is still known for certain, so
	// the header travels with the file wherever it is copied
	if err := addTranscriptHeader(xscriptPath, transcriptHeader(getWhisperModel(), time.Now())); err != nil {
		return "", fmt.Errorf("cannot add the header to transcript %s: %w", xscriptPath, err)
	}

	return xscriptPath, nil
}

// transcriptHeaderMarker begins the header line written at the top of every transcript.
// It starts like a script comment so the line reads as a note to anyone opening the
// file, and the marker word after it is what tells the header apart from a first line
// someone actually spoke.
const transcriptHeaderMarker = "# wolm-transcript"

// transcriberApp names the transcription software in the header, so a transcript can
// still be told apart if the software is ever changed
const transcriberApp = "faster-whisper"

// transcriptHeader renders the line recording how a transcript was made:
//
//	# wolm-transcript app=faster-whisper model=small transcribed=2026-10-18
//
// Transcripts are kept to be reprocessed later, and by then the model that made one
// may matter - small and large-v3 differ in accuracy, if not in their summaries.
func transcriptHeader(model string, when time.Time) string {
	return fmt.Sprintf("%s app=%s model=%s transcribed=%s",
		transcriptHeaderMarker, transcriberApp, model, when.Format("2006-01-02"))
}

// addTranscriptHeader rewrites a transcript with the header line and a blank line at
// the top. The rewrite goes through a temporary file and a rename, so an interrupted
// run cannot leave the transcript cut short.
func addTranscriptHeader(path, header string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	partial := path + ".partial"
	if err := os.WriteFile(partial, []byte(header+"\n\n"+string(content)), 0666); err != nil {
		os.Remove(partial)
		return err
	}
	if err := os.Rename(partial, path); err != nil {
		os.Remove(partial)
		return err
	}

	return nil
}

// stripTranscriptHeader drops a leading header line, and the blank lines after it,
// before the transcript is sent anywhere. Text without the header - a transcript made
// before headers existed, or one written by hand - is returned unchanged.
func stripTranscriptHeader(text string) string {
	first, rest, _ := strings.Cut(text, "\n")
	first = strings.TrimRight(first, "\r")

	if first != transcriptHeaderMarker && !strings.HasPrefix(first, transcriptHeaderMarker+" ") {
		return text
	}

	return strings.TrimLeft(rest, "\r\n")
}

// getPreservedTranscriptPath returns where a video's transcript is kept: beside the
// video, sharing its name, as .txt so that it opens in Notepad when double-clicked
func getPreservedTranscriptPath(videoPath string) string {
	return strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + ".txt"
}

// getScratchTranscriptPathFromVideoPath returns the scratch .text that transcribing
// this video's audio produces
func getScratchTranscriptPathFromVideoPath(videoPath string) string {
	return getTranscribePathFromAudioPath(getAudioPathFromVideoPath(videoPath))
}

// findExistingTranscript returns the preserved transcript for a video if there is one
// with something in it, or "" if the video needs transcribing.
//
// A transcript in the scratch directory is deliberately not considered, however
// recent. Deleting the .txt beside the video is how a transcript is regenerated, and
// that must work without also knowing to clear out the scratch directory.
func findExistingTranscript(videoPath string) string {
	path := getPreservedTranscriptPath(videoPath)
	if util.IsNonEmptyFile(path) {
		return path
	}
	return ""
}

// preserveTranscript copies a freshly made scratch transcript beside its video and
// returns the path to use from then on.
//
// A copy that fails - a read-only share, a OneDrive folder mid-sync - is not a reason
// to lose the title and description this run is about to generate, so it warns and
// returns the scratch path to carry on with. The transcript just is not kept this time.
//
// A transcript already beside the video is never overwritten: deleting it by hand is
// the only way to regenerate one. An empty one is not a transcript, and is replaced.
func preserveTranscript(scratchPath, videoPath string) string {
	preserved := getPreservedTranscriptPath(videoPath)

	if util.IsNonEmptyFile(preserved) {
		log.Printf("Not overwriting existing transcript %s", preserved)
		return scratchPath
	}

	if err := util.CopyFile(scratchPath, preserved); err != nil {
		fmt.Printf("WARNING: could not save the transcript beside the video: %s\n", err)
		fmt.Printf("         using the copy in the scratch directory, which is swept after %s\n",
			scratchRetention)
		return scratchPath
	}

	fmt.Printf("Saved transcript: %s\n", filepath.Base(preserved))
	return preserved
}
