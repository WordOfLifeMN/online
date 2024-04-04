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
	"path"
	"path/filepath"
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

	rootCmd.PersistentFlags().String("openai-key", "", "OpenAI API key")
	viper.BindPFlag("openai-key", rootCmd.PersistentFlags().Lookup("openai-key"))
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

// getTemplatePath finds the template with the specified name in the template directory. Returns
// err if a template with the name cannot be found
func getTemplatePath(templateName string) (string, error) {
	templateDir, err := getTemplateDir()
	if err != nil {
		return "", err
	}

	templatePath := filepath.Join(templateDir, templateName)
	if util.DoesPathExist(templatePath) {
		return templatePath, nil
	}

	return "", fmt.Errorf("cannot find template %s", templatePath)
}

// getTemplateDir finds the directory that stores templates for rendering pages
func getTemplateDir() (string, error) {
	// check the configured directory: for when running binary executable with a configuration
	// file
	templateDir := viper.GetString("template-dir")
	if templateDir != "" {
		// log.Printf("Looking for template dir in config: %s", templateDir)
		if util.IsDirectory(templateDir) {
			return templateDir, nil
		}
	}

	// check for a template directory relative to the executable: for when running the
	// executable in the project directory
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	execDir := filepath.Dir(execPath)
	templateDir = filepath.Join(execDir, "templates")
	// log.Printf("Looking for template dir relative to executable: %s", templateDir)
	if util.IsDirectory(templateDir) {
		return templateDir, nil
	}

	// check the current working directory: for when running a go tool like "go run"
	cwDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	templateDir = filepath.Join(cwDir, "templates")
	// log.Printf("Looking for template dir in cwd: %s", templateDir)
	if util.IsDirectory(templateDir) {
		return templateDir, nil
	}

	// check relative to the current working directory: for when running go tests, where the cwd
	// would be the pkg dir in the project
	templateDir = filepath.Join(cwDir, "..", "templates")
	// log.Printf("Looking for template dir relative to cwd: %s", templateDir)
	if util.IsDirectory(templateDir) {
		return templateDir, nil
	}

	return "", fmt.Errorf("unable to find the template directory")
}
