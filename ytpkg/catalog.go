package ytpkg

import (
	"fmt"
	"strings"

	"github.com/WordOfLifeMN/online/catalog"
)

// standAloneSeriesPrefix marks the synthetic one-message series the catalog makes for a
// message that is in no series (catalog.NewSeriesFromMessage). They are a convenience
// for the website and never become playlists.
const standAloneSeriesPrefix = "SAM-"

// SeriesFinder looks up a series by name. *catalog.Catalog satisfies it.
type SeriesFinder interface {
	FindSeriByName(name string) (*catalog.CatalogSeri, bool)
}

// NewMessageFromCatalog builds the package the spreadsheet says a video should have.
//
// The series used for the description's booklets is the message's first series, as it
// always has been. cat may be nil, in which case no series is found and the playlists
// come from the series names alone.
//
// Returns an error for raw footage: raw is the livestream, which is already on YouTube
// as private from the moment it streams and is never downloaded or edited. Reaching
// this code with a raw message means something is wrong upstream.
func NewMessageFromCatalog(
	msg *catalog.CatalogMessage,
	cat SeriesFinder,
	generatedTitle string,
	summary string,
	thumbnailPath string,
) (*Message, error) {
	if msg == nil {
		return nil, fmt.Errorf("cannot build an upload packet without a message")
	}

	privacy, err := PrivacyForView(msg.Visibility)
	if err != nil {
		return nil, err
	}

	var seri *catalog.CatalogSeri
	if len(msg.Series) > 0 {
		seri = FindSeries(cat, msg.Series[0].Name)
	}

	return &Message{
		Channel:              ChannelForMinistry(msg.Ministry),
		Title:                FormatTitle(msg, msg.Name),
		Alternative:          AlternativeTitle(msg, generatedTitle),
		Description:          MessageDescription(msg, seri, summary),
		Privacy:              privacy,
		CategoryID:           DefaultCategoryID,
		DefaultLanguage:      DefaultLanguage,
		DefaultAudioLanguage: DefaultLanguage,
		MadeForKids:          DefaultMadeForKids,
		ThumbnailPath:        thumbnailPath,
		Playlists:            PlaylistsForMessage(msg, cat),
	}, nil
}

// NewMessageForLive builds what an existing video should be, according to the sheet.
//
// The description is the sheet's Description column plus resource links: there is no
// generated summary for a video already on YouTube. The privacy is resolved against
// the live video's, so a consistent one is kept rather than corrected - a private
// message left private on YouTube stays private.
func NewMessageForLive(
	msg *catalog.CatalogMessage,
	cat SeriesFinder,
	live *Message,
) (*Message, error) {
	want, err := NewMessageFromCatalog(msg, cat, "", "", "")
	if err != nil {
		return nil, err
	}

	want.Privacy, err = ResolvePrivacy(msg.Visibility, live.Privacy)
	if err != nil {
		return nil, err
	}
	want.ID = live.ID
	want.ChannelID = live.ChannelID

	return want, nil
}

// NewSeriesForLive builds what an existing playlist should be, according to its series
// in the sheet. The title is the live one, since it is never synced, and the privacy
// is resolved against the live privacy, as for a video.
func NewSeriesForLive(seri *catalog.CatalogSeri, live *Series) (*Series, error) {
	want, err := NewSeriesFromCatalog(seri)
	if err != nil {
		return nil, err
	}

	want.Privacy, err = ResolvePrivacy(seri.Visibility, live.Privacy)
	if err != nil {
		return nil, fmt.Errorf("series %q: %w", seri.Name, err)
	}
	want.ID = live.ID
	want.ChannelID = live.ChannelID
	want.Title = live.Title

	return want, nil
}

// PlaylistsForMessage lists the playlists a message belongs in: one per series in which
// it has a non-zero track, at that track's position.
//
// Track 0 is ancillary to the public series, so it never puts a message in a playlist.
// Ask the Pastor messages go only into the one Ask the Pastor playlist, with no
// position, whatever series they name. Stand-alone messages get no playlist.
func PlaylistsForMessage(msg *catalog.CatalogMessage, cat SeriesFinder) []PlaylistRef {
	if msg.Ministry == catalog.AskThePastor {
		return []PlaylistRef{{Title: AskThePastorPlaylistTitle()}}
	}

	var playlists []PlaylistRef
	seen := map[string]bool{}

	for _, ref := range msg.Series {
		if ref.Index <= 0 || ref.IsStandAloneMessage() {
			continue
		}

		name := ref.Name
		if seri := FindSeries(cat, ref.Name); seri != nil {
			// the sheet's own spelling of the series, which the reference may not match
			// in case
			name = seri.Name
		}

		title := PlaylistTitle(msg.Ministry, name)
		if seen[title] {
			continue
		}
		seen[title] = true
		playlists = append(playlists, PlaylistRef{Title: title, Position: ref.Index})
	}

	return playlists
}

// FindSeries looks up the series a message's series reference names, tolerating a
// missing finder. Returns nil when the sheet defines no such series.
//
// A stand-alone series is never the answer. The catalog makes one for each message in
// no series, named after the message, so a message titled "Voting" yields a series
// "Voting" that a reference to a real series of that name, with no Series row of its
// own, would otherwise find. A message that really is stand-alone has no series
// reference to look up, or names "SAM", which is handled before any lookup.
func FindSeries(cat SeriesFinder, name string) *catalog.CatalogSeri {
	if cat == nil || name == "" {
		return nil
	}
	seri, ok := cat.FindSeriByName(name)
	if !ok || strings.HasPrefix(seri.ID, standAloneSeriesPrefix) {
		// the message names a series the catalog doesn't define. that is a catalog
		// consistency problem for 'check' to report, not a reason to fail publishing.
		return nil
	}
	return seri
}

// NewSeriesFromCatalog builds the package the spreadsheet says a series' playlist should
// have. Its title is the key the playlist is found by, since the sheet stores no
// playlist IDs.
func NewSeriesFromCatalog(seri *catalog.CatalogSeri) (*Series, error) {
	if seri == nil {
		return nil, fmt.Errorf("cannot build a playlist without a series")
	}
	if strings.HasPrefix(seri.ID, standAloneSeriesPrefix) {
		return nil, fmt.Errorf("%q is a stand-alone message, not a series, and has no playlist",
			seri.Name)
	}

	privacy, err := PrivacyForView(seri.Visibility)
	if err != nil {
		return nil, fmt.Errorf("series %q: %w", seri.Name, err)
	}

	ministry := seri.GetMinistry()
	return &Series{
		Channel:     ChannelForMinistry(ministry),
		Title:       PlaylistTitle(ministry, seri.Name),
		Description: SeriesDescription(seri),
		Privacy:     privacy,
	}, nil
}
