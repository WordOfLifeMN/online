# Unify the `audio` command's output

## Goals

The `audio` command ends by printing, for each message, everything needed to upload it to YouTube. That output is correct but hard to scan: loose label lines, separate open-edged boxes, and a timeline that only appears through the log as a comma-separated line with full-precision durations.

This change puts each message in one closed box with titled sections. Values meant to be copied into YouTube stay on bare lines, so they can still be selected in one gesture. The box drawing goes into a small reusable package, `util/box`, so other commands can use the same style later.

## Requirements

### Box style
- Boxes use Unicode box-drawing characters only. There is no ASCII mode. (Q1)
- A box is as wide as the terminal, borders included: header `╭…╮`, section dividers `├…┤`, footer `╰…╯`. Width is counted in runes (`unicode/utf8`).
- The width is measured once, when the box is opened, so resizing the window affects the next box rather than one half drawn. It comes from, in order: the size the terminal reports (`golang.org/x/term`), then the `COLUMNS` environment variable, then a default of 132. A measured width is one less than the terminal's, because some consoles insert a line break after a line that fills the last column, and it is held between 40 and 256. Output that is not going to the console, such as a test's `bytes.Buffer`, uses the default without measuring. (Added at the Phase 2 checkpoint; the plan originally fixed every box at 78.)
- A section title sits inside its divider: `├─ UPLOAD PACKET ──────…──┤`. (Q2) A header can carry a title the same way: `╭─ Message #1 ──────…──╮`. (Q3)
- Bordered text lines are `│ <text> │`, padded to the right edge. Text wider than the inside of the box (the width less 4) wraps at spaces. A single word too long for a line is broken at the edge.
- A labeled line (`Channel : WOL`) wraps with continuation lines indented to line up under the value.
- A *copy line* is printed bare: no border, no wrapping, no padding, and passed through exactly as given, including embedded newlines and blank lines. Copy lines are used for every value pasted into YouTube: titles, description, playlist name, and thumbnail path. (Q4, Q5)
- A title too long for its header or divider is cut short with `…`.

### Per-message box
Each message gets one box, with a blank line between boxes. When a message has an upload packet, the box contains, in order:

1. Header `╭─ Message #N ─…╮`, then `│ Video file: <base name>` and `│ Speaker   : <name>`. (Q3)
2. **UPLOAD PACKET** section: `│ Channel : …`, `│ Privacy : …`, and when there is a playlist, `│ Playlist: <name> (position N)` (without the position when it is 0). When there is a thumbnail, `│ Thumb   :` is followed by the path as a copy line. (Q5, Q6)
3. **Titles** section: `│ Title` followed by the title as a copy line. When there is an alternative, `│ Alternative Title` followed by it as a copy line.
4. **Description** section: the whole description as copy lines, including its blank lines and resource links. (Q4)
5. **Playlist** section, only when there is a playlist: the playlist name as a copy line. (Q6)
6. **Risk notes** section, only when risk notes would be printed today (`wantRiskNotes`). It holds the same content as now, as bordered, wrapped lines. Its title is `Platform risk notes (advisory - not for upload)`; the `⚠` is dropped because its display width varies between terminals and would push the right border out of line. (Q7)
7. **Timeline** section, only with `--verbose` (Q8): one row per step, durations lined up and rounded to whole seconds, then a total row (Q9):
   ```
   │ Extract   :    24s                                                         │
   │ Transcribe:  2m08s                                                         │
   │ Summarize :     4s                                                         │
   │ Total     :  2m36s                                                         │
   ```
   A step that did not run shows `reused` when the message was summarized (its earlier output was reused), and `-` otherwise (processing stopped before it). The existing `log.Printf("Timeline: …")` line is removed.
8. Footer `╰─…╯`.

Your note on Q8 asked for "something more than a comma-delimited list", with emoji prefixes as one idea. Emoji are left out for now for the same width reason as `⚠`. Revisit this at the Phase 2 checkpoint if the table isn't enough.

