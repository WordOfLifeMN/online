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
	"github.com/WordOfLifeMN/online/gclient"
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
title and description, looks the message up in the Google Sheet for its series,
track, ministry and visibility, and assembles everything needed to upload the
message by hand.`,
	SilenceUsage: true,
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

	rootCmd.PersistentFlags().String("sheet-id", "", "ID of Google spreadsheet that contains the series and messages")
	viper.BindPFlag("sheet-id", rootCmd.PersistentFlags().Lookup("sheet-id"))

	rootCmd.PersistentFlags().StringP("input", "i", "", "Path to JSON file to read catalog from (overrides --sheet-id)")
	viper.BindPFlag("input", rootCmd.PersistentFlags().Lookup("input"))

	rootCmd.PersistentFlags().String("anthropic-model", "", "Model used to generate titles and descriptions")
	viper.BindPFlag("anthropic-model", rootCmd.PersistentFlags().Lookup("anthropic-model"))

	rootCmd.PersistentFlags().String("scratch-dir", "", "Directory for intermediate audio and transcript files")
	viper.BindPFlag("scratch-dir", rootCmd.PersistentFlags().Lookup("scratch-dir"))

	rootCmd.PersistentFlags().String("whisper-model", "", "Transcription model to use")
	viper.BindPFlag("whisper-model", rootCmd.PersistentFlags().Lookup("whisper-model"))
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
		log.Printf("    %s = %s", key, viper.GetString(key))
	}

}

// readOnlineContentFromInput reads the content of a catalog from wherever
// requested. If there is an --input parameter, then it is read from that file.
// Otherwise, it is read from the --sheet-id. If there is no --input or --sheet-id, then an error is returned
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

	// check if reading from Google Sheet
	sheetID := viper.GetString("sheet-id")
	if sheetID != "" {
		sheetService, err := gclient.GetSheetService(ctx)
		if err != nil {
			return nil, err
		}

		return gclient.NewCatalogFromSheet(sheetService, sheetID)
	}

	// no input
	return nil, fmt.Errorf("no input specified. please provide an --input or --sheet-id parameter, or configure a default sheet-id in the ~/.wolm/online.yaml file")
}

