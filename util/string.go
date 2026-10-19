package util

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
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

// matchNoise is everything punctuation-ish, flattened to a space before comparing. The
// two sides punctuate differently for reasons that have nothing to do with meaning:
// a colon is illegal in a Windows file name, and "Q&A" is written "QA" or "Q and A"
// depending on who typed it.
var matchNoise = regexp.MustCompile(`[^a-z0-9]+`)

// NormalizeForMatch reduces a title to the part worth comparing: lower case, no
// articles, no punctuation, single spaces. Applied to both sides so neither is
// penalised for a house style the other does not share.
//
// Articles go because they come and go between the sheet and a file name - "The
// Importance of Voting" is saved as "Importance of Voting" - without changing which
// message is meant. They are dropped as whole words before punctuation is flattened,
// so the "A" in "Q&A" is not mistaken for one.
func NormalizeForMatch(title string) string {
	words := strings.Fields(strings.ToLower(title))
	kept := words[:0]
	for _, word := range words {
		if !matchArticles[matchNoise.ReplaceAllString(word, "")] {
			kept = append(kept, word)
		}
	}

	title = matchNoise.ReplaceAllString(strings.Join(kept, " "), " ")
	return strings.TrimSpace(strings.Join(strings.Fields(title), " "))
}

// matchArticles are the words NormalizeForMatch leaves out
var matchArticles = map[string]bool{"the": true, "a": true, "an": true}