### Messages without an upload packet (Q10)
- **Processed but not matched to a spreadsheet row** (`PacketError` set and a title or summary was generated): same header and video/speaker lines, then a **No upload packet** section with the reason as a labeled, wrapped line (`│ Reason: …`), then **Titles** (`│ Title` and the generated title as a copy line) and **Description** (the generated summary as copy lines). Risk notes, timeline, and footer follow as above.
- **Not processed** (no title and no summary): same header and video/speaker lines, then a **Not processed** section with `│ Processing failed - see the error below.`, and with `│ Reason: …` when there is also a `PacketError`. Timeline (verbose only) and footer follow.
- The order of these checks matters. `PacketError` is set before processing when there is no spreadsheet row (`newMessageInfo`), so a message that has no row *and* failed processing must go to "Not processed", not "No upload packet" with empty titles. Today it does the latter.

### Startup banner (Q11)
`printAudioBanner` draws its box with `util/box`: header `╭─ Prepare messages for YouTube ─…╮`, then the existing instructions as wrapped text lines with a blank bordered line between paragraphs, then a footer. The text itself does not change.

## Out of scope
- Any output other than the startup banner and the per-message results: the input prompts, the `── <file>` and `Spreadsheet:` lines printed as each video is entered, warnings, and the scratch-directory clean-up message stay as they are.
- What the output contains. Every value printed today is still printed; only its layout changes, plus the Timeline moving from the log into the box.
- Other commands. `util/box` is reusable, but nothing outside `cmd/audio*.go` and `cmd/publish.go` switches to it in this change.

## Architecture

### New package `util/box` (`util/box/box.go`)

```go
package box

// widths of a box, borders included
const (
	DefaultWidth = 132 // when the terminal's width cannot be measured
	MinWidth     = 40  // a measured width is held between these two
	MaxWidth     = 256
)

// Box draws one box to a writer, line by line. It holds no buffered state, so lines
// appear as soon as they are written.
type Box struct {
	w     io.Writer
	width int
}

func New(w io.Writer) *Box                 // the terminal's width, measured by Width(w)
func NewWidth(w io.Writer, width int) *Box // an explicit width, not held to Min/MaxWidth; for tests
func Width(w io.Writer) int                // terminal size, then COLUMNS, then DefaultWidth; clamped

func (b *Box) Header(title string)         // ╭─ title ───╮, or ╭────╮ when title is ""
func (b *Box) Section(title string)        // ├─ title ───┤, or ├────┤ when title is ""
func (b *Box) Footer()                     // ╰────╯
func (b *Box) Blank()                      // │          │
func (b *Box) Line(text string)            // │ text     │  wrapped; "\n" in text starts a new line
func (b *Box) Linef(format string, a ...any)
func (b *Box) Labeled(label, value string) // │ label value │  continuation lines indented by len(label)
func (b *Box) Copy(text string)            // text written bare, exactly as given, with a trailing newline
```

Helpers inside the package, not exported: `wrap(text string, width, indent int) []string` (wrap at spaces, hard-break overlong words), `rule(left, title, right string) string` (header, divider, and footer lines), and `pad(s string) string` (pad to the inner width in runes).

`Labeled` covers the risk-note bullets as well: `Labeled("• ", text)` indents continuation lines by two.

Writing to an `io.Writer` is what lets the tests capture output in a `bytes.Buffer`. Errors from the writer are ignored, as `fmt.Printf` ignores them today.

### Changes in `cmd`
- `cmd/publish.go`: `(*UploadPacket).Print()` becomes `(*UploadPacket).Print(b *box.Box)`. It writes the UPLOAD PACKET, Titles, Description, and Playlist sections into a box the caller has opened, and it no longer prints a header or footer. Its comment about copy-friendly fields is updated to describe copy lines.
- `cmd/audio.go`:
  - `printMessageInfo(index int, info *MessageInfo)` becomes `printMessageInfo(w io.Writer, index int, info *MessageInfo)`. It opens the box, writes the header and the video/speaker lines, chooses among the three cases in the order given above, calls `printRiskNotes` and `printTimeline`, closes the box, and prints a blank line. `audio()` passes `os.Stdout`.
  - New `printTimeline(b *box.Box, info *MessageInfo)`: returns at once unless `viper.GetBool("verbose")`. It replaces the `log.Printf` timeline.
  - New `formatStepDuration(w util.StopWatch, summarized bool) string`: `24s`, `2m08s`, `1h02m03s`, or `reused`/`-` when the stopwatch never started (`StartTime.IsZero()`). The total row sums only the steps that ran.
  - `printAudioBanner()` becomes `printAudioBanner(w io.Writer)` and uses `util/box`.
