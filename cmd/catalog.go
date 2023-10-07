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
	"sort"
	"strings"
	"time"

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
	Days      int    // number of days to include in recent messages

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

	catalogCmd.Flags().StringVar(&catalogCmd.Ministry, "ministry", "all", "Ministry to generate catalog for: all (default), wol, core (with sub-ministry), tbo, ask-pastor, or faith-freedom")
	catalogCmd.Flags().StringVar(&catalogCmd.View, "view", "all", "View for catalog: all (default), public, partner, private")
	catalogCmd.Flags().StringVarP(&catalogCmd.OutputDir, "output", "o", "~/.wolm/online", "Output directory. Defaults to $HOME/.wolm/online")
	catalogCmd.Flags().IntVar(&catalogCmd.Days, "days", 60, "Number of days to include in the recent message pages. Defaults to 60")
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
		ministries = append([]catalog.Ministry{}, catalog.AllMinistries...)
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
			// catalog.Private,
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

	// set up the output directory
	if err := cmd.initializeOutputDir(); err != nil {
		return err
	}

	// load our templates
	if err := cmd.loadTemplates(); err != nil {
		return fmt.Errorf("unable to load templates for generating the catalog: %w", err)
	}

	// set up the static files
	if err := cmd.copyStaticFilesToOutputDir(ministries); err != nil {
		return err
	}

	// generate ministry styles
	log.Printf("Generating style sheets")
	for _, ministry := range ministries {
		log.Printf("  Ministry %s", ministry.Description())
		if err := cmd.createStyleSheet(ministry); err != nil {
			return err
		}
	}

	// generate the seri pages
	log.Printf("Generating seri pages")
	for _, ministry := range ministries {
		for _, view := range views {
			log.Printf("  Ministry %s (%s)", ministry.Description(), string(view))
			if err := cmd.createAllCatalogSeriPages(ministry, view); err != nil {
				return err
			}
		}
	}

	// generate the series pages
	log.Printf("Generating series pages")
	for _, ministry := range ministries {
		for _, view := range views {
			log.Printf("  Ministry %s (%s)", ministry.Description(), string(view))
			if err := cmd.createAllCatalogSeriesPages(ministry, view); err != nil {
				return err
			}
		}
	}

	// generate recent messages
	log.Printf("Generating recent message pages")
	for _, ministry := range ministries {
		log.Printf("  Ministry %s", ministry.Description())
		if err := cmd.createRecentMessagePage(ministry); err != nil {
			return err
		}
	}

	{
		log.Printf("Creating booklet reference page")
		if err := cmd.createBookletPage(catalog.WordOfLife); err != nil {
			return err
		}
	}

	{
		log.Printf("Creating resource reference page")
		if err := cmd.createResourcePage(catalog.WordOfLife); err != nil {
			return err
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

	// create the template
	cmd.template = template.New("catalog")
	cmd.template.Funcs(template.FuncMap{
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("dict call must contain an even number of parameters")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict key %d is not a string: (%v)", i, values[i])
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"GetCatalogFileNameForSeriList": GetCatalogFileNameForSeriList,
	})

	// parse the templates
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
	// find the source directory containing static files
	templateDir, err := getTemplateDir()
	if err != nil {
		return err
	}
	sourceDir := filepath.Join(templateDir, "static")
	if !util.IsDirectory(sourceDir) {
		return fmt.Errorf("cannot find static template directory %s", sourceDir)
	}

	// find the target directory for static files
	targetDir := filepath.Join(cmd.OutputDir, "static")

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
	return copy.Copy(sourceDir, targetDir, opt)
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

	data := struct {
		Ministry catalog.Ministry
	}{
		Ministry: ministry,
	}

	return cmd.template.ExecuteTemplate(output, "catalog.css", data)
}

// ----------------------------------------------------------------------------
// | Pages containing a list of series
// ----------------------------------------------------------------------------

const (
	ALPHABETICAL_ASC   string = "az"
	ALPHABETICAL_DESC  string = "za"
	CHRONOLOGICAL_ASC  string = "09"
	CHRONOLOGICAL_DESC string = "90"
)

// createAllCatalogSeriesPages generates all the series list pages for a specific ministry and
// view. The name of each page will be generated by generateSeriesListPageName()
func (cmd *catalogCmdStruct) createAllCatalogSeriesPages(ministry catalog.Ministry, view catalog.View) error {
	var seriList []catalog.CatalogSeri
	switch ministry {
	case catalog.WordOfLife:
		// 2022-12: Pastor wants ATP messages on both ATP and WOL pages
		seriList = cmd.cat.FindSeriesByMinistry(catalog.WordOfLife, catalog.AskThePastor)
	default:
		seriList = cmd.cat.FindSeriesByMinistry(ministry)
	}
	seriList = catalog.FilterSeriesByView(seriList, view)

	if len(seriList) == 0 {
		log.Printf("    (no series found)")
	}

	for _, order := range []string{ALPHABETICAL_ASC, CHRONOLOGICAL_ASC, CHRONOLOGICAL_DESC} {
		filePath := cmd.getOutputFilePath(GetCatalogFileNameForSeriList(ministry, view, order))
		log.Printf("    series for (%s,%s,%s) --> %s", ministry, view, "series name", filePath)

		// sort
		switch order {
		case ALPHABETICAL_ASC:
			sort.Sort(catalog.SortSeriByName(seriList))
		case CHRONOLOGICAL_ASC:
			sort.Sort(catalog.SortSeriOldestToNewest(seriList))
		case CHRONOLOGICAL_DESC:
			sort.Sort(catalog.SortSeriNewestToOldest(seriList))
		}

		// create the file name
		f, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("cannot create output file %s: %w", filePath, err)
		}
		defer f.Close()

		// write the series list
		err = cmd.printCatalogSeriList(ministry, view, order, seriList, f)
		if err != nil {
			return fmt.Errorf("cannot print series list to %s: %w", filePath, err)
		}
	}

	return nil
}

