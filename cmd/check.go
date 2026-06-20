package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check [--input FILE | --sheet-id ID]",
	Short: "Validate a catalog has correct data in it.",
	Long: `Ensures that an online content catalog is internall consistent.

Validates:
- All series referenced by messages actually exist`,
	RunE: check,
	Args: cobra.NoArgs,
}

func init() {
	rootCmd.AddCommand(checkCmd)
}

func check(cmd *cobra.Command, args []string) error {
	initLogging()

	// // testing output streams
	// fmt.Fprintf(os.Stdout, "Sent to stdout\n")
	// fmt.Fprintf(os.Stderr, "Sent to stderr\n")
	// log.Printf("Sent to log\n")
	// return nil

	catalog, err := readOnlineContentFromInput(cmd.Context())
	if err != nil {
		return err
	}

	valid := catalog.IsValid(true)

	if !valid {
		return fmt.Errorf("the online catalog was not valid (see errors above)")
	}

	fmt.Printf("Online content is valid\n")

	return nil
}