- `cmd/audio_summarize.go`: `printRiskNotes(info)` becomes `printRiskNotes(b *box.Box, info *MessageInfo)` and writes a section, keeping its current wording and its `wantRiskNotes` gate.

`Width` measures only when `w` is an `*os.File`. Any other writer gets `DefaultWidth`, so tests that capture output in a buffer do not depend on the terminal they run in or on `COLUMNS`.

### Sample (matched message, `--verbose`, drawn 78 wide to fit the page)

```
╭─ Message #1 ───────────────────────────────────────────────────────────────╮
│ Video file: 2026-10-04-pm Pay Attention.mp4                                │
│ Speaker   : Pastor Mary Peltz                                              │
├─ UPLOAD PACKET ────────────────────────────────────────────────────────────┤
│ Channel : WOL                                                              │
│ Privacy : public                                                           │
│ Playlist: Prayer (position 3)                                              │
│ Thumb   :                                                                  │
C:\Users\WordofLifeMNMedia\Documents\WOLMessages\2026-q4 Prayer\2026-10-04-pm Pay Attention.jpg
├─ Titles ───────────────────────────────────────────────────────────────────┤
│ Title                                                                      │
Pay Attention | Mary Peltz | October 4, 2026
│ Alternative Title                                                          │
Pay Attention to God's Breadcrumbs | Mary Peltz | October 4, 2026
├─ Description ──────────────────────────────────────────────────────────────┤
Pastor Mary talks about the importance of paying attention to spiritual breadcrumbs God is laying out for us this week. ...

Sermon notes
https://example.org/notes.pdf
├─ Playlist ─────────────────────────────────────────────────────────────────┤
Prayer
├─ Timeline ─────────────────────────────────────────────────────────────────┤
│ Extract   :    24s                                                         │
│ Transcribe:  2m08s                                                         │
│ Summarize :     4s                                                         │
│ Total     :  2m36s                                                         │
╰────────────────────────────────────────────────────────────────────────────╯
```

## Tasks

### Phase 1: `util/box` package
- [x] 1.1 Create `util/box/box.go` with `Box`, `New`, `NewWidth`, `Header`, `Section`, `Footer`, `Blank`, and the helpers `rule` and `pad`, including cutting long titles short with `…`.
- [x] 1.2 Add `wrap` and the text methods `Line`, `Linef`, `Labeled`, and `Copy` to `util/box/box.go`.
- [x] 1.3 Create `util/box/box_test.go` (a testify `suite`, like `util/os_test.go`) covering the cases under "Automated" below for `util/box`, and get `go test ./util/...` passing.

### Phase 2: per-message box
- [x] 2.1 `cmd/publish.go`: change `(*UploadPacket).Print` to `Print(b *box.Box)` writing the UPLOAD PACKET, Titles, Description, and Playlist sections as specified; update its comment.
- [x] 2.2 `cmd/audio_summarize.go`: change `printRiskNotes` to `printRiskNotes(b *box.Box, info *MessageInfo)` writing the Risk notes section.
- [x] 2.3 `cmd/audio.go`: add `formatStepDuration` and `printTimeline`, and remove the `log.Printf` timeline.
- [x] 2.4 `cmd/audio.go`: rewrite `printMessageInfo` to take an `io.Writer` and draw the box, with the three cases checked in the specified order. Update the call in `audio()`.
- [x] 2.5 Add tests to `cmd/audio_test.go` and `cmd/publish_test.go` covering the cases under "Automated" below for `cmd`, and get `go test ./...` passing.
- [x] 2.6 `util/box/box.go`: size boxes to the terminal. Add `MinWidth`, `MaxWidth`, and `Width`; set `DefaultWidth` to 132; make `New` use `Width(w)`. Add `golang.org/x/term` to `go.mod`. Test `Width` in `util/box/box_test.go`: a buffer gets the default, `COLUMNS` is used for a file that is not a terminal, out-of-range values are clamped, and an unreadable `COLUMNS` falls back to the default.
- [x] **Checkpoint (you):** run `online audio` on one or two videos, once without and once with `--verbose`, and again in a narrow and a wide window. Check that each message is one closed box whose right border lines up, that the title, description, playlist, and thumbnail lines copy cleanly, and that the Timeline appears only with `--verbose`. Say whether the timeline table is enough or you want something more (Q8).

