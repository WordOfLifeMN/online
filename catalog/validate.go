package catalog

// Contains code for validating a catalog, including series and messages

import (
	"fmt"
	"sort"
	"strings"

	"github.com/WordOfLifeMN/online/util"
)

// List of states that audio and video's can be in. Most of these should be temporary states
// during editing, but some are permanent
var possibleAudioVideoStates = []string{
	"-",           // permanent - doesn't exist
	"n/a",         // permanent - not available
	"n/e",         // permanent - not edited
	"abrogated",   // permanent
	"in progress", // temporary
	"exporting",   // temporary
	"exported",    // temporary
	"editing",     // temporary
	"edited",      // temporary
	"rendering",   // temporary
	"rendered",    // temporary
	"uploading",   // temporary
}

// +---------------------------------------------------------------------------
// | Catalog validation
// +---------------------------------------------------------------------------

// Validates that all aspects of the catalog are valid. Returns true if the
// entire catalog is valid. Returns false and outputs to stderr if any problems
// are found. Note that just because the catalog is not valid does not mean it
// cannot be used.
//
// If reporting is loud then warnings will be sent to stderr, otherwise they are
// logged
//
// Validations:
//  - All series referenced in messages exist
//  - Series track indexes start with 1 and are sequential
//  - Series names are unique
//  - Message names are unique
//  - Message names not in a series are unique wrt Series names
func (c *Catalog) IsValid(reportLoud bool) bool {
	var report *util.IndentingReport
	if reportLoud {
		report = util.NewIndentingReport(util.ReportErr)
	} else {
		report = util.NewIndentingReport(util.ReportLog)
	}

	valid := true

	// validate series
	for _, seri := range c.Series {
		valid = seri.IsValid(report) && valid
	}

	// validate messages
	for _, msg := range c.Messages {
		valid = msg.IsValid(report) && valid
	}

	// validate seris and messages wrt each other
	valid = c.IsMessageSeriesValid(report) && valid
	valid = c.IsMessageSeriesIndexValid(report) && valid
	valid = c.IsSeriesNamesValid(report) && valid
	// valid = c.IsMessageNamesValid(report) && valid
	// valid = c.IsSeriesAndMessageNamesValid(report) && valid

	return valid
}

// Validates that all the series referenced by messages actually exist in the
// series records. Any problems will be output to stderr
func (c *Catalog) IsMessageSeriesValid(report *util.IndentingReport) bool {
	report.StartSection("Series Reference Checks")
	defer report.StopSection()

	valid := true

	for _, msg := range c.Messages {
		for _, ref := range msg.Series {

			// ignore references to "stand-alone-messages"
			if ref.IsStandAloneMessage() {
				continue
			}

			if _, ok := c.FindSeriByName(ref.Name); !ok {
				valid = false
				report.Printf("Message '%s' references series named '%s' which cannot be found\n",
					msg.Name, ref.Name)
			}
		}
	}

	return valid
}

// Validates that all the series referenced by messages have consistent track
// indexes. Checks for duplicate or skipped track numbers. Allows any number of
// track "0" though (which flags a message as part of a series that should not
// be displayed)
func (c *Catalog) IsMessageSeriesIndexValid(report *util.IndentingReport) bool {
	report.StartSection("Series Index Checks")
	defer report.StopSection()

	valid := true

	// check each series
	for _, seri := range c.Series {
		msgs := c.FindMessagesInSeries(seri.Name)

		for index := 0; index < len(msgs)-1; index++ {
			seriesIndex1 := msgs[index].Series[0].Index
			seriesIndex2 := msgs[index+1].Series[0].Index
			if seriesIndex2 == 0 {
				// quit when we encounter the 0's
				break
			}
			if index == 0 && seriesIndex1 != 1 {
				valid = false
				report.Printf("Series '%s' first message '%s' has index %d\n",
					seri.Name, msgs[index].Name, seriesIndex1)
			}
			if seriesIndex1 == seriesIndex2 {
				valid = false
				report.Printf("Series '%s' has at least two messages with index %d: '%s' and '%s'\n",
					seri.Name, seriesIndex1, msgs[index].Name, msgs[index+1].Name)
			} else if seriesIndex2 > seriesIndex1+1 {
				valid = false
				report.Printf("Series '%s' has a gap between indexes %d ('%s') and %d ('%s')\n",
					seri.Name, seriesIndex1, msgs[index].Name, seriesIndex2, msgs[index+1].Name)
			}
		}
	}

	return valid
}

// Verifies that all series names are unique
func (c *Catalog) IsSeriesNamesValid(report *util.IndentingReport) bool {
	report.StartSection("Series Name Checks")
	defer report.StopSection()

	valid := true

	// extract series names
	names := make([]string, len(c.Series))
	for index, seri := range c.Series {
		names[index] = seri.Name
	}
	sort.Strings(names)

	// look for duplicates
	for index := 0; index < len(names)-1; index++ {
		if names[index] == names[index+1] {
			valid = false
			report.Printf("There are multiple series with the name '%s'", names[index])
		}
	}
	return valid
}

