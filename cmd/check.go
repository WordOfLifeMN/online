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
	"log"

	"github.com/WordOfLifeMN/online/gclient"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate access and spreadsheet configuration",
	Long:  `Ensures that the current information and spreadsheets are set up correctly.`,
	RunE:  check,
}

func init() {
	rootCmd.AddCommand(checkCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// checkCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// checkCmd.Flags().String("sheet-id", "", "ID of Google spreadsheet that contains the series and messages")
	// viper.BindPFlag("sheet-id", checkCmd.Flags().Lookup("sheet-id"))

}

func check(cmd *cobra.Command, args []string) error {
	initLogging()

	sheetID := viper.GetString("sheet-id")

	// get the spreadsheet service
	log.Printf("Checking spreadsheet %s", sheetID)
	service, err := gclient.GetSheetService(cmd.Context())
	if err != nil {
		return err
	}

	// output the sheet information
	spreadsheet, err := service.Spreadsheets.Get(sheetID).Do()
	if err != nil {
		return err
	}
	fmt.Printf("Spreadsheet \"%s\" (%s)\n", spreadsheet.Properties.Title, sheetID)

	// output the sheet info
	for _, sheet := range spreadsheet.Sheets {
		fmt.Printf("  Sheet #%d : %s\n", sheet.Properties.Index, sheet.Properties.Title)
	}

	return nil
}
