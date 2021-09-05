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
	"fmt"
	"io"
	"log"
	"os"

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
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	rootCmd.PersistentFlags().String("sheet-id", "", "ID of Google spreadsheet that contains the series and messages")
	viper.BindPFlag("sheet-id", rootCmd.PersistentFlags().Lookup("sheet-id"))

	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Include logging")
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
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
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

// initLogging updates the configuration for the default logger
func initLogging() {
	// disable logging if not verbose
	verbose := viper.GetBool("verbose")
	if !verbose {
		log.Default().SetOutput(io.Discard)
	}
}
