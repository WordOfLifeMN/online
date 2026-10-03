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
// The transcript is a disposable input to title and description generation and is
// never published, so the cheapest model that still yields usable text is the right
// default - but "cheapest" turned out to be further up the range than expected.
// Measured on a real message:
//
//	tiny.en   produced an EMPTY transcript while reporting success
//	base.en   worked, but is not bundled and has to be downloaded, and misheard words
//	small     worked, is already bundled, and was the most accurate of the three
//
// On a CUDA machine small runs at roughly 34 audio-seconds per second, so even a two
// hour service transcribes in about four minutes. There is no reason to go lower.
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
