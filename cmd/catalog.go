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
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/util"
	"github.com/otiai10/copy"
	"github.com/spf13/cobra"
)

type catalogCmdStruct struct {
	cobra.Command // catalog command definition

	// flags for the catalog command
	Ministry  string // which ministry to generate catalog for, "all" or "*" for all
	View      string // which view to generate catalog for, "all" or "*" for all
	OutputDir string // directory to write output to

	// internal reference
	cat           *catalog.Catalog   // the catalog to process
	template      *template.Template // html templates for generating pages
	templateError error              // cached error from trying to load a template
}

const (
	FLAG_FILE_NAME string = "is.online.catalog.dir"
)

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
	log.Printf("Generating catalog in directory %s", cmd.OutputDir)

	// find ministries to generate catalogs for
	var ministries []catalog.Ministry
	ministry := catalog.NewMinistryFromString(cmd.Ministry)
	if ministry != catalog.UnknownMinistry {
		ministries = append(ministries, ministry)
	} else if cmd.Ministry == "all" || cmd.Ministry == "*" {
		ministries = []catalog.Ministry{
			catalog.WordOfLife,
			catalog.AskThePastor,
			catalog.CenterOfRelationshipExperience,
			catalog.FaithAndFreedom,
			catalog.TheBridgeOutreach,
		}
	} else {
		return fmt.Errorf("unknown ministry '%s'", cmd.Ministry)
	}

	// find views to generate catalogs for
	var views []catalog.View
	view := catalog.NewViewFromString(cmd.View)
	if view != catalog.UnknownView {
		views = append(views, view)
	} else if cmd.View == "all" || cmd.View == "*" {
		views = []catalog.View{
			catalog.Public,
			catalog.Partner,
			catalog.Private,
		}
	} else {
		return fmt.Errorf("unknown view '%s'", cmd.View)
	}

	// get the catalog
	var err error
	cmd.cat, err = readOnlineContentFromInput(cmd.Context())
	if err != nil {
		return err
	}
	if !cmd.cat.IsValid(false) {
		return fmt.Errorf("catalog is not valid. run 'check' on it")
	}
	if err := cmd.cat.Initialize(); err != nil {
		return err
	}

	// load our templates
	if err := cmd.loadTemplates(); err != nil {
		return fmt.Errorf("unable to load templates for generating the catalog: %w", err)
	}

	// set up the output directory with the static files
	if err := cmd.initializeOutputDir(); err != nil {
		return err
	}
	if err := cmd.copyStaticFilesToOutputDir(ministries); err != nil {
		return err
	}

	// generate ministry styles
	log.Printf("Generating style sheets")
	for _, ministry := range ministries {
		log.Printf("  Ministry %s", ministry.String())
		if err := cmd.createStyleSheet(ministry); err != nil {
			return err
		}
	}

	// generate the seri pages
	log.Printf("Generating seri pages")
	for _, ministry := range ministries {
		for _, view := range views {
			log.Printf("  Ministry %s (%s)", ministry.String(), string(view))
			if err := cmd.createAllCatalogSeriPages(ministry, catalog.Public); err != nil {
				return err
			}
		}
	}

	return nil
}

// loadTemplates all the templates for processing catalog files. Finds all the templates that
// match catalog.*.html in the templates directory. If this returns an error, then the templates
// could not be loaded and subsequent calls to the print methods will fail
func (cmd *catalogCmdStruct) loadTemplates() error {
	if cmd.template != nil {
		// already loaded the templates
		return nil
	}
	if cmd.templateError != nil {
		// we already tried and cached an error so don't try again
		return cmd.templateError
	}

	log.Printf("Loading templates")
	templateDir, err := getTemplateDir()
	if err != nil {
		return err
	}

	// parse the templates
	cmd.template = template.New("catalog")
	log.Printf("  Templates %s", filepath.Join(templateDir, "catalog.css"))
	cmd.template, err = cmd.template.ParseGlob(filepath.Join(templateDir, "catalog.css"))
	if err != nil {
		return err
	}
	log.Printf("  Templates %s", filepath.Join(templateDir, "catalog.*.html"))
	cmd.template, err = cmd.template.ParseGlob(filepath.Join(templateDir, "catalog.*.html"))
	if err != nil {
		return err
	}

	log.Printf("Loaded templates%s", cmd.template.DefinedTemplates())

	return nil
}

// ----------------------------------------------------------------------------
// | Output directory management
// ----------------------------------------------------------------------------

