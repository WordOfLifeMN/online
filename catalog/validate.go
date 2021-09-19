package catalog

// Contains code for validating a catalog, including series and messages

import (
	"fmt"
	"log"
	"os"
	"sort"
)

// +---------------------------------------------------------------------------
// | reorting
// +---------------------------------------------------------------------------
var isLoud bool = false

// Prints a validation problem to the appropriate output channel
func (c *Catalog) printProblem(format string, a ...interface{}) {
	if isLoud {
		fmt.Fprintf(os.Stderr, format, a...)
	} else {
		log.Printf(format, a...)
	}
}

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
	isLoud = reportLoud

	valid := true

	// TODO - add validation for series and messages individually
	// TODO - messages should validate audio/video is URL or in progress/exporting/rendering/etc

	valid = c.IsMessageSeriesValid() && valid
	valid = c.IsMessageSeriesIndexValid() && valid
	valid = c.IsSeriesNamesValid() && valid
	// valid = c.IsMessageNamesValid() && valid
	// valid = c.IsSeriesAndMessageNamesValid() && valid

	return valid
}

// Validates that all the series referenced by messages actually exist in the
// series records. Any problems will be output to stderr
func (c *Catalog) IsMessageSeriesValid() bool {
	valid := true

	for _, msg := range c.Messages {
		for _, ref := range msg.Series {

			// ignore references to "stand-alone-messages"
			if ref.IsStandAloneMessage() {
				continue
			}

			if _, ok := c.FindSeries(ref.Name); !ok {
				valid = false
				c.printProblem("Message '%s' references series named '%s' which cannot be found\n",
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
func (c *Catalog) IsMessageSeriesIndexValid() bool {
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
				c.printProblem("Series '%s' first message '%s' has index %d\n",
					seri.Name, msgs[index].Name, seriesIndex1)
			}
			if seriesIndex1 == seriesIndex2 {
				valid = false
				c.printProblem("Series '%s' has at least two messages with index %d: '%s' and '%s'\n",
					seri.Name, seriesIndex1, msgs[index].Name, msgs[index+1].Name)
			} else if seriesIndex2 > seriesIndex1+1 {
				valid = false
				c.printProblem("Series '%s' has a gap between indexes %d ('%s') and %d ('%s')\n",
					seri.Name, seriesIndex1, msgs[index].Name, seriesIndex2, msgs[index+1].Name)
			}
		}
	}

	return valid
}

// Verifies that all series names are unique
func (c *Catalog) IsSeriesNamesValid() bool {
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
			c.printProblem("There are multiple series with the name '%s'", names[index])
		}
	}
	return valid
}

// // Verifies that all message names are unique within a ministry
// func (c *Catalog) IsMessageNamesValid() bool {
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
// 			c.printProblem("There are multiple messages with the name '%s'\n", names[index])
// 		}
// 	}
// 	return valid
// }

// // Verifies that all messages (that are not in a series) have names that are unique
// func (c *Catalog) IsSeriesAndMessageNamesValid() bool {
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
// 			c.printProblem("Message name '%s' conflicts with another message with the same name\n", names[index])
// 		}
// 	}

// 	return valid
// }
