/*
Copyright © 2021 Word of Life Ministries <info@wordoflifemn.org>

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
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/gclient"
	"github.com/spf13/cobra"

	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "online",
	Short: "Generates online media content for Word of Life Ministries",
	Long: `This client-side application will read Google Sheets containing information
series or messages that are presented, then generate the static files for 
accessing the content online.

Supports generating a RSS podcast as well as a HTML static website.`,
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

}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		// home, err := os.UserHomeDir()
		// cobra.CheckErr(err)

		// Search config in $HOME/.wolm directory with name "online" (without extension).
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.wolm")
		viper.SetConfigType("yaml")
		viper.SetConfigName("online")
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
		switch {
		case strings.HasSuffix(strings.ToUpper(inputFile), ".JSON"):
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

func getTemplatePath(templateName string) (string, error) {
	// read template path from configuration
	templateDir := viper.GetString("template-dir")
	templatePath := filepath.Join(templateDir, templateName)
	if _, err := os.Stat(templatePath); !os.IsNotExist(err) {
		log.Printf("Template from config: %s", templatePath)
		return templatePath, nil
	}

	// find a path relative to the executable
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	execDir := filepath.Dir(execPath)
	templatePath = filepath.Join(execDir, "templates", templateName)
	if _, err = os.Stat(templatePath); !os.IsNotExist(err) {
		log.Printf("Template from executable: %s", templatePath)
		return templatePath, nil
	}

	templatePath = filepath.Join("/Users/kmurray/git/go/src/github.com/WordOfLifeMN/online/templates", templateName)
	if _, err := os.Stat(templatePath); !os.IsNotExist(err) {
		log.Printf("Template hardcoded: %s", templatePath)
		return templatePath, nil
	}

	return "", fmt.Errorf("cannot find templates to use to generate web pages")
}
