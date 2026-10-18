package ytpkg

import (
	"fmt"
	"math/rand"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// playlistOf builds items in the given current order, named by their video IDs
func playlistOf(videoIDs ...string) []PlaylistItem {
	items := make([]PlaylistItem, len(videoIDs))
	for index, id := range videoIDs {
		items[index] = PlaylistItem{ItemID: "item-" + id, VideoID: id, Title: "Title " + id, Position: index}
	}
	return items
}

// apply performs moves on a playlist as YouTube would, and returns the video IDs in
// their resulting order
func apply(items []PlaylistItem, moves []Move) []string {
	list := slices.Clone(sortedByPosition(items))
	for _, move := range moves {
		item := list[move.From]
		if item.ItemID != move.ItemID {
			panic(fmt.Sprintf("move expects %s at %d, found %s", move.ItemID, move.From, item.ItemID))
		}
		list = slices.Delete(list, move.From, move.From+1)
		list = slices.Insert(list, move.To, item)
	}

	ids := make([]string, len(list))
	for index, item := range list {
		ids[index] = item.VideoID
	}
	return ids
}

func ids(items []PlaylistItem) []string {
	out := make([]string, len(items))
	for index, item := range items {
		out[index] = item.VideoID
	}
	return out
}

func TestReorder_AlreadyOrderedNeedsNoMoves(t *testing.T) {
	items := playlistOf("a", "b", "c", "x")
	tracks := map[string]int{"a": 1, "b": 2, "c": 3}

	assert.Empty(t, PlanReorder(items, tracks))
	assert.Empty(t, PlanReorder(nil, tracks))
}

func TestReorder_OneOutOfPlace(t *testing.T) {
	// the newest upload, added at the end, is track 1
	items := playlistOf("b", "c", "d", "a")
	tracks := map[string]int{"a": 1, "b": 2, "c": 3, "d": 4}

	moves := PlanReorder(items, tracks)
	require.Len(t, moves, 1)
	assert.Equal(t, Move{ItemID: "item-a", VideoID: "a", Title: "Title a", From: 3, To: 0}, moves[0])
	assert.Equal(t, []string{"a", "b", "c", "d"}, apply(items, moves))
}

// moving the one early item costs one move, where placing each track in turn would
// cost three
func TestReorder_MinimalNotNaive(t *testing.T) {
	items := playlistOf("d", "a", "b", "c")
	tracks := map[string]int{"a": 1, "b": 2, "c": 3, "d": 4}

	moves := PlanReorder(items, tracks)
	require.Len(t, moves, 1)
	assert.Equal(t, "d", moves[0].VideoID)
	assert.Equal(t, []string{"a", "b", "c", "d"}, apply(items, moves))
}

// videos the sheet does not number go after the numbered ones, in their current order
func TestReorder_UnnumberedFollowInCurrentOrder(t *testing.T) {
	items := playlistOf("x", "b", "y", "a", "z")
	tracks := map[string]int{"a": 1, "b": 2}

	assert.Equal(t, []string{"a", "b", "x", "y", "z"}, ids(DesiredOrder(items, tracks)))

	moves := PlanReorder(items, tracks)
	assert.Equal(t, []string{"a", "b", "x", "y", "z"}, apply(items, moves))
	assert.Len(t, moves, 2, "x y z stay put; a and b move in front of them")
}

// a playlist with no numbered videos is left as it is
func TestReorder_NothingNumbered(t *testing.T) {
	items := playlistOf("x", "y", "z")
	assert.Empty(t, PlanReorder(items, nil))
	assert.Empty(t, PlanReorder(items, map[string]int{"elsewhere": 1}))
}

// equal tracks keep their current order
func TestReorder_TiesKeepCurrentOrder(t *testing.T) {
	items := playlistOf("q", "p", "a")
	tracks := map[string]int{"p": 2, "q": 2, "a": 1}
	assert.Equal(t, []string{"a", "q", "p"}, ids(DesiredOrder(items, tracks)))
}

// positions are taken from the items, not from the order they are handed over in
func TestReorder_UsesPositions(t *testing.T) {
	items := playlistOf("a", "b", "c")
	slices.Reverse(items)
	assert.Empty(t, PlanReorder(items, map[string]int{"a": 1, "b": 2, "c": 3}))
}

// whatever the starting order, the moves produce the desired order, and no more of
// them than there are items outside the longest run already in order
func TestReorder_RandomOrdersAlwaysEndCorrect(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for round := 0; round < 300; round++ {
		n := r.Intn(12)
		var videoIDs []string
		tracks := map[string]int{}
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("v%02d", i)
			videoIDs = append(videoIDs, id)
			if r.Intn(3) > 0 {
				tracks[id] = r.Intn(8) + 1
			}
		}
		r.Shuffle(len(videoIDs), func(a, b int) { videoIDs[a], videoIDs[b] = videoIDs[b], videoIDs[a] })
		items := playlistOf(videoIDs...)

		desired := ids(DesiredOrder(items, tracks))
		moves := PlanReorder(items, tracks)
		require.Equal(t, desired, apply(items, moves), "round %d from %v", round, videoIDs)

		// the moves are minimal: n minus the longest run already in order
		target := map[string]int{}
		for index, id := range desired {
			target[id] = index
		}
		sequence := make([]int, n)
		for index, id := range videoIDs {
			sequence[index] = target[id]
		}
		assert.Len(t, moves, n-len(longestIncreasing(sequence)), "round %d", round)
	}
}

func TestLongestIncreasing(t *testing.T) {
	values := []int{3, 0, 1, 2}
	assert.Equal(t, []int{1, 2, 3}, longestIncreasing(values))
	assert.Empty(t, longestIncreasing(nil))
	assert.Len(t, longestIncreasing([]int{5, 4, 3}), 1)
}
