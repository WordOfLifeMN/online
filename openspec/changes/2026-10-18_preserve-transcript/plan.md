# Preserve transcripts beside their videos

## Goals

Making a transcript is the slow part of `online audio`: extracting the audio with ffmpeg, then running faster-whisper over it. Today the transcript is written to the scratch directory and swept after 24 hours. That's acceptable for a run that only needs a title and description, but it throws away something that is expensive to make and cheap to keep. Later uses include backfilling descriptions when the rules change, extracting keywords, and searching.

This change keeps every transcript the `audio` pipeline makes, as a `.txt` file beside the `.mp4` it came from. Whenever a transcript already exists, it is used, and the audio extraction and transcription are skipped.

## Requirements

### Where the transcript is kept
- The preserved transcript sits in the video's folder and has the video's name with a `.txt` extension: `D:\Videos\2026-10-12 Sun Pay Attention.mp4` gives `D:\Videos\2026-10-12 Sun Pay Attention.txt` (Q1).
- The scratch copy keeps faster-whisper's own `<name>.text` name, as today (Q1).
- Transcription still runs into the scratch directory. Only a transcript that has passed the existing non-empty check is copied beside the video. The copy goes through a temporary `.partial` file and a rename, so a failed or interrupted run never leaves a partial `.txt` beside the video. Because the reuse check skips transcription whenever the `.txt` exists, a partial file there would block every later run (Q2).
- If the copy fails (read-only share, OneDrive mid-sync, and so on), a warning is printed on stdout, the scratch transcript is used, and the message carries on to its summary. The transcript just isn't preserved this time (Q3).
- An existing `.txt` beside the video is never overwritten. The only way to regenerate a transcript is to delete that file by hand (from the prompt's out-of-scope section).

### Reusing a transcript
- `audio` looks for a transcript **before** extracting audio. Only the preserved transcript counts:
  1. a non-empty `<video>.txt` beside the video: use it, and skip both extraction and transcription;
  2. otherwise extract (reusing a non-empty scratch `.mp3` as today), transcribe, and copy beside the video.
- **Deleting the `.txt` alone always triggers a new transcription.** A scratch `.text` is never reused by `audio`, however recent. A scratch `.mp3` still is, so a regeneration within 24 hours only re-runs faster-whisper (Q4, changed after round 2: the user chose this over copying the scratch `.text` across). As a result, a transcript left in scratch before this change, or after a failed copy (Q3), is transcribed again on the next run, not copied across.
- Before transcribing, `audio` deletes any existing scratch `.text` without asking. Otherwise `transcribeAudio`'s "File exists. Do you want to overwrite it?" prompt would interrupt the slow part of the run, after the operator has gone. Answering "n" would also quietly reuse the stale file, which defeats deleting the `.txt`. The prompt stays for standalone `audio transcribe`.
- When the `.txt` is found, the Timeline already shows the two skipped steps as `reused`, so no change is needed there.
- Reusing a transcript is announced on stdout: `Using existing transcript: 2026-10-12 Sun Pay Attention.txt`. Since regenerating means deleting the file by hand, the operator needs to see when it was reused.
- `audio summarize <video>.mp4` reads the `.txt` beside the video, and falls back to the scratch `.text`, reading it only and not copying it. `summarize` never transcribes, so that fallback can't block anything. Given an `.mp3`, it still looks in scratch only. `.txt` is now accepted as direct input, alongside `.text`.
- Standalone `audio transcribe <file>.mp3` is unchanged: it writes only to scratch, which is swept after 24 hours (Q5).

### Header line
- As soon as faster-whisper finishes and the output passes the non-empty check, a header line and a blank line are added to the top of the scratch file. Every transcript made after this change carries its true model, wherever it is later copied from, including those from standalone `audio transcribe` (Q6, Q8):
  ```
  # wolm-transcript app=faster-whisper model=small transcribed=2026-10-18

  <transcript text>
  ```
  The line starts with `# `, the common script comment format (Q7 note). The `wolm-transcript` marker follows, so the summarizer can tell the header apart from a first line someone actually spoke. `app=faster-whisper` records the transcription software in case it changes later (Q7 note). `model=` is `getWhisperModel()` at transcription time. `transcribed=` is the local date, as `YYYY-MM-DD` (Q7).
- Transcripts written beside the video by this change always carry the header, since every one comes from a fresh transcription (Q8).
- Before sending a transcript to Claude, `generateMessageSummary` drops the first line if it starts with `# wolm-transcript`, along with the blank lines after it. A transcript without the header, including one written by hand, is sent unchanged (Q6).

### Settled without asking
- The `.mp3` stays a scratch intermediate, swept after 24 hours as today.
- The scratch sweep (`cleanUpScratchDir`) only ever deletes `.mp3` and `.text`, so a preserved `.txt` is safe even if `scratch-dir` is ever pointed at a video folder. No change is needed.
- The docs say transcripts are now kept: the `audio`, `audio transcribe`, `audio extract` and `audio summarize` help text, the README workflow diagram and its "intermediates" paragraph, the `scratch-dir` row in the README config table, and the `scratch-dir` comment in `TODO-km-online-config.yaml`.

## Out of scope
- Any refresh or override flag. To regenerate a transcript, delete the `.txt` by hand.
- Uploading transcripts anywhere: S3, YouTube captions, or the sheet.
- Migrating transcripts already in scratch. They are swept as usual, and their messages are transcribed again if run again.
- Recording the faster-whisper version, the trim length or the source video name in the header (Q7).
- Changes to `audio extract`'s behaviour.

## Architecture

All of the new code is in `cmd/audio_transcribe.go`, apart from one file helper in `util/os.go`.

### `util/os.go`
```go
// CopyFile copies src to dst by way of dst+".partial" and a rename, so that dst either
// does not exist or is complete - never cut short by a failure or an interrupted run
func CopyFile(src, dst string) error
```
Remove the `.partial` file if the copy fails.

### `cmd/audio_transcribe.go`
```go
// transcriptHeaderMarker begins the header line written at the top of every transcript
const transcriptHeaderMarker = "# wolm-transcript"

// transcriberApp names the transcription software in the header
const transcriberApp = "faster-whisper"

// getPreservedTranscriptPath returns <video dir>/<video name>.txt
func getPreservedTranscriptPath(videoPath string) string

// getScratchTranscriptPathFromVideoPath returns the scratch .text that transcribing this
// video's audio produces (getTranscribePathFromAudioPath(getAudioPathFromVideoPath(v)))
func getScratchTranscriptPathFromVideoPath(videoPath string) string

// transcriptHeader renders "# wolm-transcript app=faster-whisper model=small transcribed=2026-10-18"
func transcriptHeader(model string, when time.Time) string

// addTranscriptHeader rewrites the file with the header line and a blank line at the top
func addTranscriptHeader(path, header string) error

// stripTranscriptHeader drops a leading header line and the blank lines after it
func stripTranscriptHeader(text string) string

// preserveTranscript copies a scratch transcript beside its video, returning the
// preserved path. On failure it prints a warning and returns the scratch path, so
// the caller carries on with that (Q3).
func preserveTranscript(scratchPath, videoPath string) string

// findExistingTranscript returns the preserved .txt for a video if it exists and is
// non-empty, else "". A scratch .text is deliberately not considered: deleting the
// .txt must trigger a new transcription.
func findExistingTranscript(videoPath string) string
```

`transcribeAudio` calls `addTranscriptHeader(xscriptPath, transcriptHeader(getWhisperModel(), time.Now()))` after its existing non-empty check, and returns an error if that fails.

### `cmd/audio.go`: `processOneAudio`
```go
if path := findExistingTranscript(info.VideoPath); path != "" {
    info.TranscriptPath = path
    fmt.Printf("Using existing transcript: %s\n", filepath.Base(path))
} else {
    // extract (existing code, including the scratch .mp3 reuse)
    // delete any stale scratch .text without prompting: deleteExistingFile(path, false)
    // transcribe (now unconditional), then:
    info.TranscriptPath = preserveTranscript(info.TranscriptPath, info.VideoPath)
}
```
The old `if !util.IsNonEmptyFile(info.TranscriptPath)` check around transcription and its `else { log.Printf("Reusing existing transcript ...") }` branch both go.

Sample output when the copy fails (Q3):
```
WARNING: could not save the transcript beside the video: open D:\Videos\...partial: Access is denied.
         using the copy in the scratch directory, which is swept after 24 hours
```

### `cmd/audio_summarize.go`
- `xscriptSummarize`: accept `.txt` in `PromtUserForInputFile`. For an `.mp4`, use `findExistingTranscript`, else `getScratchTranscriptPathFromVideoPath` if that file exists, else report `no transcript found for <video>`. The `.mp3` and transcript-file paths are unchanged.
- `generateMessageSummary`: `xscript := strings.TrimSpace(stripTranscriptHeader(string(xscriptBytes)))`. The empty check stays after the strip, so a file holding only a header counts as empty.

## Tasks

### Phase 1: Helpers
- [x] 1.1 Add `CopyFile` with the `.partial`-then-rename behaviour. Files: `util/os.go` (`CopyFile`), `util/os_test.go`. The tests cover: content copied; no `.partial` left behind; a missing destination folder gives an error and leaves no `.partial`.
- [x] 1.2 Add `transcriptHeaderMarker`, `transcriberApp`, `transcriptHeader`, `addTranscriptHeader` and `stripTranscriptHeader`. Files: `cmd/audio_transcribe.go`, new `cmd/audio_transcribe_test.go`. The tests cover: the exact header for a fixed `time.Time`; adding a header then stripping it gives back the original text; text with no header is unchanged; a first line of ordinary `# ` text that isn't the marker is kept; CRLF line endings strip cleanly.
- [x] 1.3 Add `getPreservedTranscriptPath`, `getScratchTranscriptPathFromVideoPath`, `preserveTranscript` and `findExistingTranscript`. Files: `cmd/audio_transcribe.go`, `cmd/audio_transcribe_test.go`. Use a temporary `scratch-dir` and a temporary video folder, the way `withScratchDir` does in `cmd/audio_test.go`. The tests cover: `findExistingTranscript` returns the `.txt` when there is one; it returns `""` when there's only a scratch `.text`, when the `.txt` is empty, and when there's nothing; `preserveTranscript` copies the file beside the video and returns the `.txt` path; when the video's folder doesn't exist (so the copy fails), it returns the scratch path; it never overwrites an existing `.txt`.
- [x] 1.4 `go build ./... && go test ./...` passes.

### Phase 2: Pipeline
- [x] 2.1 In `transcribeAudio`, add the header after the non-empty check. File: `cmd/audio_transcribe.go`.
- [x] 2.2 Reorder `processOneAudio` as in Architecture: find the `.txt` first, otherwise extract, delete any stale scratch `.text` without prompting, transcribe and preserve. File: `cmd/audio.go` (`processOneAudio`, and its doc comment, which says intermediates are deleted on success).
- [x] 2.3 Strip the header in `generateMessageSummary`. File: `cmd/audio_summarize.go`.
- [x] 2.4 `go build -o online.exe && go test ./...` passes.
- [x] **Checkpoint (you):** Run `online --verbose audio "<a video>.mp4"` on a video that has never been transcribed, with its scratch `.text` deleted. Expect: `<video>.txt` appears beside the `.mp4`, with the `# wolm-transcript app=faster-whisper model=small transcribed=…` line and a blank line at the top, and the title and description look normal. Run it again. Expect: `Using existing transcript: <video>.txt`, no ffmpeg or faster-whisper output, and `reused` for both Extract and Transcribe in the Timeline.
- [x] **Checkpoint (you):** Delete the `.txt` from the previous checkpoint and run `audio` again. Expect: no "File exists" prompt, no ffmpeg run (the scratch `.mp3` is reused, so the Timeline shows Extract `reused`), a fresh faster-whisper run, and a new `.txt` with today's header.

### Phase 3: `audio summarize` and docs
- [x] 3.1 `xscriptSummarize` accepts `.txt`, and resolves an `.mp4` with `findExistingTranscript`. File: `cmd/audio_summarize.go`.
- [x] 3.2 Update the help text: `audioCmd.Long` (`cmd/audio.go`), `audioTranscribeCmd.Long` and the `transcribeAudio` doc comment (`cmd/audio_transcribe.go`), `audioExtractCmd.Long` (`cmd/audio_extract.go`) and `audioSummarizeCmd.Long` (`cmd/audio_summarize.go`). Also update `README.md` (the workflow diagram's `transcript (temporary)`, the intermediates paragraph, the `scratch-dir` table row) and the `scratch-dir` comment in `TODO-km-online-config.yaml`.
- [x] 3.3 `go build -o online.exe && go test ./...` passes.
- [x] **Checkpoint (you):** Run `online audio summarize "<the video from Phase 2>.mp4"`. Expect: it summarizes from the `.txt` beside the video. Also run `online audio --help` and check the wording says transcripts are kept beside the video.

## Test plan

### Automated (Claude)
- [x] `go test ./util/...`: `CopyFile` copies the file completely, leaves no `.partial` behind, and fails cleanly when the destination folder is missing.
- [x] `go test ./cmd/...`: the header renders exactly, round-trips through strip, and leaves headerless and ordinary-`#` text alone.
- [x] `go test ./cmd/...`: `findExistingTranscript` finds only a non-empty `.txt` and ignores a scratch `.text`. `preserveTranscript` copies the file, falls back to the scratch path when the copy fails, and never overwrites an existing `.txt`.
- [x] `go vet ./... && go build ./...` is clean.

### Manual (you)
- [x] A new video gets a `.txt` beside it, with the header (Phase 2 checkpoint).
- [x] Re-running skips extraction and transcription, and says so (Phase 2 checkpoint).
- [x] Deleting the `.txt` alone regenerates it, reusing the scratch `.mp3` if it's still there and without asking anything (Phase 2 checkpoint).
- [ ] Optional, for Q3: copy a video into a read-only folder and run `audio` on it. Expect the WARNING, the scratch transcript used, and a title and description produced anyway.
- [x] `audio summarize <video>.mp4` uses the preserved `.txt` (Phase 3 checkpoint).

## Deviations

Small additions made during implementation, reported at the first checkpoint and approved afterwards.

- **`extractAndTranscribe` split out of `processOneAudio`** (`cmd/audio.go`). The plan's Architecture had extraction, the stale-transcript delete, transcription and preservation all inline in the `else` branch. They moved into their own function, so `processOneAudio` reads as "use the `.txt`, or make one". Behaviour is unchanged.
- **An empty `.txt` beside the video is replaced** (`preserveTranscript`). The plan said an existing `.txt` is never overwritten. That still holds for one with content. An empty one is ignored by `findExistingTranscript`, so if it were also protected, every run would transcribe again without ever saving the result. Covered by `TestPreserve_ReplacesEmpty`.
- **A successful save prints `Saved transcript: <name>.txt`** (`preserveTranscript`). The plan only specified the warning for a failed copy. This line shows the operator where the transcript went, matching the `Using existing transcript:` line on reuse.
