package catalog

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"path/filepath"
	"strings"
)

// OnlineResource describes a resource (pdf, document, video, website, etc)
// online that is used as reference material for a series or message
type OnlineResource struct {
	// URL of the resource, required
	URL string `json:"url"`
	// name of the resource, optional. if undefined, then GetDisplayName() will generate one from the URL
	Name string `json:"name,omitempty"`
	// Metadata contains miscellaneous metadata
	Metadata map[string]string `json:"metadata,omitempty"`

	// special references. these are only used for generating independent resources that need to
	// reference back to the series or message that they came from. in every other instance,
	// these will be nil
	Seri    *CatalogSeri    `json:"-"`
	Message *CatalogMessage `json:"-"`

	// cached or generated data

	// URL of small thumbnail for the resource, optional. this is generated
	// dynamically for the current context of the resource display
	thumbnail string `json:"-"` // TODO(km) delete?
	// short string to clarify the type of the resource. generated dynamically to help the user
	// identify what will happen when they click on it, like "video", "pdf"
	classifier string `json:"-"` // TODO(km) delete?
}

// NewResourceFromString creates a new OnlineResource from a string definition. If the input
// string is empty or only whitespace, returns an unititialized Online Resource
//
// String definitions can be in multiple formats:
//  - Raw URL: "http://blah/path+to+file.doc", in which case the name is the file name without the extension
//  - Markdown: "[name](url)"
//  - Wiki: "name|url"
//  - Metadata can be included as a JSON object, like `{"iframe":"https://rumble.com/embed/vjrceb/?pub=r095p"}`.
//    This can be embedded anywhere in the string, everything from the first to last brace will be treated as
//    metadata.
func NewResourceFromString(s string) *OnlineResource {
	r := OnlineResource{}

	s, r.Metadata = extractResourceMetadata(s)
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

// extractResourceMetadata looks at the string and extracts any resource metadata. This will
// return the string without the metadata and the extracted metadata. If there are any errors
// then (s, nil) is returned and s will not be modified
//
// Example: If the input string is `https://youtu.be/999 {"id":"999"}`, then the output will
// be ("https://youtu.be/999 ", map[string]string{"id": "999"})
func extractResourceMetadata(s string) (string, map[string]string) {
	if !strings.Contains(s, "{") {
		// no metadata
		return s, nil
	}
	if !strings.Contains(s, "}") {
		// has metadata, but without closing brace, it is invalid
		return s, nil
	}

	// parse the JSON into a map of interfaces
	p1 := strings.Index(s, "{")
	p2 := strings.LastIndex(s, "}") + 1

	minterface := map[string]interface{}{}
	err := json.Unmarshal([]byte(s[p1:p2]), &minterface)
	if err != nil {
		// return the unmodified string
		return s, nil
	}

	// convert the interfaces to strings
	mstrings := map[string]string{}
	for k, v := range minterface {
		mstrings[k] = fmt.Sprintf("%v", v)
	}

	// remove the metadata from the string
	s = s[0:p1] + s[p2:]

	return s, mstrings
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

// GetFileName returns the file name of the URL
func (r *OnlineResource) GetFileName() string {
	url, err := url.Parse(r.URL)
	if err != nil {
		return r.URL
	}

	return filepath.Base(url.Path)
}

// GetThumbnail returns the path to the thumbnail to use for this file type. The thumbnail is
// bigger than the icon and can be use in place of the resource. For instance, the thumbnail is
// an image the user can click on to go to the resource (as opposed to a decorator for the link)
func (r *OnlineResource) GetThumbnail() string {
	switch {
	case strings.Contains(r.URL, "youtube"), strings.Contains(r.URL, "youtu.be"):
		return "static/all.thumbnail_youtube_light.png"
	case strings.Contains(r.URL, "rumble"):
		return "static/all.thumbnail_rumble_light.png"
	case strings.Contains(r.URL, "bitchute"):
		return "static/all.thumbnail_bitchute.png"
	}
	return "static/all.thumbnail_video.png"
}

// GetIcon returns the path to the icon for this file type. The icon is very small and is a
// suitable as a decorator for the link
func (r *OnlineResource) GetIcon() string {
	switch {
	case strings.HasSuffix(r.URL, ".pdf"):
		return "static/all.icon_pdf.png"
	case strings.HasSuffix(r.URL, ".mp3"):
		return "static/all.icon_mp3.png"
	case strings.HasSuffix(r.URL, ".wmv"):
		return "static/all.icon_wmv.png"
	case strings.HasSuffix(r.URL, ".mov"):
		return "static/all.icon_mov.png"
	case strings.HasSuffix(r.URL, ".doc"), strings.HasSuffix(r.URL, ".docx"):
		return "static/all.icon_word.png"
	case strings.Contains(r.URL, "youtube"), strings.Contains(r.URL, "youtu.be"):
		return "static/all.icon_youtube.png"
	}
	return "static/all.icon_web.png"
}

// GetClassifier returns a short description of the file type. Examples: YouTube video, PDF,
// Microsoft Word, etc
func (r *OnlineResource) GetClassifier() string {
	switch {
	case strings.HasSuffix(r.URL, ".pdf"):
		return "PDF file"
	case strings.Contains(r.URL, "youtube"), strings.Contains(r.URL, "youtu.be"):
		return "YouTube video"
	case strings.Contains(r.URL, "rumble"):
		return "Rumble video"
	case strings.Contains(r.URL, "bitchute"):
		return "BitChute video"
	}
	return "Internet link"
}

// GetEmbeddedURL returns a version of the URL used for embedding the resource in an iframe. For
// example, the format for a YouTube URL is different depending on whether it's a clickable link
// or a reference to a video to play in an iframe.
func (r *OnlineResource) GetEmbeddedURL() string {
	switch {
	case strings.Contains(r.URL, "//youtu.be/"):
		return strings.ReplaceAll(r.URL, "//youtu.be/", "//www.youtube.com/embed/")
	case strings.Contains(r.URL, "//rumble.com/"):
		// rumble videos have different IDs for embedded vs. direct links. The URL should be for
		// the direct link, but the embedded URL can be in an "iframe" metadata field
		if v, ok := r.Metadata["iframe"]; ok {
			return v
		}
		return r.URL
	}
	return r.URL
}

func (r *OnlineResource) GetEmbeddedVideo(width int) template.HTML {
	switch {
	case strings.Contains(r.URL, "//youtu.be/"):
		return template.HTML(
			fmt.Sprintf(
				`<iframe width="%dpx" src="%s" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowfullscreen></iframe>`,
				width, r.GetEmbeddedURL()),
		)
	case strings.Contains(r.URL, "rumble"):
		return template.HTML(
			fmt.Sprintf(
				`<iframe width="%dpx" src="%s" title="Rumble video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowfullscreen></iframe>`,
				width, r.GetEmbeddedURL()),
		)
	}
	return template.HTML(
		fmt.Sprintf(
			`<a href="%s" target="wolmVideo"><img src="%s" height="64px" alt="%s" /></a>`,
			r.URL, r.GetThumbnail(), r.GetClassifier()),
	)
}
