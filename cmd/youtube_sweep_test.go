package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/util/box"
	"github.com/WordOfLifeMN/online/ytpkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sweepCatalog is a CORE series of three numbered messages and a track-0 extra, a WOL
// series with no Series row, and an Ask the Pastor message
func sweepCatalog(t *testing.T) *catalog.Catalog {
	core := catalog.CenterOfRelationshipExperience
	video := func(id string) *catalog.OnlineResource { return &catalog.OnlineResource{URL: "https://youtu.be/" + id} }

	cat := &catalog.Catalog{
		Series: []catalog.CatalogSeri{{Name: "Healthy Limits", Visibility: catalog.Public}},
		Messages: []catalog.CatalogMessage{
			{Name: "One", Ministry: core, Visibility: catalog.Public, Video: video("one00000000"),
				Series: []catalog.SeriesReference{{Name: "Healthy Limits", Index: 1}}},
			{Name: "Two", Ministry: core, Visibility: catalog.Public, Video: video("two00000000"),
				Series: []catalog.SeriesReference{{Name: "healthy limits", Index: 2}}},
			{Name: "Three", Ministry: core, Visibility: catalog.Public, Video: video("three000000"),
				Series: []catalog.SeriesReference{{Name: "Healthy Limits", Index: 3}}},
			{Name: "Q&A", Ministry: core, Visibility: catalog.Public, Video: video("qanda000000"),
				Series: []catalog.SeriesReference{{Name: "Healthy Limits", Index: 0}}},
			{Name: "Not uploaded", Ministry: core, Visibility: catalog.Public,
				Series: []catalog.SeriesReference{{Name: "Healthy Limits", Index: 4}}},

			{Name: "Voting", Ministry: catalog.WordOfLife, Visibility: catalog.Public},
			{Name: "The Importance of Voting", Ministry: catalog.WordOfLife, Visibility: catalog.Partner,
				Video: video("-4AH6lVaW38"), Series: []catalog.SeriesReference{{Name: "Voting", Index: 1}}},

			{Name: "Tithing?", Ministry: catalog.AskThePastor, Visibility: catalog.Public, Video: video("atp00000000")},
		},
	}
	require.NoError(t, cat.Initialize())
	return cat
}

func TestTracksFor(t *testing.T) {
	run := &syncRun{cat: sweepCatalog(t), trackOf: map[string]map[string]int{}}

	// track 0 and a row with no video are not numbered
	assert.Equal(t, map[string]int{"one00000000": 1, "two00000000": 2, "three000000": 3},
		run.tracksFor("CORE | Healthy Limits"))

	// a series with no Series row still has its tracks, and a stand-alone message of
	// the same name does not confuse it
	assert.Equal(t, map[string]int{"-4AH6lVaW38": 1}, run.tracksFor("Voting"))

	// Ask the Pastor is not numbered
	assert.Empty(t, run.tracksFor("Ask the Pastor"))

	// a video this run uploaded, whose link is not in the sheet yet, is added
	run.trackOf["CORE | Healthy Limits"] = map[string]int{"four0000000": 4}
	assert.Equal(t, 4, run.tracksFor("CORE | Healthy Limits")["four0000000"])
}

func TestSeriesForPlaylist(t *testing.T) {
	run := &syncRun{cat: sweepCatalog(t)}

	seri := run.seriesForPlaylist("CORE | Healthy Limits")
	require.NotNil(t, seri)
	assert.Equal(t, "Healthy Limits", seri.Name)

	assert.Nil(t, run.seriesForPlaylist("Healthy Limits"), "a CORE series' playlist carries the marker")
	assert.Nil(t, run.seriesForPlaylist("Voting"), "no Series row, and the stand-alone series is not it")
	assert.Nil(t, run.seriesForPlaylist("Ask the Pastor"))
}

func TestPrintProposedOrder(t *testing.T) {
	items := []ytpkg.PlaylistItem{
		{ItemID: "i2", VideoID: "two00000000", Title: "Two", Position: 0},
		{ItemID: "ix", VideoID: "extra000000", Title: "Extra", Position: 1},
		{ItemID: "i1", VideoID: "one00000000", Title: "One", Position: 2},
	}
	tracks := map[string]int{"one00000000": 1, "two00000000": 2}
	moves := ytpkg.PlanReorder(items, tracks)
	require.Len(t, moves, 1)

	var buf bytes.Buffer
	b := box.New(&buf)
	b.Header("")
	printProposedOrder(b, items, tracks, moves)
	b.Footer()
	out := buf.String()

	positions := []int{}
	for _, marker := range []string{
		"├─ Proposed order ",
		"│   1. One  (track 1, moves from 3) ",
		"│   2. Two  (track 2) ",
		"│   3. Extra  (not numbered) ",
		"│ 1 moves, about 50 quota units. ",
	} {
		index := strings.Index(out, marker)
		assert.GreaterOrEqual(t, index, 0, "missing %q in\n%s", marker, out)
		positions = append(positions, index)
	}
	assert.IsIncreasing(t, positions)
}

// a playlist both created and refused a reorder is listed once
func TestPrintSummary_ListsEachPlaylistOnce(t *testing.T) {
	var buf bytes.Buffer
	voting := &ytpkg.Series{ID: "PLvote", Title: "Voting"}
	run := &syncRun{
		out:         &buf,
		created:     []*ytpkg.Series{voting},
		needsManual: []*ytpkg.Series{voting, {ID: "PLcore", Title: "CORE | Healthy Limits"}},
	}

	run.printSummary()
	out := buf.String()
	assert.Equal(t, 1, strings.Count(out, "PLvote/edit"))
	assert.Equal(t, 1, strings.Count(out, "PLcore/edit"))
}
