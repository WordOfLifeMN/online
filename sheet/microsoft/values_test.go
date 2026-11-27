package microsoft

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCellText(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"empty", nil, ""},
		{"text", "Grace", "Grace"},
		{"text with line break", "one\ntwo", "one\ntwo"},
		{"whole number", 1.0, "1"},
		{"big whole number", 46353.0, "46353"},
		{"negative", -3.0, "-3"},
		{"fraction", 1.5, "1.5"},
		{"float noise", 0.1 + 0.2, "0.3"},
		{"float32", float32(2), "2"},
		{"true", true, "TRUE"},
		{"false", false, "FALSE"},
		{"#N/A", cellError(2042), "#N/A"},
		{"#DIV/0!", cellError(2007), "#DIV/0!"},
		{"unknown error", cellError(9999), "#ERROR"},
		{"date", time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC), "2026-11-27"},
		{"other", int32(7), "7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, cellText(tt.in))
		})
	}
}

func TestSerialDate(t *testing.T) {
	d, ok := serialDate(46353, false)
	assert.True(t, ok)
	assert.Equal(t, "2026-11-27", d)

	d, ok = serialDate(36892, false)
	assert.True(t, ok)
	assert.Equal(t, "2001-01-01", d)

	// the 1904 date system counts from 1904-01-01
	d, ok = serialDate(44891, true)
	assert.True(t, ok)
	assert.Equal(t, "2026-11-27", d)

	_, ok = serialDate(0, false)
	assert.False(t, ok)
}

func TestFromColumnMajor(t *testing.T) {
	// a 2 x 3 range as COM stores it: column A, then B, then C
	flat := []any{"Date", 46353.0, "Name", "First", "Track", 1.0}
	assert.Equal(t, [][]any{
		{"Date", "Name", "Track"},
		{46353.0, "First", 1.0},
	}, fromColumnMajor(flat, 2, 3))
}

func TestGridToRows(t *testing.T) {
	grid := [][]any{
		{"Date", "Name", "Track", nil},
		{46353.0, "Serial date", 1.0, nil},
		{"2026-11-28", "Text date", nil, nil},
		{nil, nil, nil, nil}, // trailing empty rows, as data validation leaves them
		{nil, nil, nil, nil},
	}

	assert.Equal(t, [][]string{
		{"Date", "Name", "Track"},
		{"2026-11-27", "Serial date", "1"},
		{"2026-11-28", "Text date"},
	}, gridToRows(grid, 1, 1, false))
}

func TestGridToRows_RangeNotAtA1(t *testing.T) {
	// a used range starting at B3: rows 1-2 and column A are empty
	grid := [][]any{
		{"x", 2.0},
		{nil, "y"},
	}

	assert.Equal(t, [][]string{
		{},
		{},
		{"", "x", "2"},
		{"", "", "y"},
	}, gridToRows(grid, 3, 2, false))
}

func TestGridToRows_DateColumnNotFirst(t *testing.T) {
	grid := [][]any{
		{"Name", "Date"},
		{"First", 46353.0},
	}

	assert.Equal(t, [][]string{
		{"Name", "Date"},
		{"First", "2026-11-27"},
	}, gridToRows(grid, 1, 1, false))
}

func TestGridToRows_Empty(t *testing.T) {
	assert.Empty(t, gridToRows([][]any{{nil}}, 1, 1, false))
	assert.Empty(t, gridToRows(nil, 1, 1, false))
}
