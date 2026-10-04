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

The input file must be .mp3 and the transcript is written to the scratch directory.
The transcript is an intermediate used to generate a title and description; it is
not published or preserved.

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
// the resulting text file. Only text output is produced - the .vtt, .srt, .tsv and
// .json formats existed to publish transcripts online, which we no longer do.
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

	return xscriptPath, nil
}