// initializeOutputDir sets up the output directory by making sure it exists and is empty except
// for the flag file. If the directory already exists and contains the flag file, the contents
// will be deleted so we can start with a clean slate. If the directory exists but does not
// contain the flag file, then an error will be returned because this might not be a directory
// it is safe to delete
func (cmd *catalogCmdStruct) initializeOutputDir() error {
	if cmd.OutputDir == "" {
		return fmt.Errorf("no output directory specified")
	}

	// check if the directory is ok for our output directory
	if util.IsDirectory(cmd.OutputDir) && !util.IsFile(filepath.Join(cmd.OutputDir, FLAG_FILE_NAME)) {
		return fmt.Errorf(`output directory %s exists, but not recognized as an online 
catalog directory  because it doesn't contain the file '%s'. aborting 
the catalog generation because I'm not sure it's safe to delete this directory. 
before running again, either delete this directory or create the file 
%s/%s manually`,
			cmd.OutputDir, FLAG_FILE_NAME, cmd.OutputDir, FLAG_FILE_NAME)
	}

	// delete the directory and all the files
	if err := os.RemoveAll(cmd.OutputDir); err != nil {
		return fmt.Errorf("unable to delete all the files in %s", cmd.OutputDir)
	}

	// create the directory and our flag-file
	if err := os.MkdirAll(cmd.OutputDir, os.FileMode(0777)); err != nil {
		return fmt.Errorf("cannot create the output directory %s: %w", cmd.OutputDir, err)
	}
	if _, err := os.Create(filepath.Join(cmd.OutputDir, FLAG_FILE_NAME)); err != nil {
		return fmt.Errorf("unable to create file %s", filepath.Join(cmd.OutputDir, FLAG_FILE_NAME))
	}

	return nil
}

// copyStaticFilesToOutputDir copies all the static files to the output directory. This includes
// any image files or other files that are not generated dynamically.
func (cmd *catalogCmdStruct) copyStaticFilesToOutputDir(ministries []catalog.Ministry) error {
	// find the static directory
	templateDir, err := getTemplateDir()
	if err != nil {
		return err
	}
	staticDir := filepath.Join(templateDir, "static")
	if !util.IsDirectory(staticDir) {
		return fmt.Errorf("cannot find static template directory %s", staticDir)
	}

	// build a list of the prefixes we need to copy
	prefixesToCopy := []string{"all."}
	for _, ministry := range ministries {
		prefixesToCopy = append(prefixesToCopy, string(ministry)+".")
	}

	// copy the files
	log.Printf("Copying static files:")
	opt := copy.Options{
		Skip: func(src string) (bool, error) {
			for _, prefix := range prefixesToCopy {
				if strings.HasPrefix(filepath.Base(src), prefix) {
					log.Printf("  %s", src)
					return false, nil
				}
			}
			return true, nil
		},
	}
	return copy.Copy(staticDir, cmd.OutputDir, opt)
}

// getOutputFilePath generates the path of an output file based on the output parameters. Given a
// file name, will return the full path to the file to create
func (cmd *catalogCmdStruct) getOutputFilePath(fileName string) string {
	return filepath.Join(cmd.OutputDir, fileName)
}

// ----------------------------------------------------------------------------
// | Style sheet pages
// ----------------------------------------------------------------------------

// createStyleSheet creates the style sheet for the specified ministry in the output directory
func (cmd *catalogCmdStruct) createStyleSheet(ministry catalog.Ministry) error {
	// create the file
	filePath := cmd.getOutputFilePath(fmt.Sprintf("catalog.%s.css", string(ministry)))
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("cannot create style sheet file %s", filePath)
	}
	defer f.Close()

	// write the style sheet to it
	return cmd.printCatalogStyle(ministry, f)
}

// printCatalogStyle prints the style sheet file (CSS) for the specific ministry to the writer
func (cmd *catalogCmdStruct) printCatalogStyle(ministry catalog.Ministry, output io.Writer) error {
	if err := cmd.loadTemplates(); err != nil {
		return err
	}
	return cmd.template.ExecuteTemplate(output, "catalog.css", ministry)
}

// ----------------------------------------------------------------------------
// | Pages containing a single series
// ----------------------------------------------------------------------------

// createAllCatalogSeriPages generates all the series pages for a specific ministry and view.
// The name of each page will be the series ID for public pages and a mystic hash if the view is
// not public, this just makes the private pages harder to guess
func (cmd *catalogCmdStruct) createAllCatalogSeriPages(ministry catalog.Ministry, view catalog.View) error {
	seriList := cmd.cat.FindSeriesByMinistry(ministry)
	seriList = catalog.FilterSeriesByView(seriList, view)

	if len(seriList) == 0 {
		log.Printf("    (no series found)")
	}

	for _, seri := range seriList {
		if err := cmd.createCatalogSeriPage(&seri, view); err != nil {
			return err
		}
	}

	return nil
}

// createCatalogSeri creates a catalog page for a single series to a file in the output
// directory. The name of the page will be the series ID (+ .html)
func (cmd *catalogCmdStruct) createCatalogSeriPage(seri *catalog.CatalogSeri, view catalog.View) error {
	filePath := cmd.getOutputFilePath(seri.GetViewID(view) + ".html")
	log.Printf("    %s --> %s", seri.Name, filePath)

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("cannot create output file %s: %w", filePath, err)
	}
	defer f.Close()

	return cmd.printCatalogSeri(seri, f)
}

// printCatalogSeri prints a catalog page for a single series to the writer
func (cmd *catalogCmdStruct) printCatalogSeri(seri *catalog.CatalogSeri, output io.Writer) error {
	if err := cmd.loadTemplates(); err != nil {
		return err
	}

	data := struct {
		Date     catalog.DateOnly
		Ministry catalog.Ministry
		Seri     *catalog.CatalogSeri
	}{
		Date:     catalog.NewDateToday(),
		Ministry: seri.GetMinistry(),
		Seri:     seri,
	}

	return cmd.template.ExecuteTemplate(output, "catalog.seri.html", data)
}