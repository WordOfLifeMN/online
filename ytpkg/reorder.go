package ytpkg

import (
	"slices"
	"sort"
)

// Move is one playlistItems.update that repositions an item. Positions count from
// zero, as YouTube's do. To is the item's position once the move is done.
type Move struct {
	ItemID  string
	VideoID string
	Title   string
	From    int
	To      int
}

// DesiredOrder is the order a playlist should be in: the videos the sheet numbers come
// first, in track order, and everything else follows in its current relative order.
// tracks maps a video ID to its track. Items keep their current order where tracks tie.
func DesiredOrder(items []PlaylistItem, tracks map[string]int) []PlaylistItem {
	current := sortedByPosition(items)

	var numbered, rest []PlaylistItem
	for _, item := range current {
		if tracks[item.VideoID] > 0 {
			numbered = append(numbered, item)
		} else {
			rest = append(rest, item)
		}
	}
	sort.SliceStable(numbered, func(a, b int) bool {
		return tracks[numbered[a].VideoID] < tracks[numbered[b].VideoID]
	})

	return append(numbered, rest...)
}

// PlanReorder lists the moves that put a playlist in its desired order, as few as
// possible.
//
// Each move costs 50 quota units, so only what must move does. The items forming the
// longest run that is already in the right relative order stay where they are; every
// other item is moved, in desired order, to just after the item that should precede
// it. With one video out of place, that is one move.
func PlanReorder(items []PlaylistItem, tracks map[string]int) []Move {
	current := sortedByPosition(items)
	desired := DesiredOrder(items, tracks)

	// where each item should end up, by item ID
	target := make(map[string]int, len(desired))
	for index, item := range desired {
		target[item.ItemID] = index
	}

	// the items that stay put: the longest increasing run of targets in the current
	// order
	sequence := make([]int, len(current))
	for index, item := range current {
		sequence[index] = target[item.ItemID]
	}
	stays := map[string]bool{}
	for _, index := range longestIncreasing(sequence) {
		stays[current[index].ItemID] = true
	}

	// simulate the moves on a copy of the list, so each one's positions are those
	// YouTube will see when it is made
	list := slices.Clone(current)
	indexOf := func(itemID string) int {
		return slices.IndexFunc(list, func(item PlaylistItem) bool { return item.ItemID == itemID })
	}

	var moves []Move
	for index, item := range desired {
		if stays[item.ItemID] {
			continue
		}

		from := indexOf(item.ItemID)
		list = slices.Delete(list, from, from+1)

		to := 0
		if index > 0 {
			to = indexOf(desired[index-1].ItemID) + 1
		}
		list = slices.Insert(list, to, item)

		if from != to {
			moves = append(moves, Move{
				ItemID: item.ItemID, VideoID: item.VideoID, Title: item.Title, From: from, To: to,
			})
		}
	}

	return moves
}

// sortedByPosition returns the items in their current playlist order
func sortedByPosition(items []PlaylistItem) []PlaylistItem {
	out := slices.Clone(items)
	sort.SliceStable(out, func(a, b int) bool { return out[a].Position < out[b].Position })
	return out
}

// longestIncreasing returns the indexes of a longest strictly increasing subsequence of
// values, in order
func longestIncreasing(values []int) []int {
	// tails[k] is the index of the smallest value that ends an increasing run of
	// length k+1; previous links each index to the one before it in its run
	var tails []int
	previous := make([]int, len(values))

	for index, value := range values {
		k := sort.Search(len(tails), func(i int) bool { return values[tails[i]] >= value })
		if k > 0 {
			previous[index] = tails[k-1]
		} else {
			previous[index] = -1
		}
		if k == len(tails) {
			tails = append(tails, index)
		} else {
			tails[k] = index
		}
	}

	if len(tails) == 0 {
		return nil
	}
	run := make([]int, len(tails))
	for i, index := len(tails)-1, tails[len(tails)-1]; i >= 0; i, index = i-1, previous[index] {
		run[i] = index
	}
	return run
}
