package util

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
)

// IndentingReport manages printing out a report with indents. Every time you start a section,
// the indent will be increased, and when you end a section the indent will be decreased.
// Sections titles are not printed unless there is something under them. So you can start a
// section whenever you want, defer the end of the section, and if nothing gets printed in the
// meantime, then nothing about the section will ever be printed
type IndentingReport struct {
	Level          ReportLevel  // true to print to stderr, false to log
	Size           int          // total number of report lines printed (not including section headers)
	depth          int          // number of headers deep we are reporting on
	pendingHeaders []string     // headers that are waiting to be printed
	report         bytes.Buffer // saves the report as a string
}

type ReportLevel string

const (
	ReportSilent ReportLevel = "silent" // do not write the report anywhere (get it later with String())
	ReportLog    ReportLevel = "log"    // write the report to the log
	ReportErr    ReportLevel = "err"    // write the report to stderr
	ReportOut    ReportLevel = "out"    // write the report to stdout
)

func NewIndentingReport(level ReportLevel) *IndentingReport {
	return &IndentingReport{
		Level: level,
	}
}

// Prints a report line to the appropriate output channel
func (r *IndentingReport) Printf(format string, a ...interface{}) {
	// if we have pending section titles, print and indent appropriately
	if len(r.pendingHeaders) > 0 {
		for _, title := range r.pendingHeaders {
			r.println(title + ":")
			r.depth++
		}
		r.pendingHeaders = []string{}
	}

	// generate the output string
	r.println(fmt.Sprintf(format, a...))
	r.Size++
}

func (r *IndentingReport) println(s string) {
	// build the indent prefix as a whitespace proportional to the depth
	s = strings.Repeat("   ", r.depth) + s

	// update internal cache
	r.report.Write([]byte(s + "\n"))

	// print the string to the correct location
	switch r.Level {
	case ReportLog:
		log.Println(s)
	case ReportErr:
		fmt.Fprintln(os.Stderr, s)
	case ReportOut:
		fmt.Fprintln(os.Stdout, s)
	default:
		// don't write anything out
	}
}

// String returns the cached report as a string
func (r *IndentingReport) String() string {
	return r.report.String()
}

// StartSection starts a new section in the report by printing a title and indenting subsequent
// output by one level. The section title is not printed unless there is actually something to
// print under it though
func (r *IndentingReport) StartSection(title string) {
	r.pendingHeaders = append(r.pendingHeaders, title)
}

// StopSection stops a section by reducing the indent on subsequent output
func (r *IndentingReport) StopSection() {
	if len(r.pendingHeaders) > 0 {
		r.pendingHeaders = r.pendingHeaders[0 : len(r.pendingHeaders)-1]
	} else {
		r.depth--
	}
}
