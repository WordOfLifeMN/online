package catalog

import (
	"net/url"
	"path/filepath"
	"strings"
)

// OnlineResource describes a resource (pdf, document, video, website, etc)
// online that is used as reference material for a series or message
type OnlineResource struct {
	// URL of the resource, required
	URL string `json:"url"`
	// name of the resource, optional. if undefined, then GetDisplayName() will
	// generate one from the URL
	Name string `json:"name,omitempty"`

	// cached or generated data

	// URL of small thumbnail for the resource, optional. this is generated
	// dynamically for the current context of the resource display
	thumbnail string `json:"-"`
	// short string to clarify the type of the resource. generated dynamically to help the user
	// identify what will happen when they click on it, like "video", "pdf"
	classifier string `json:"-"`
}

// NewResourceFromString creates a new OnlineResource from a string definition.
// If the input string is empty or only whitespace, returns an unititialized
// Online Resource
//
// String definitions can be in multiple formats:
//
// Raw URL: "http://blah/path+to+file.doc", in which case the name is the
// file name without the extension
//
// Markdown: "[name](url)"
//
// Wiki: "name|url"
func NewResourceFromString(s string) *OnlineResource {
	r := OnlineResource{}

	s = strings.TrimSpace(s)

	switch {
	case s == "":
		break

	case strings.Contains(s, "|"):
		// Wiki: "name|url"
		p := strings.Index(s, "|")
		r.URL = strings.TrimSpace(s[p+1:])
		r.Name = strings.TrimSpace(s[:p])

	case strings.HasPrefix(s, "[") && strings.Contains(s, "](") && strings.HasSuffix(s, ")"):
		// Markdown: "[name](url)"
		p := strings.Index(s, "](")
		r.URL = strings.TrimSpace(s[p+2 : len(s)-1])
		r.Name = strings.TrimSpace(s[1:p])

	default:
		// just a URL
		r.URL = s
		r.Name = r.GetNameFromURL()
	}

	return &r
}

// NewResourcesFromString parses a string that may contain multiple resources separated by semi-colons.
// Returns an array of resources found, in the order they were found. Empty array if nothing found
func NewResourcesFromString(s string) []OnlineResource {
	results := []OnlineResource{}

	s = strings.TrimSpace(s)

	if s == "-" || s == "n/a" {
		return results
	}

	for _, part := range strings.Split(s, ";") {
		r := NewResourceFromString(part)
		if r.URL != "" {
			results = append(results, *r)
		}
	}

	return results
}

// GetNameFromURL creates a human-readable name from the resource's URL. It does this by
// extracting the last field of the URL and trying to eliminate any URL encoding or markup
func (r *OnlineResource) GetNameFromURL() string {
	// extract the last element of the URL
	name := filepath.Base(r.URL)

	// trim the extension
	p := strings.LastIndex(name, ".")
	if p != -1 {
		name = name[:p]
	}

	// decode URL
	if human, err := url.QueryUnescape(name); err == nil {
		name = human
	}

	// convert underscores to spaces
	name = strings.ReplaceAll(name, "_", " ")

	return name
}
