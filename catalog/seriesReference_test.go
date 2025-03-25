package catalog

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestOnlineSeriesReferenceSuite(t *testing.T) {
	suite.Run(t, new(OnlineSeriesReferenceTestSuite))
}

type OnlineSeriesReferenceTestSuite struct {
	suite.Suite
}

// Runs the test suite as a test
func (t *OnlineSeriesReferenceTestSuite) TestEmptyString() {
	t.Empty(nil, NewSeriesReferencesFromStrings("", ""))
	t.Empty(nil, NewSeriesReferencesFromStrings("", "1"))
	t.Empty(nil, NewSeriesReferencesFromStrings("", "1; 2"))
}

func (t *OnlineSeriesReferenceTestSuite) TestFromString() {
	s := NewSeriesReferencesFromStrings("x", "1")
	t.Len(s, 1)
	t.Equal(SeriesReference{"x", 1}, s[0])

	s = NewSeriesReferencesFromStrings("x", "1; 2")
	t.Len(s, 1)
	t.Equal(SeriesReference{"x", 1}, s[0])

	s = NewSeriesReferencesFromStrings("x;y", "1; 2")
	t.Len(s, 2)
	t.Equal(SeriesReference{"x", 1}, s[0])
	t.Equal(SeriesReference{"y", 2}, s[1])

	s = NewSeriesReferencesFromStrings("x;y;z", "1; 2")
	t.Len(s, 3)
	t.Equal(SeriesReference{"x", 1}, s[0])
	t.Equal(SeriesReference{"y", 2}, s[1])
	t.Equal(SeriesReference{"z", 2}, s[2])
}
