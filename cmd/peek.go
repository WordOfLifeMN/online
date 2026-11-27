package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// peekCmd represents the peek command
var peekCmd = &cobra.Command{
	Use:   "peek",
	Short: "Peek into the spreadsheet",
	Long: `Basically a debugging command to see if we can access and read the spreadsheet.
This reads the spreadsheet named by --sheet-id - an Excel workbook or a Google Sheet -
and prints each tab's column titles and the first few rows.`,
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
	log.Printf("Checking spreadsheet %s", sheetID)
	src, err := openSheetSource(cmd.Context(), sheetID)
	if err != nil {
		return err
	}
	fmt.Printf("Spreadsheet %s\n", src.Title())

	tabs, err := src.Tabs()
	if err != nil {
		return err
	}

	maxColumns := viper.GetInt("columns")
	maxRows := viper.GetInt("rows")
	for tabIndex, tab := range tabs {
		fmt.Printf("  Sheet #%d : %s\n", tabIndex, tab)

		rows, err := src.Rows(tab)
		if err != nil {
			fmt.Printf("ERROR: Unable to read the sheet: %v\n", err)
			continue
		}
		if len(rows) == 0 {
			fmt.Printf("    (empty)\n")
			continue
		}

		// print columns (title row)
		fmt.Printf("    %d columns : ", len(rows[0]))
		for columnIndex, title := range rows[0] {
			if columnIndex > 0 {
				fmt.Print(", ")
			}
			if columnIndex >= maxColumns {
				fmt.Print("...")
				break
			}
			fmt.Printf("%s", title)
		}
		fmt.Println()

		// print rows (first column). the first row is the column titles
		fmt.Printf("    %d rows : ", len(rows)-1)
		for rowIndex, row := range rows {
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
				fmt.Printf("%q", row[0])
			}
		}
		fmt.Println()
	}

	return nil
}
