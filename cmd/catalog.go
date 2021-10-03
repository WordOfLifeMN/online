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
	"html/template"
	"io"
	"log"
	"path/filepath"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/spf13/cobra"
)

type catalogCmdStruct struct {
	cobra.Command // catalog command definition

	// flags for the catalog command
	Ministry  string // which ministry to generate catalog for, "all" or "*" for all
	View      string // which view to generate catalog for, "all" or "*" for all
	OutputDir string // directory to write output to

	// internal reference
	template      *template.Template // html templates for generating pages
	templateError error              // cached error from trying to load a template
}

// catalogCmd represents the catalog command
var catalogCmd *catalogCmdStruct

func init() {
	catalogCmd = &catalogCmdStruct{
		Command: cobra.Command{
			Use:   "catalog [--ministry=all] [--view=all] [--output=~/.wolm/online]",
			Short: "Generate the online catalog",
			Long: `Generates the online catalog.
	
By default this generates the catalog for all views and ministries, but can 
be limited with the parameters.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				return catalogCmd.catalog()
			},
		},
	}

	rootCmd.AddCommand(&catalogCmd.Command)

	catalogCmd.Flags().StringVar(&catalogCmd.Ministry, "ministry", "all", "Ministry to generate catalog for: all (default), wol, core, tbo, ask-pastor, or faith-freedom")
	catalogCmd.Flags().StringVar(&catalogCmd.View, "view", "all", "View for catalog: all (default), public, partner, private")
	catalogCmd.Flags().StringVarP(&catalogCmd.OutputDir, "output", "o", "~/.wolm/online", "Output directory. Defaults to $HOME/.wolm/online")
}

func (cmd *catalogCmdStruct) catalog() error {
	initLogging()

	// find ministries to generate catalogs for
	var ministries []catalog.Ministry
	ministry := catalog.NewMinistryFromString(cmd.Ministry)
	if ministry == catalog.UnknownMinistry {
		ministries = []catalog.Ministry{
			catalog.WordOfLife,
			catalog.AskThePastor,
			catalog.CenterOfRelationshipExperience,
			catalog.FaithAndFreedom,
			catalog.TheBridgeOutreach,
		}
	} else {
		ministries = append(ministries, ministry)
	}

	// find views to generate catalogs for
	var views []catalog.View
	view := catalog.NewViewFromString(cmd.View)
	if view == catalog.UnknownView {
		views = []catalog.View{
			catalog.Public,
			catalog.Partner,
			catalog.Private,
		}
	} else {
		views = append(views, view)
	}

	// get the catalog
	// cat, err := readOnlineContentFromInput(cmd.Context())
	// if err != nil {
	// 	return err
	// }

	return nil
}

// loadTemplates all the templates for processing catalog files. Finds all the templates that
// match catalog.*.html in the templates directory. If this returns an error, then the templates
// could not be loaded and subsequent calls to the print methods will fail
func (cmd *catalogCmdStruct) loadTemplates() error {
	if cmd.templateError != nil {
		// we already tried and cached an error so don't try again
		return cmd.templateError
	}

	templateDir, err := getTemplateDir()
	if err != nil {
		return err
	}
	log.Printf("Loading templates from %s", filepath.Join(templateDir, "catalog.*.html"))
	cmd.template = template.New("catalog")
	cmd.template, err = cmd.template.ParseGlob(filepath.Join(templateDir, "catalog.*.html"))
	if err != nil {
		return err
	}

	log.Printf("Loaded templates%s", cmd.template.DefinedTemplates())

	return nil
}

func (cmd *catalogCmdStruct) printCatalogSeri(seri *catalog.CatalogSeri, output io.Writer) error {
	if err := cmd.loadTemplates(); err != nil {
		return err
	}

	return cmd.template.ExecuteTemplate(output, "catalog.seri.html", seri)
}
