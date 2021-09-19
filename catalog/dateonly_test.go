package catalog

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	testDate, err := time.Parse("2006-01-02T15:04:05Z", "2010-09-08T07:06:05Z")
	assert.NoError(t, err)

	sut := NewDateOnly(testDate)

	assert.Equal(t, 2010, sut.Time.Year())
	assert.Equal(t, time.September, sut.Time.Month())
	assert.Equal(t, 8, sut.Time.Day())
	assert.Equal(t, 0, sut.Hour())
	assert.Equal(t, 0, sut.Minute())
	assert.Equal(t, 0, sut.Second())
}

func TestParse(t *testing.T) {
	sut, err := ParseDateOnly("2010-09-08")
	assert.NoError(t, err)

	assert.Equal(t, 2010, sut.Time.Year())
	assert.Equal(t, time.September, sut.Time.Month())
	assert.Equal(t, 8, sut.Time.Day())
	assert.Equal(t, 0, sut.Hour())
	assert.Equal(t, 0, sut.Minute())
	assert.Equal(t, 0, sut.Second())
}

func TestMustParse(t *testing.T) {
	sut := MustParseDateOnly("2011-10-09")

	assert.Equal(t, 2011, sut.Time.Year())
	assert.Equal(t, time.October, sut.Time.Month())
	assert.Equal(t, 9, sut.Time.Day())
	assert.Equal(t, 0, sut.Hour())
	assert.Equal(t, 0, sut.Minute())
	assert.Equal(t, 0, sut.Second())
}

func TestString(t *testing.T) {
	sut, err := ParseDateOnly("2010-09-08")
	assert.NoError(t, err)

	assert.Equal(t, "2010-09-08", sut.String())
}

func TestMarshalNullDate(t *testing.T) {
	s := struct {
		Date DateOnly `json:"date"`
	}{}
	t.Logf("Marshalled data structure is: %+v", s)

	bytes, err := json.Marshal(s)
	assert.NoError(t, err)

	assert.Equal(t, `{"date":null}`, string(bytes))
}

func TestMarshalDate(t *testing.T) {
	d, err := ParseDateOnly("2010-09-08")
	assert.NoError(t, err)
	s := struct {
		Date DateOnly `json:"date"`
	}{
		Date: d,
	}
	t.Logf("Marshalled data structure is: %+v", s)

	bytes, err := json.Marshal(s)
	assert.NoError(t, err)

	assert.Equal(t, `{"date":"2010-09-08"}`, string(bytes))
}

func TestUnmarshalNullDate(t *testing.T) {
	j := `{"date":null}`

	s := struct {
		Date DateOnly `json:"date"`
	}{}

	err := json.Unmarshal([]byte(j), &s)
	assert.NoError(t, err)

	assert.Equal(t, NewDateOnly(time.Time{}), s.Date)
}

func TestUnmarshalDate(t *testing.T) {
	j := `{"date":"2002-03-04"}`

	s := struct {
		Date DateOnly `json:"date"`
	}{}

	err := json.Unmarshal([]byte(j), &s)
	assert.NoError(t, err)

	assert.Equal(t, 2002, s.Date.Year())
	assert.Equal(t, time.March, s.Date.Month())
	assert.Equal(t, 4, s.Date.Day())
}

func TestZeroDate(t *testing.T) {
	sut := DateOnly{}

	assert.True(t, sut.IsZero())
}
