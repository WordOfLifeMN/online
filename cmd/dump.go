package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/WordOfLifeMN/online/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

	dumpCmd.Flags().StringP("output", "o", "~/.wolm/online.cache.json", "File to output to")
	viper.BindPFlag("output", dumpCmd.Flags().Lookup("output"))
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

	outFileName := viper.GetString("output")
	// fmt.Printf("TODO(km) outFileName = %s\n", outFileName)
	if outFileName == "" || strings.Contains(outFileName, "stdout") {
		fmt.Print(string(bytes))
	} else {
		outFile, err := os.Create(util.NormalizePath(outFileName))
		if err != nil {
			return err
		}
		defer outFile.Close()
		log.Printf("Writing message data to %s\n", outFileName)
		fmt.Fprint(outFile, string(bytes))
	}

	return nil
}
