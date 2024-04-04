package util

import (
	"encoding/json"
	"fmt"
)

// JSONString returns a JSON string representation of the given data. This is intended for use
// in debugging easily (e.g. `log.Debugf("received: %s", to.JSONString(body))`), and so returns
// one string. Since this has no errors, we make efforts to return reasonable values that are
// always legal JSON strings:
// - If input is nil
// {}
// - If data is an error, a string representation of the error
// "error message"
// - If data cannot be marshalled, a "%#v" string representation of the data
// "main.T{b:true, i:42, s:[]string{\"life\", \"universe\", \"everything\"}}"
// - Otherwise, the JSON representation of the data (no private fields, etc)
func ToJSON(data any) string {
	if data == nil {
		return "{}"
	}

	_, isError := data.(error)
	if isError {
		return fmt.Sprintf("%q", data.(error).Error())
	}

	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("%q", fmt.Sprintf(`%#v`, data))
	}
	return string(b)
}
