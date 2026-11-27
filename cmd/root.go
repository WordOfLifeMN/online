package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/sheet"
	"github.com/WordOfLifeMN/online/util"
	"github.com/spf13/cobra"

	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "online",
	Short: "Prepares Word of Life Ministries messages for publication",
	Long: `This client-side application prepares recorded messages for publication to
YouTube.

Given an edited video it extracts the audio, transcribes it, generates a suggested
title and description, looks the message up in the spreadsheet for its series,
track, ministry and visibility, and assembles everything needed to upload the
message by hand.`,
	SilenceUsage: true,

	// Execute() reports the error through cobra.CheckErr. Without this, cobra prints
	// it too and every failure appears twice.
	SilenceErrors: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Include logging")
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	rootCmd.PersistentFlags().String("sheet-id", "", "Spreadsheet of series and messages: the path of an Excel workbook (.xlsx), or the ID of a Google Sheet")
	viper.BindPFlag("sheet-id", rootCmd.PersistentFlags().Lookup("sheet-id"))

	rootCmd.PersistentFlags().StringP("input", "i", "", "Path to JSON file to read catalog from (overrides --sheet-id)")
	viper.BindPFlag("input", rootCmd.PersistentFlags().Lookup("input"))

	rootCmd.PersistentFlags().String("anthropic-model", "", "Model used to generate titles and descriptions")
	viper.BindPFlag("anthropic-model", rootCmd.PersistentFlags().Lookup("anthropic-model"))

	rootCmd.PersistentFlags().String("scratch-dir", "", "Directory for intermediate audio and transcript files")
	viper.BindPFlag("scratch-dir", rootCmd.PersistentFlags().Lookup("scratch-dir"))

	rootCmd.PersistentFlags().String("whisper-model", "", "Transcription model to use")
	viper.BindPFlag("whisper-model", rootCmd.PersistentFlags().Lookup("whisper-model"))

	rootCmd.PersistentFlags().String("thumb-dir", "", "Directory holding the generic fallback thumbnails")
	viper.BindPFlag("thumb-dir", rootCmd.PersistentFlags().Lookup("thumb-dir"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in $HOME/.wolm directory with name "online" (without extension).
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.wolm")
		viper.SetConfigType("yaml")
		viper.SetConfigName("online-config")

		// For windows
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		viper.AddConfigPath(path.Join(home, ".wolm"))
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		err = fmt.Errorf("cannot read configuration file: %w", err)
		fmt.Fprintf(os.Stderr, "%s", err.Error())
	}
}

// initLogging updates the configuration for the default logger
func initLogging() {
	// disable logging if not verbose
	verbose := viper.GetBool("verbose")
	if !verbose {
		log.Default().SetOutput(io.Discard)
		return
	}

	// since we're verbose, let's dump the configuration
	log.Printf("Using config file: %s", viper.ConfigFileUsed())
	for _, key := range viper.AllKeys() {
		log.Printf("    %s = %s", key, redactIfSecret(key, viper.GetString(key)))
	}

}

// redactIfSecret hides the value of configuration keys that hold credentials.
//
// The verbose configuration dump is on by default for anyone launching through
// bin/wolm-audio.bat, so without this the API key would be printed to the console on
// every single run and end up in any captured output.
func redactIfSecret(key string, value string) string {
	if value == "" {
		return value
	}

	lowered := strings.ToLower(key)
	for _, marker := range []string{"key", "token", "secret", "password", "credential"} {
		if strings.Contains(lowered, marker) {
			return fmt.Sprintf("(redacted, %d chars)", len(value))
		}
	}

	return value
}

// readOnlineContentFromInput reads the content of a catalog from wherever
// requested. If there is an --input parameter, then it is read from that file.
// Otherwise, it is read from the --sheet-id: an Excel workbook when that is a path,
// else a Google Sheet. If there is no --input or --sheet-id, then an error is returned
func readOnlineContentFromInput(ctx context.Context) (*catalog.Catalog, error) {

	// check if reading from file
	inputFile := viper.GetString("input")
	if inputFile != "" {
		inputFile = util.NormalizePath(inputFile)
		if strings.HasSuffix(strings.ToUpper(inputFile), ".JSON") {
			return catalog.NewCatalogFromJSON(inputFile)
		}
		return nil, fmt.Errorf("filetype %s is not supported", inputFile)
	}

	// check if reading from the spreadsheet: a Google Sheet or an Excel workbook
	sheetID := viper.GetString("sheet-id")
	if sheetID != "" {
		src, err := openSheetSource(ctx, sheetID)
		if err != nil {
			return nil, err
		}

		return sheet.NewCatalog(src)
	}

	// no input
	return nil, fmt.Errorf("no input specified. please provide an --input or --sheet-id parameter, or configure a default sheet-id in the ~/.wolm/online-config.yaml file")
}
