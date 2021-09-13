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
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// dumpCmd represents the dump command
var dumpCmd = &cobra.Command{
	Use:     "dump [--sheet-id ID | --input FILE]",
	Short:   "Read the content and output the data in JSON",
	Long:    `Used to make a local copy of the data.`,
	Example: `dump --sheet-id 1vvhIGMPvVF-DtWoYsEbVBvzk_VtLyKuIw_zyLdsB-JY >/tmp/catalog.json`,
	RunE:    dump,
}

func init() {
	rootCmd.AddCommand(dumpCmd)
}

func dump(cmd *cobra.Command, args []string) error {
	initLogging()

	catalog, err := readOnlineContentFromInput(cmd.Context())
	if err != nil {
		return err
	}

	bytes, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return err
	}

	fmt.Print(string(bytes))

	return nil
}
