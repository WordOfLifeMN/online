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
	"log"

	"github.com/WordOfLifeMN/online/gclient"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// peekCmd represents the peek command
var peekCmd = &cobra.Command{
	Use:   "peek",
	Short: "Peek into the Google Sheet",
	Long: `Basically a debgging command to see if we can access and read a Google sheet.
This reads the Google Sheet associated with the --sheet-id and print some stuff out.`,
	RunE: peek,
}

func init() {
	rootCmd.AddCommand(peekCmd)

	peekCmd.Flags().IntP("columns", "c", 3, "Maximum columns to print [3]")
	viper.BindPFlag("columns", peekCmd.Flags().Lookup("columns"))

	peekCmd.Flags().IntP("rows", "r", 3, "Maximum rows to print [3]")
	viper.BindPFlag("rows", peekCmd.Flags().Lookup("rows"))

}

func peek(cmd *cobra.Command, args []string) error {
	initLogging()

	sheetID := viper.GetString("sheet-id")

	// get the spreadsheet service
	log.Printf("Checking spreadsheet %s", sheetID)
	service, err := gclient.GetSheetService(cmd.Context())
	if err != nil {
		return err
	}

	// output the sheet information
	document, err := service.Spreadsheets.Get(sheetID).Do()
	if err != nil {
		return err
	}
	fmt.Printf("Spreadsheet \"%s\" (%s)\n", document.Properties.Title, sheetID)

	// output the sheet info
	maxColumns := viper.GetInt("columns")
	maxRows := viper.GetInt("rows")
	for _, sheet := range document.Sheets {
		fmt.Printf("  Sheet #%d : %s\n", sheet.Properties.Index, sheet.Properties.Title)

		// print columns (title row)
		titleRange := fmt.Sprintf("'%s'!1:1", sheet.Properties.Title)
		values, err := service.Spreadsheets.Values.Get(sheetID, titleRange).Do()
		if err != nil {
			fmt.Printf("ERROR: Unable to get the column titles: %v", err)
			continue
		}
		fmt.Printf("    %d columns : ", len(values.Values[0]))
		for columnIndex, v := range values.Values[0] {
			if columnIndex > 0 {
				fmt.Print(", ")
			}
			if columnIndex >= maxColumns {
				fmt.Print("...")
				break
			}
			fmt.Printf("%s", v.(string))
		}
		fmt.Println()

		// print rows (titles). assume first row is still column titles
		titleRange = fmt.Sprintf("'%s'!A:A", sheet.Properties.Title)
		values, err = service.Spreadsheets.Values.Get(sheetID, titleRange).Do()
		if err != nil {
			fmt.Printf("ERROR: Unable to get the row titles: %v", err)
			continue
		}
		fmt.Printf("    %d rows : ", len(values.Values)-1)
		for rowIndex, row := range values.Values {
			if rowIndex == 0 {
				continue
			}
			if rowIndex > 1 {
				fmt.Print(", ")
			}
			if rowIndex >= maxRows+1 {
				fmt.Print("...")
				break
			}
			if len(row) == 0 {
				fmt.Printf("\"\"")
			} else {
				fmt.Printf("%q", row[0].(string))
			}
		}
		fmt.Println()
	}

	return nil
}