// GetCatalogFileNameForSeriList generates the name of the HTML file that should be used for a
// page that contains a list of seri. The name will incorporate the identifies passed in as
// parameters plus a hash to make the page name harder to guess.
func GetCatalogFileNameForSeriList(ministry catalog.Ministry, view catalog.View, sortingOrder string) string {
	nameBase := string(ministry) + "-" + string(view) + "-" + sortingOrder
	return "catalog." + nameBase + "-" + util.ComputeHash(nameBase) + ".html"
}

// printCatalogSeriList prints the catolog page for a list of series to the output writer
func (cmd *catalogCmdStruct) printCatalogSeriList(
	ministry catalog.Ministry,
	view catalog.View,
	order string,
	series []catalog.CatalogSeri,
	output io.Writer,
) error {
	if err := cmd.loadTemplates(); err != nil {
		return err
	}

	data := struct {
		Date     catalog.DateOnly
		Ministry catalog.Ministry
		View     catalog.View
		Order    string
		Series   []catalog.CatalogSeri
	}{
		Date:     catalog.NewDateToday(),
		Ministry: ministry,
		View:     view,
		Order:    order,
		Series:   series,
	}

	return cmd.template.ExecuteTemplate(output, "catalog.series.html", data)
}

// ----------------------------------------------------------------------------
// | Pages containing a single series
// ----------------------------------------------------------------------------

// createAllCatalogSeriPages generates all the single series pages for a specific ministry and
// view. The name of each page will be the series ID for public pages and a mystic hash if the
// view is not public, this just makes the private pages harder to guess
func (cmd *catalogCmdStruct) createAllCatalogSeriPages(ministry catalog.Ministry, view catalog.View) error {
	var seriList []catalog.CatalogSeri
	switch ministry {
	case catalog.WordOfLife:
		// 2022-12: Pastor wants ATP messages on both ATP and WOL pages
		seriList = cmd.cat.FindSeriesByMinistry(catalog.WordOfLife, catalog.AskThePastor)
	default:
		seriList = cmd.cat.FindSeriesByMinistry(ministry)
	}
	seriList = catalog.FilterSeriesByView(seriList, view)

	if len(seriList) == 0 {
		log.Printf("    (no series found)")
	}

	for _, seri := range seriList {
		if err := cmd.createCatalogSeriPage(&seri); err != nil {
			return err
		}
	}

	return nil
}

// createCatalogSeri creates a catalog page for a single series to a file in the output
// directory. The name of the page will be the series ID (+ .html)
func (cmd *catalogCmdStruct) createCatalogSeriPage(seri *catalog.CatalogSeri) error {
	// file name is just the View ID
	filePath := cmd.getOutputFilePath(seri.GetCatalogFileName(seri.View))
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

// ----------------------------------------------------------------------------
// | Pages containing recent messages
// ----------------------------------------------------------------------------

// createRecentMessagePage a page that contains recent messages.
func (cmd *catalogCmdStruct) createRecentMessagePage(ministry catalog.Ministry) error {
	cutoff := time.Now().AddDate(0, 0, -1*cmd.Days)

	// get recent messages for this ministry
	messages := []catalog.CatalogMessage{}
	for index := range cmd.cat.Messages {
		msg := cmd.cat.Messages[index]
		if msg.Ministry != ministry ||
			msg.Visibility != catalog.Public ||
			msg.Date.Before(cutoff) {
			continue
		}

		messages = append(messages, msg.Copy())
	}
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Date.Time.After(messages[j].Date.Time)
	})
	for index := range messages {
		messages[index].Series = []catalog.SeriesReference{
			{Name: "Recent Messages", Index: index + 1},
		}
	}

	// figure out the file name
	filePath := cmd.getOutputFilePath(fmt.Sprintf("catalog.%s-recent.html", string(ministry)))
	log.Printf("    recent messages for %s --> %s", ministry, filePath)

	// create the file name
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("cannot create output file %s: %w", filePath, err)
	}
	defer f.Close()

	// write the series list
	err = cmd.printRecentMessages(ministry, messages, f)
	if err != nil {
		return fmt.Errorf("cannot print series list to %s: %w", filePath, err)
	}

	return nil
}

