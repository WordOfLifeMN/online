package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// audioExtractCmd represents the command to extract audio from video
var audioExtractCmd = &cobra.Command{
	Use:   "extract video-file",
	Short: "Extract audio track from video",
	Long: `Extracts the audio as an mp3 file from the mp4 video file.

The input file must be .mp4. The audio is written to the scratch directory as an
.mp3 file. It is an intermediate on the way to a transcript and is not preserved.

Requires 'ffmpeg' be installed and accessible on the path.`,
	RunE: audioExtract,
}

func init() {
	audioCmd.AddCommand(audioExtractCmd)

	audioExtractCmd.Args = cobra.MaximumNArgs(1)
}

func audioExtract(cmd *cobra.Command, args []string) error {
	initLogging()

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

	fmt.Printf("Extracted to %s\n", audioPath)
	return nil
}

func extractAudioFromVideo(videoPath string) (string, error) {
	audioPath := getAudioPathFromVideoPath(videoPath)
	if err := deleteExistingFile(audioPath, true); err != nil {
		return "", err
	}

	// make sure the scratch directory exists
	if err := os.MkdirAll(filepath.Dir(audioPath), os.FileMode(0777)); err != nil {
		return "", fmt.Errorf("cannot create scratch directory %s: %w", filepath.Dir(audioPath), err)
	}

	// compute trim length based on file name
	trimLen := 9.8
	if strings.Contains(videoPath, " FF ") {
		trimLen = 9.9
	} else if strings.Contains(videoPath, " CORE ") {
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