### Phase 3: startup banner
- [x] 3.1 `cmd/audio.go`: change `printAudioBanner` to `printAudioBanner(w io.Writer)` drawn with `util/box`, and update its call in `audio()`.
- [x] 3.2 Add a banner test to `cmd/audio_test.go`, and get `go test ./...` passing.
- [x] **Checkpoint (you):** run `online audio` with no arguments and check that the banner is a closed box with the same instructions.

## Test plan

### Automated (Claude)
- [x] `util/box`: `Width` uses the default for a non-file writer, reads `COLUMNS` for a file that is not a terminal, clamps to 40–256, and ignores an unreadable `COLUMNS`.
- [x] `util/box`: every header, divider, footer, and bordered line is exactly the box width in runes, including lines with multi-byte text such as curly quotes and `•`.
- [x] `util/box`: a header or section title appears inside the rule; an empty title gives a plain rule; a title too long for the width is cut short with `…`.
- [x] `util/box`: `Line` wraps at spaces, starts a new line at an embedded `\n`, and hard-breaks a word longer than the inner width.
- [x] `util/box`: `Labeled` lines up continuation lines under the value.
- [x] `util/box`: `Copy` writes its text exactly, with newlines and blank lines intact and no border.
- [x] `cmd`: a matched message's output contains the title, alternative, description, playlist name, and thumbnail path as exact bare lines, and the sections in the specified order.
- [x] `cmd`: a message without a playlist has no Playlist line and no Playlist section; one without an alternative has no Alternative Title.
- [x] `cmd`: an unmatched message shows a No upload packet section with the reason, plus the generated title and summary; a message that failed processing shows Not processed, including when it also has no spreadsheet row.
- [x] `cmd`: the Timeline section appears only when `verbose` is set (`viper.Set`), with `reused` and `-` shown in the right cases.
- [x] `cmd`: `formatStepDuration` gives `24s`, `2m08s`, and `1h02m03s` for matching durations.
- [x] `go vet ./...` and `go test ./...` pass.

### Manual (you)
- [x] Phase 2 checkpoint: per-message boxes look right and copy cleanly, with and without `--verbose`.
- [x] Phase 3 checkpoint: the startup banner is a closed box.
- [ ] Run a message that has no spreadsheet row (for example a renamed copy of a video) and check the No upload packet box.

## Deviations

- **Summary stopwatch stopped on failure** (`processOneAudio`, `cmd/audio.go`). *Approved at the Phase 2 checkpoint.* `SummaryTime` was started but never stopped when `generateMessageSummary` failed, so the new Timeline would have shown the time elapsed since the summary began, including the processing of every later video. It is now stopped before the error check, as Extract and Transcribe already were.
- **`wrap` keeps a line that fits exactly as given** (`util/box`). The plan said lines wrap at spaces. Collapsing spaces on every line also flattened the spacing that lines things up: the right-aligned Timeline column, and `Thumb   :` under `Channel :`. Runs of spaces now collapse only on lines that actually wrap. The internal helper is also `wrap(text, width)`, without the plan's `indent` parameter, because `Labeled` does the indenting.
- **Boxes sized to the terminal** (`util/box`, task 2.6). Requested at the Phase 2 checkpoint, replacing the fixed width of 78: the terminal's reported width, then `COLUMNS`, then a default of 132, held between 40 and 256. A measured width is one column narrower than the terminal, so a console that wraps a line filling the last column does not double-space the box. Approved at the Phase 2 checkpoint. Adds `golang.org/x/term` v0.33.0, the version already in the module graph, as a direct dependency.
