package catalog

import (
	"log"
	"strconv"
	"strings"
)

// SeriesReference contains information that a message stores about the series
// it belongs to. This information may or may not be accurate or meaningful, it
// is just data without meaning until an external process decides to give it
// meaning
type SeriesReference struct {
	// name of a series. it is a warning if this series isn't defined, but not
	// an error. however, if the series doesn't exist, then this message may not
	// be displayed
	Name string `json:"name"`
	// index within a series. same as a track on a CD. the first index in a
	// series is "1".
	//
	// index "0" means it is part of the series but hidden. this is not
	// recommended as you should use the "View" to mark a message private if it
	// shouldn't be visible in a series, but sometimes a message is in multiple
	// series, and it should be private in one and public in another
	Index int `json:"index,omitempty"`
}

// NewSeriesReferencesFromStrings takes two strings and returns an array of
// series references. The input strings are semi-colon delimited and each string
// should have the same number of entries, like ("a; b; c", "1; 2; 1"). The
// number of returned references will be the number of names, and any missing
// track numbers will default to 0
func NewSeriesReferencesFromStrings(names string, tracks string) []SeriesReference {
	// parse names
	nameList := []string{}
	for _, name := range strings.Split(names, ";") {
		nameList = append(nameList, strings.TrimSpace(name))
	}

	// parse tracks
	trackList := []int{}
	for _, track := range strings.Split(tracks, ";") {
		track = strings.TrimSpace(track)
		if trackNumber, err := strconv.Atoi(track); err == nil {
			trackList = append(trackList, trackNumber)
		} else {
			if track != "" {
				log.Printf("WARNING: Encountered illegal track number '%s'", track)
			}
			trackList = append(trackList, 0)
		}
	}

	// build series list
	seriesRefs := []SeriesReference{}

	for index, name := range nameList {
		if name == "" {
			continue
		}

		// build a reference for this name
		seriesRef := SeriesReference{Name: name}
		if index < len(trackList) {
			seriesRef.Index = trackList[index]
		} else {
			seriesRef.Index = trackList[len(trackList)-1]
		}
		seriesRefs = append(seriesRefs, seriesRef)
	}

	return seriesRefs
}

// Determines if this is a special case of a reference to a message that is stand-alone
func (r *SeriesReference) IsStandAloneMessage() bool {
	return r.Name == "SAM" || r.Name == "sam"
}
