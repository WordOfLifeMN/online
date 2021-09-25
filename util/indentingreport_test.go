package util

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type IndentingReportTestSuite struct {
	suite.Suite
}

// Runs the test suite as a test
func TestIndentingReportTestSuite(t *testing.T) {
	suite.Run(t, new(IndentingReportTestSuite))
}

func (t *IndentingReportTestSuite) TestNoSection() {
	// capture output in a string
	sut := NewIndentingReport(ReportSilent)

	sut.Printf("one")
	sut.Printf("two")

	t.Equal("one\ntwo\n", sut.String())
	t.Equal(2, sut.Size)
}

func (t *IndentingReportTestSuite) TestOneSection() {
	// capture output in a string
	sut := NewIndentingReport(ReportSilent)

	sut.StartSection("SECT1")
	sut.Printf("one")
	sut.Printf("two")
	sut.StopSection()

	t.Equal("SECT1:\n   one\n   two\n", sut.String())
	t.Equal(2, sut.Size)
}

func (t *IndentingReportTestSuite) TestTwoSections() {
	// capture output in a string
	sut := NewIndentingReport(ReportSilent)

	sut.StartSection("SECT1")
	sut.Printf("one")
	sut.StopSection()
	sut.StartSection("SECT2")
	sut.Printf("two")
	sut.StopSection()

	t.Equal("SECT1:\n   one\nSECT2:\n   two\n", sut.String())
	t.Equal(2, sut.Size)
}

func (t *IndentingReportTestSuite) TestNestedSections() {
	// capture output in a string
	sut := NewIndentingReport(ReportSilent)

	sut.StartSection("SECT1")
	sut.Printf("one")
	sut.StartSection("SECT2")
	sut.Printf("two")
	sut.StopSection()
	sut.StopSection()

	t.Equal("SECT1:\n   one\n   SECT2:\n      two\n", sut.String())
	t.Equal(2, sut.Size)
}

func (t *IndentingReportTestSuite) TestSectionWithoutOutput() {
	// capture output in a string
	sut := NewIndentingReport(ReportSilent)

	sut.Printf("one")
	sut.StartSection("SECT1")
	sut.StopSection()
	sut.Printf("two")

	t.Equal("one\ntwo\n", sut.String())
	t.Equal(2, sut.Size)
}
