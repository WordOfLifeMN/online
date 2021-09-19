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
		return fmt.Errorf("The online catalog was not valid (see errors above)")
	}

	return nil
}
