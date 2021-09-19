/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

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
	"html/template"
	"io"
	"os"
	"time"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// podcastCmd represents the podcast command
var podcastCmd = &cobra.Command{
	Use:   "podcast [--days n]",
	Short: "Output a podcast document for recent messages",
	Long:  `Output a podcast for recent messages.`,
	RunE:  podcast,
}

func init() {
	rootCmd.AddCommand(podcastCmd)

	podcastCmd.Flags().IntP("days", "d", 180, "How many days of history should be included in the podcast")
	viper.BindPFlag("days", podcastCmd.Flags().Lookup("days"))

	podcastCmd.Flags().StringP("ministry", "m", "wol", "Which ministry should the podcast be generated for?")
	viper.BindPFlag("ministry", podcastCmd.Flags().Lookup("ministry"))
}

func podcast(cmd *cobra.Command, args []string) error {
	ministry := catalog.NewMinistryFromString(viper.GetString("ministry"))
	if ministry == catalog.UnknownMinistry {
		return fmt.Errorf("ministry '%s' is unknown", viper.GetString("ministry"))
	}

	// get the catalog
	cat, err := readOnlineContentFromInput(cmd.Context())
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"title":         "Word of Life Ministries: Sunday",
		"description":   "Podcast of Word of Life Ministries Sunday services",
		"copyrightYear": time.Now().Year(),
	}

	// TODO - filter the catalog
	data["messages"] = []catalog.CatalogMessage{}
	cat = cat // suppress warning

	// TODO - do we want to create multiple podcasts to different files?

	return printPodcast(data, os.Stdout)
}

// printPodcast prints a RSS podcast file to the writer. data contains the data
// to use for the template rendering:
//  - Title - string
//  - Description - string
//  - CopyrightYear - int - 4-digit copyright year
//  - Messages - []CatalogMessage - list of messages to display (in order to display)
func printPodcast(data map[string]interface{}, output io.Writer) error {
	// get the podcast template
	// TODO: how is this going to work from any wd?
	fileName := "../templates/podcast.xml"
	template, err := template.ParseFiles(fileName)
	if err != nil {
		return fmt.Errorf("cannot read template '%s': %w", fileName, err)
	}

	return template.Execute(output, data)
}
