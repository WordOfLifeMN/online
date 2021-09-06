package catalog

// Time field that only stores the date (time components are 0). The marshalled
// format of the date is a string with the format "yyyy-mm-dd"

import (
	"encoding/json"
	"strings"
	"time"
)

type DateOnly struct {
	time.Time
}

const dateLayout = "2006-01-02"

func NewDateOnly(t time.Time) DateOnly {
	d := DateOnly{}
	d.Time = t.Truncate(24 * time.Hour)
	return d
}

func ParseDateOnly(t string) (d DateOnly, err error) {
	d = DateOnly{}
	d.Time, err = time.Parse(dateLayout, t)
	return
}

func MustParseDateOnly(t string) DateOnly {
	d, err := ParseDateOnly(t)
	if err != nil {
		panic(err)
	}
	return d
}

func (d *DateOnly) String() string {
	return d.Time.Format(dateLayout)
}

// JSON support
var nilTime = (time.Time{}).UnixNano()

func (d DateOnly) MarshalJSON() ([]byte, error) {
	if d.Time.UnixNano() == nilTime {
		return []byte("null"), nil
	}
	return json.Marshal(d.String())
}

func (d *DateOnly) UnmarshalJSON(b []byte) (err error) {
	s := strings.Trim(string(b), "\"")
	if s == "null" {
		d.Time = time.Time{}
		return
	}
	d.Time, err = time.Parse(dateLayout, s)
	return
}
