package ytpkg

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/WordOfLifeMN/online/util"
	"github.com/WordOfLifeMN/online/util/box"
	"github.com/pmezard/go-difflib/difflib"
)

// Change is one field that differs between YouTube and what the sheet says it should be
type Change struct {
	Field string
	Now   string // what YouTube has
	New   string // what it will have

	// Destructive marks a change that loses something not recorded anywhere else: a
	// description whose text is not in the sheet
	Destructive bool

	// Summary, when set, describes the change in one line in place of Now and New. Used
	// for changes that are actions rather than values, such as adding to a playlist.
	Summary string
}

// field names, also the labels the changes are printed under
const (
	FieldTitle                = "Title"
	FieldDescription          = "Description"
	FieldPrivacy              = "Privacy"
	FieldCategory             = "Category"
	FieldDefaultLanguage      = "Language"
	FieldDefaultAudioLanguage = "Audio lang"
	FieldMadeForKids          = "For kids"
	FieldPlaylist             = "Playlist"
	FieldThumbnail            = "Thumbnail"
)

// DiffMessage lists the managed fields of a video that differ from what the sheet
// wants: title, description, privacy, category, both languages and made-for-kids.
// Everything else YouTube holds is left alone, so is not compared. Playlists are
// handled separately.
//
// want's privacy must already be resolved against the live video (see
// NewMessageForLive), so that a consistent live privacy is not reported as a change.
func DiffMessage(live, want *Message) []Change {
	var changes []Change
	add := func(field, now, new string) {
		if now != new {
			changes = append(changes, Change{Field: field, Now: now, New: new})
		}
	}

	add(FieldTitle, live.Title, want.Title)
	add(FieldPrivacy, string(live.Privacy), string(want.Privacy))
	add(FieldCategory, live.CategoryID, want.CategoryID)
	add(FieldDefaultLanguage, live.DefaultLanguage, want.DefaultLanguage)
	add(FieldDefaultAudioLanguage, live.DefaultAudioLanguage, want.DefaultAudioLanguage)
	add(FieldMadeForKids, strconv.FormatBool(live.MadeForKids), strconv.FormatBool(want.MadeForKids))

	return append(changes, diffDescription(live.Description, want.Description)...)
}

// DiffSeries lists the fields of a playlist that differ from what the sheet wants: its
// description and privacy. The title is never synced, because it is the key the
// playlist was found by.
func DiffSeries(live, want *Series) []Change {
	var changes []Change
	if live.Privacy != want.Privacy {
		changes = append(changes, Change{
			Field: FieldPrivacy, Now: string(live.Privacy), New: string(want.Privacy),
		})
	}
	return append(changes, diffDescription(live.Description, want.Description)...)
}

// diffDescription compares descriptions, ignoring trailing whitespace, which YouTube
// strips
func diffDescription(now, new string) []Change {
	if normalizeDescription(now) == normalizeDescription(new) {
		return nil
	}
	return []Change{{
		Field:       FieldDescription,
		Now:         now,
		New:         new,
		Destructive: IsDestructiveDescription(now, new),
	}}
}

