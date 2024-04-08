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
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/util"
	"github.com/spf13/cobra"
)

type podcastCmdStruct struct {
	cobra.Command // the podcast command definition

	// flags for this command
	Days           int    // number of days of history the podcast should include
	Ministry       string // which ministry the podcast should be for
	OutputFileName string // where to write the podcast file to
}

// podcastCmd represents the podcast command
var podcastCmd *podcastCmdStruct

func init() {
	podcastCmd = &podcastCmdStruct{
		Command: cobra.Command{
			Use:   "podcast [--days n]",
			Short: "Output a podcast document for recent messages",
			Long:  `Output a podcast for recent messages.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				return podcastCmd.podcast()
			},
		},
	}

	rootCmd.AddCommand(&podcastCmd.Command)

	podcastCmd.Flags().IntVarP(&podcastCmd.Days, "days", "d", 180, "How many days of history should be included in the podcast")
	podcastCmd.Flags().StringVarP(&podcastCmd.Ministry, "ministry", "m", "wol", "Which ministry should the podcast be generated for?")
	podcastCmd.Flags().StringVarP(&podcastCmd.OutputFileName, "output", "o", "", "Where to write the podcast file. Default is stdout.")
}

func (cmd *podcastCmdStruct) podcast() error {
	initLogging()

	ministry := catalog.NewMinistryFromString(cmd.Ministry)
	if ministry == catalog.UnknownMinistry {
		return fmt.Errorf("ministry '%s' is unknown", cmd.Ministry)
	}

	// get the catalog
	cat, err := readOnlineContentFromInput(cmd.Context())
	if err != nil {
		return err
	}

	// determine the maximum age of items in the podcast
	cutoff := time.Now().AddDate(0, 0, -1*cmd.Days)

	// get messages that match
	//  - ministry == WOL
	//  - visibility == Public
	//  - date > cutoff
	//  - playlist == Service
	messages := []catalog.CatalogMessage{}
	for _, msg := range cat.Messages {
		if msg.Ministry != catalog.WordOfLife ||
			msg.Visibility != catalog.Public ||
			msg.Date.Before(cutoff) {
			continue
		}

		found := false
		for _, playlist := range msg.Playlist {
			found = found || playlist == "service"
		}
		if !found {
			continue
		}

		messages = append(messages, msg)
	}

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Date.Time.Before(messages[j].Date.Time)
	})

	data := map[string]interface{}{
		"Title":         "Word of Life Ministries: Sunday",
		"Description":   "Podcast of Word of Life Ministries Sunday services",
		"CopyrightYear": time.Now().Year(),
		"Messages":      messages,
	}

	var outFile io.Writer = os.Stdout
	outFileName := cmd.OutputFileName
	if outFileName != "" && !strings.Contains(outFileName, "stdout") {
		f, err := os.Create(util.NormalizePath(outFileName))
		if err != nil {
			return err
		}
		defer f.Close()
		outFile = f
	}
	return cmd.printPodcast(data, outFile)
}

// printPodcast prints a RSS podcast file to the writer. data contains the data
// to use for the template rendering:
//   - Title - string
//   - Description - string
//   - CopyrightYear - int - 4-digit copyright year
//   - Messages - []CatalogMessage - list of messages to display (in order to display)
func (cmd *podcastCmdStruct) printPodcast(data map[string]interface{}, output io.Writer) error {
	// get the podcast template
	templateName, err := getTemplatePath("podcast.xml")
	if err != nil {
		return fmt.Errorf("cannot find template for 'podcast': %w", err)
	}

	t := template.New("podcast")
	t.Funcs(template.FuncMap{
		"xml": func(s string) string {
			var b bytes.Buffer
			xml.EscapeText(&b, []byte(s))
			return b.String()
		},
	})
	t, err = t.ParseFiles(templateName)
	if err != nil {
		return fmt.Errorf("cannot read template '%s': %w", templateName, err)
	}

	err = t.ExecuteTemplate(output, "podcast.xml", data)
	if err != nil {
		return fmt.Errorf("failed to execute the template '%s': %w", templateName, err)
	}

	return nil
}