// printRecentMessages Creates a page with the specified messages. This creates a fake series
// containing the messages and prints the seri page with the messages
func (cmd *catalogCmdStruct) printRecentMessages(ministry catalog.Ministry, messages []catalog.CatalogMessage, output io.Writer) error {
	// create the series
	seri := catalog.CatalogSeri{
		Name:     fmt.Sprintf("Recent messages from %s", ministry.Description()),
		Messages: messages,
	}
	seri.Initialize()
	seri.Normalize()

	data := struct {
		Date     catalog.DateOnly
		Ministry catalog.Ministry
		Seri     *catalog.CatalogSeri
	}{
		Date:     catalog.NewDateToday(),
		Ministry: ministry,
		Seri:     &seri,
	}

	return cmd.template.ExecuteTemplate(output, "catalog.seri.html", data)
}

// ----------------------------------------------------------------------------
// | Pages containing online resources
// ----------------------------------------------------------------------------

// createBookletPage creates a page that lists all the booklets that we have, whether they are
// attached to a series or not.
func (cmd *catalogCmdStruct) createBookletPage(ministry catalog.Ministry) error {
	// get the list of resources
	resources := []catalog.OnlineResource{}

	// find all the appropriate series
	seriList := []catalog.CatalogSeri{}
	for _, seri := range cmd.cat.Series {
		if (seri.IsBooklet() || seri.GetMinistry() == ministry) &&
			catalog.IsVisibleInView(seri.Visibility, catalog.Public) {
			seriList = append(seriList, seri)
		}
	}

	// extract all the booklets
	for _, seri := range seriList {
		for _, booklet := range seri.Booklets {
			if !seri.IsBooklet() {
				copy := seri.Copy()
				booklet.Seri = &copy
			}
			resources = append(resources, booklet)
		}
	}

	// sort by name
	sort.Slice(resources, func(i, j int) bool { return resources[i].Name < resources[j].Name })

	// file name is just the View ID
	filePath := cmd.getOutputFilePath(fmt.Sprintf("catalog.%s-booklets.html", string(ministry)))
	log.Printf("    %s --> %s", string(ministry), filePath)

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("cannot create output file %s: %w", filePath, err)
	}
	defer f.Close()

	return cmd.printOnlineResources("booklet", ministry, resources, f)
}

// createResourcePage creates a page that lists all the resources that we have associated with
// public messages
func (cmd *catalogCmdStruct) createResourcePage(ministry catalog.Ministry) error {
	// get the list of resources
	resources := []catalog.OnlineResource{}

	// find all the appropriate series
	seriList := cmd.cat.FindSeriesByMinistry(ministry)
	seriList = catalog.FilterSeriesByView(seriList, catalog.Public)

	// extract all the booklets
	for _, seri := range seriList {
		for _, message := range seri.Messages {
			for _, resource := range message.Resources {
				seriCopy := seri.Copy()
				resource.Seri = &seriCopy

				msgCopy := message.Copy()
				resource.Message = &msgCopy

				resources = append(resources, resource)
			}
		}
	}

	// sort by name
	sort.Slice(resources, func(i, j int) bool { return resources[i].Name < resources[j].Name })

	// file name is just the View ID
	filePath := cmd.getOutputFilePath(fmt.Sprintf("catalog.%s-resources.html", string(ministry)))
	log.Printf("    %s --> %s", string(ministry), filePath)

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("cannot create output file %s: %w", filePath, err)
	}
	defer f.Close()

	return cmd.printOnlineResources("online", ministry, resources, f)
}

func (cmd *catalogCmdStruct) printOnlineResources(
	resourceType string, // "booklet" or "online"
	ministry catalog.Ministry,
	resources []catalog.OnlineResource,
	output io.Writer,
) error {
	// generate the template
	var title string
	switch resourceType {
	case "booklet":
		title = ministry.Description() + " Booklets"
	case "online":
		title = ministry.Description() + " Online Resources"
	default:
		title = ministry.Description() + " Resources"
	}

	data := struct {
		Title     string
		Type      string
		Date      catalog.DateOnly
		Ministry  catalog.Ministry
		Resources []catalog.OnlineResource
	}{
		Title:     title,
		Type:      resourceType,
		Date:      catalog.NewDateToday(),
		Ministry:  ministry,
		Resources: resources,
	}

	return cmd.template.ExecuteTemplate(output, "catalog.resources.html", data)
}