// normalizeDescription removes the differences YouTube introduces by itself: Windows
// line endings and trailing whitespace
func normalizeDescription(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for index := range lines {
		lines[index] = strings.TrimRight(lines[index], " \t")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// IsDestructiveDescription reports whether replacing a description loses text: the
// current one is not empty, and some non-blank line of it is neither in the new one nor
// edited into a line of it. Adding lines, such as resource links, is not destructive,
// and neither is editing one.
func IsDestructiveDescription(now, new string) bool {
	return len(LostDescriptionLines(now, new)) > 0
}

// editedLineOverlap is the share of a line's words that must survive in a line of the
// new description for the line to count as edited rather than removed. Deleting a
// sentence from a three-sentence paragraph keeps about two thirds of it; replacing the
// paragraph with a different summary keeps far less.
const editedLineOverlap = 0.6

// LostDescriptionLines returns the non-blank lines of the current description that are
// gone from the new one, in order.
//
// A line is gone when the new description neither contains it, ignoring surrounding
// whitespace, nor has a line it was edited into. The sheet's previous text is not
// known, so an edit made in the sheet and text that only ever existed on YouTube look
// alike line for line. Word overlap tells them apart: an edited line keeps most of its
// words, while a line YouTube alone had - "Watch live Sundays at 10", or a hand-written
// description the sheet's summary replaces - shares few with anything new.
func LostDescriptionLines(now, new string) []string {
	newLines := strings.Split(normalizeDescription(new), "\n")
	keep := map[string]bool{}
	for _, line := range newLines {
		keep[strings.TrimSpace(line)] = true
	}

	var lost []string
	for _, line := range strings.Split(normalizeDescription(now), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || keep[line] || isEditedInto(line, newLines) {
			continue
		}
		lost = append(lost, line)
	}
	return lost
}

// isEditedInto reports whether some new line keeps enough of a line's words for the
// line to have been edited into it. Two adjacent new lines are also tried together, so
// a paragraph split in two counts as edited. Wider than that, unrelated text spread
// over a whole description could add up to a match.
func isEditedInto(line string, newLines []string) bool {
	words := strings.Fields(util.NormalizeForMatch(line))
	if len(words) == 0 {
		return false
	}

	var nonBlank []string
	for _, candidate := range newLines {
		if strings.TrimSpace(candidate) != "" {
			nonBlank = append(nonBlank, candidate)
		}
	}
	candidates := append([]string{}, nonBlank...)
	for index := 1; index < len(nonBlank); index++ {
		candidates = append(candidates, nonBlank[index-1]+" "+nonBlank[index])
	}

	for _, candidate := range candidates {
		available := map[string]int{}
		for _, word := range strings.Fields(util.NormalizeForMatch(candidate)) {
			available[word]++
		}

		kept := 0
		for _, word := range words {
			if available[word] > 0 {
				available[word]--
				kept++
			}
		}
		if float64(kept) >= editedLineOverlap*float64(len(words)) {
			return true
		}
	}
	return false
}

// DescriptionDiff renders a unified line diff from YouTube's description to the
// sheet's
func DescriptionDiff(now, new string) string {
	diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(normalizeDescription(now)),
		B:        difflib.SplitLines(normalizeDescription(new)),
		FromFile: "YouTube",
		ToFile:   "Sheet",
		Context:  3,
	})
	if err != nil {
		return fmt.Sprintf("(cannot show the difference: %s)", err)
	}
	return strings.TrimRight(diff, "\n")
}

// changeLabelWidth lines the values up under the longest field name
const changeLabelWidth = 10

// PrintChanges writes changes into a box the caller has opened: the short fields as
// now/new pairs, then the description as a line diff, then a warning for anything
// destructive, with the text that would be lost as a copy line so it can be saved.
func PrintChanges(b *box.Box, changes []Change) {
	var description *Change

	b.Section("Changes")
	for index := range changes {
		change := &changes[index]
		label := fmt.Sprintf("%-*s: ", changeLabelWidth, change.Field)
		indent := strings.Repeat(" ", len(label))

		switch {
		case change.Field == FieldDescription:
			description = change
			b.Labeled(label, "see below")
		case change.Summary != "":
			b.Labeled(label, change.Summary)
		default:
			b.Labeled(label, "now: "+orEmpty(change.Now))
			b.Labeled(indent, "new: "+orEmpty(change.New))
		}
	}

	if description == nil {
		return
	}

	b.Section("Description")
	b.Copy(DescriptionDiff(description.Now, description.New))

	if description.Destructive {
		// the lost lines below stay plain: they are copy lines, meant to be selected
		// and pasted back into the sheet
		b.WarningSection("WARNING")
		b.WarningLine("This removes YouTube description text that is not in the sheet:")
		b.Copy(strings.Join(LostDescriptionLines(description.Now, description.New), "\n"))
	}
}

// HasDestructive reports whether any change is destructive
func HasDestructive(changes []Change) bool {
	for _, change := range changes {
		if change.Destructive {
			return true
		}
	}
	return false
}

// orEmpty shows an empty value as something visible
func orEmpty(s string) string {
	if s == "" {
		return "(empty)"
	}
	return s
}