// // Verifies that all message names are unique within a ministry
// func (c *Catalog) IsMessageNamesValid(report *ValidationReport) bool {
// report.StartSection("Message Name Checks")
// defer report.StopSection()

// 	valid := true

// 	// extract message names
// 	names := make([]string, len(c.Messages))
// 	for index, msg := range c.Messages {
// 		names[index] = string(msg.Ministry) + ":" + strings.ToLower(msg.Name)
// 	}
// 	sort.Strings(names)

// 	// look for duplicates
// 	for index := 0; index < len(names)-1; index++ {
// 		if names[index] == names[index+1] {
// 			valid = false
// 			report.Printf("There are multiple messages with the name '%s'\n", names[index])
// 		}
// 	}
// 	return valid
// }

// // Verifies that all messages (that are not in a series) have names that are unique
// func (c *Catalog) IsSeriesAndMessageNamesValid(report *ValidationReport) bool {
// report.StartSection("Series and Mesasge Name Checks")
// defer report.StopSection()

// 	valid := true

// 	// extract series and message names
// 	names := []string{}
// 	for _, seri := range c.Series {
// 		names = append(names, seri.Name)
// 	}
// 	for _, msg := range c.Messages {
// 		if len(msg.Series) > 0 {
// 			continue
// 		}
// 		names = append(names, msg.Name)
// 	}
// 	sort.Strings(names)

// 	// look for duplicates
// 	for index := 0; index < len(names)-1; index++ {
// 		if names[index] == names[index+1] {
// 			valid = false
// 			report.Printf("Message name '%s' conflicts with another message with the same name\n", names[index])
// 		}
// 	}

// 	return valid
// }

// +---------------------------------------------------------------------------
// | Series validation
// +---------------------------------------------------------------------------

// IsValid checks if this series has valid values in it's fields
func (s *CatalogSeri) IsValid(report *util.IndentingReport) bool {
	report.StartSection(fmt.Sprintf("Checking series %s", s.Name))
	defer report.StopSection()

	valid := true

	// name
	if s.Name == "" {
		report.Printf("Has no name")
		valid = false
	}

	// id
	if s.ID == "" && !s.IsBooklet() && (s.Visibility == Public || s.Visibility == Partner) {
		report.Printf("Has no ID (and is not a booklet)")
		valid = false
	}

	// booklets
	for _, booklet := range s.Booklets {
		valid = booklet.IsValid(report) && valid
	}

	return valid
}

// +---------------------------------------------------------------------------
// | Message validation
// +---------------------------------------------------------------------------

// IsValid checks if this message has valid values in it's fields
func (m *CatalogMessage) IsValid(report *util.IndentingReport) bool {
	report.StartSection(fmt.Sprintf("Checking message %s - %s", m.Date.String(), m.Name))
	defer report.StopSection()

	valid := true

	// date
	if m.Date.IsZero() {
		report.Printf("Has no date")
		valid = false
	}

	// name
	if m.Name == "" {
		report.Printf("Has no name")
		valid = false
	}

	// ministry
	if m.Ministry == "" {
		report.Printf("No ministry")
		valid = false
	}
	if m.Ministry == UnknownMinistry {
		report.Printf("Unknown ministry '%s'", string(m.Ministry))
		valid = false
	}

	// visibility
	if m.Visibility == "" {
		report.Printf("No visibility")
		valid = false
	}
	if m.Visibility == UnknownView {
		report.Printf("Unknown visibility '%s'", string(m.Visibility))
		valid = false
	}

	// type
	if m.Visibility != Raw && m.Visibility != Private {
		if m.Type == "" {
			report.Printf("No type")
			valid = false
		}
		if m.Type == UnknownType {
			report.Printf("Unknown type '%s'", string(m.Type))
			valid = false
		}
	}

	// audio
	if m.Audio != nil && !strings.Contains(m.Audio.URL, "://") {
		found := false
		for _, expected := range possibleAudioVideoStates {
			if m.Audio.URL == expected {
				found = true
				break
			}
		}
		if !found {
			report.Printf("Audio '%s' isn't valid. It is neither a URL nor one of the expected values %v", m.Audio.URL, possibleAudioVideoStates)
			valid = false
		}
	}

	// video
	if m.Video != nil && !strings.Contains(m.Video.URL, "://") {
		found := false
		for _, expected := range possibleAudioVideoStates {
			if m.Video.URL == expected {
				found = true
				break
			}
		}
		if !found {
			report.Printf("Video '%s' isn't valid. It is neither a URL nor one of the expected values %v", m.Video.URL, possibleAudioVideoStates)
			valid = false
		}
	}

	// resources
	for _, resource := range m.Resources {
		valid = resource.IsValid(report) && valid
	}

	return valid
}

// +---------------------------------------------------------------------------
// | Resource validation
// +---------------------------------------------------------------------------

func (r *OnlineResource) IsValid(report *util.IndentingReport) bool {
	if !strings.Contains(r.URL, "://") {
		report.Printf("Resource '%s' (%s) does not contain a valid URL", r.Name, r.URL)
		return false
	}

	return true
}
