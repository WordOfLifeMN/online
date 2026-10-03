# Implementation Tasks: Retarget the Workflow to a YouTube-Backed Website

**Change ID:** `retarget-workflow-to-youtube`

Phases are ordered so the codebase compiles and tests pass at every phase boundary.
Deletion comes first because it removes the only callers of the helpers deleted in
Phase 2 — doing it the other way round leaves the tree broken in between.

---

## Phase 1: Remove the static site generator

- [x] 1.1 Delete `cmd/catalog.go` and `cmd/catalog_test.go`
- [x] 1.2 Delete the `templates/` tree, including `podcast.xml` and `templates/static/`
- [x] 1.3 Remove `getTemplateDir()` and `getTemplatePath()` from `cmd/root.go`
- [x] 1.4 Remove the `template-dir` config key and any `~/.wolm/online-config.yaml`
      reference to it
- [x] 1.5 Remove HTML-rendering helpers from `catalog/resource.go`: `GetThumbnail()`,
      `GetIcon()`, `GetClassifier()`, `GetEmbeddedURL()`, `GetEmbeddedVideo()`, and the
      unused `thumbnail` / `classifier` fields
- [x] 1.6 Remove web page-naming helpers: `CatalogSeri.GetViewID()`,
      `CatalogSeri.GetCatalogFileName()`
- [x] 1.7 Drop the `otiai10/copy` dependency; `go mod tidy`
- [x] 1.8 Prune the corresponding tests in `catalog/resource_test.go` and
      `catalog/catalogseri_test.go`

**Quality Gate:**
- [x] `go build ./...` succeeds
- [x] `go vet ./...` clean
- [x] `make test` passes
- [x] `grep -r "html/template" --include=*.go` returns nothing

---

## Phase 2: Stop preserving audio and transcripts

- [x] 2.1 Remove `uploadAudioToS3()`, `getAudioS3URL()`, `getAudioHTTPURL()` from
      `cmd/audio_extract.go`
- [x] 2.2 Remove `uploadTranscriptionsToS3()` and the `xscriptInfo` type from
      `cmd/audio_transcribe.go`
- [x] 2.3 Remove transcript discovery from `catalog/catalogmessage.go`:
      `HasTranscript()`, `LoadTranscriptsCache()`, `LoadTranscriptCacheForYear()`,
      `GetTranscriptURL()`, and the `transcriptCache` / `transcriptCacheOnce` /
      `transcriptCacheMu` package vars
- [x] 2.4 Remove podcast-only model support: `GetAudioSize()`, `HasAudio()`, and the
      `CatalogMessage.Audio` field
- [x] 2.5 Stop reading the `Audio` column in `gclient/catalog.go` (remove `msgAudio`
      and its entry in `requiredMessageColumns`)
- [x] 2.6 Route intermediate `.mp3` and transcript files to a scratch directory
      (new `scratch-dir` config key, defaulting under `~/.wolm/`) instead of writing
      beside the source video
- [x] 2.7 Delete intermediates after the summary succeeds; add `--keep-intermediates`
      to suppress deletion. Never delete on a failed summary.
- [x] 2.8 Reduce transcription cost: text output only (drop `.vtt`), model selectable
      via a new `whisper-model` config key
- [ ] 2.9 Evaluate `tiny.en` vs `base.en` vs `small` on 2–3 real messages; record the
      chosen default and the evidence in this file
- [x] 2.10 Update `MessageInfo` to drop `AudioURL`, `TranscriptURLs`, `UploadTime`,
      `UploadTranscriptTime`, and the corresponding lines in `printMessageInfo()`

### Summarizer: OpenAI → Claude

- [x] 2.11 Add `github.com/anthropics/anthropic-sdk-go`; remove
      `github.com/sashabaranov/go-openai`
- [x] 2.12 Rewrite `generateMessageSummary()` against the Anthropic SDK, keeping the
      existing prompt (speaker name, six-word title cap, three casual sentences)
- [x] 2.13 Use structured outputs (`output_config.format`) for the `{title, summary}`
      schema
- [x] 2.14 **Delete** the hand-rolled JSON recovery — the line-scanning fallback for
      `"title":` / `"summary":` and the quote trimming. Structured outputs make it dead
      code; do not port it.
- [x] 2.15 Replace the `openai-key` config key with `anthropic-model` (default
      `claude-opus-5`). Do not add an API-key config key — let the SDK resolve
      `ANTHROPIC_API_KEY` or an `ant auth login` profile.
- [x] 2.16 Send the **full transcript** instead of a sample: read the transcript file
      and pass it whole
- [x] 2.17 Delete the sampling machinery — `xscriptExtractSample()`,
      `extractSampleFromMiddle()`, `findPreviousSentenceStart()`,
      `findStartOfSentence()`, `findStartOfWord()` — and delete
      `cmd/audio_summarize_test.go`, which tests nothing else
- [ ] 2.18 Compare `claude-opus-5` against `claude-haiku-4-5` on 2–3 real messages, at
      full transcript; record the chosen default and the reasoning here

### Platform risk notes (advisory)

- [x] 2.19 Extend the summarization schema with risk notes: `{policy_area, quote,
      reasoning}` per entry, plus an explicit "nothing flagged" state
- [x] 2.20 Extend the prompt to request observations rather than verdicts. Describe
      categories of concern; do not encode a policy rulebook that will go stale.
      Require verbatim quotes and an explicit "transcript unreliable here" option.
- [x] 2.21 Gate on ministry: on for `faith-freedom`, off elsewhere, with a flag to
      enable
- [x] 2.22 Render notes in the packet as clearly advisory, separate from the
      copy-paste-ready fields — they are never pasted into YouTube
- [ ] 2.23 Validate on real F&F messages, including at least one you would *not* upload
      to YouTube and one you would. Check precision over recall — if it flags ordinary
      preaching, tighten the prompt or drop the feature.
- [ ] 2.24 Confirm the combined prompt has not degraded title or summary quality
      (compare against 2.18 output). If it has, split into a second call.

**Quality Gate:**
- [x] `grep -rn "aws s3" --include=*.go` returns nothing
- [x] `grep -rn "wordoflife.mn.audio" --include=*.go` returns nothing
- [x] `grep -rn "openai" --include=*.go go.mod` returns nothing
- [x] `grep -rn "ExtractSample\|findStartOf" --include=*.go` returns nothing
- [ ] A full run over one real video leaves no `.mp3` or transcript on disk
- [x] `make test` passes

---

## Phase 3: Sheet lookup and the upload packet

- [x] 3.1 Define the `UploadPacket` struct (channel, title, description, playlist,
      position, privacy, thumbnail path) in a new `cmd/publish.go`
- [x] 3.2 Implement `View` → YouTube privacy mapping: `public`→`public`,
      `partner`/`private`→`unlisted`, `raw`→refuse with the livestream explanation
- [x] 3.3 Implement `Ministry` → channel mapping (WOL / Faith & Freedom / TBO)
- [x] 3.4 Implement playlist naming in **one** function: bare series title, `CORE: `
      prefix for `core` and every CORE sub-ministry, fixed `Ask the Pastor` for
      `ask-pastor`. Do not derive the prefix from `Ministry.Description()` — it returns
      `"C.O.R.E."` for the parent and `"CORE: Health Matters"` for sub-ministries.
- [x] 3.5 Infer message type from the filename (`p` adjacent to the date → `prayer`,
      otherwise `message`); add an explicit `--type` flag to override
- [x] 3.6 Add sheet lookup in `gclient/`: given a date and type, return the matching
      `CatalogMessage` plus its `CatalogSeri`
- [x] 3.7 Add the track-number tie-breaker: ignored when date + type is unique,
      required when it is not. On unresolved ambiguity, list candidates and their
      tracks rather than guessing.
- [x] 3.8 Implement title assembly: `Name | Speaker | Mon D, YYYY`
- [x] 3.9 Implement description assembly: generated summary, then one link per resource
      from the message and its series
- [x] 3.10 Resolve the thumbnail to a local file path (no upload)
- [x] 3.11 Implement copy-paste-ready rendering: each field in its own delimited block,
      label outside the fence and bare value inside, following the existing
      `╭─── … ╰───` pattern in `cmd/audio.go`
- [x] 3.12 Wire the packet into the end of the `audio` pipeline
- [x] 3.13 Tests: title formatting; privacy mapping for all four `View` values; channel
      mapping for every `Ministry`; CORE prefix including sub-ministries; description
      assembly with zero/one/many resources; type inference from filenames; lookup with
      a prayer and message sharing a date; tie-break resolved, unresolved, and
      track-matches-nothing

**Quality Gate:**
- [x] `make test` passes
- [x] A `raw` message produces a refusal, not a packet
- [x] A prayer and a message on the same date resolve to different rows
- [ ] Manual check: every block pastes into YouTube Studio without editing

---

## Phase 4: Integration and cleanup

- [x] 4.1 Update the `Makefile`: remove `dryrun-catalog`, `win-local`,
      `win-test-local`, `run-refresh`
- [x] 4.2 Update `README.md` — it currently describes generating a static website
- [x] 4.3 Update `cmd/root.go` long description (still claims "Supports generating a
      RSS podcast as well as a HTML static website")
- [ ] 4.4 Verify `dump`, `check`, and `peek` still work against the live sheet
- [x] 4.5 Refresh `testdata/small-catalog.json` if the `Audio` field removal
      invalidates it
- [ ] 4.6 End-to-end run on one real message, timed against the current workflow

**Quality Gate:**
- [x] `make test` passes
- [x] `go vet ./...` clean
- [x] `make build` succeeds
- [ ] Every Success Criterion in `proposal.md` verified

---

## Notes

- **Do not delete the published S3 website.** This change stops regenerating it. What
  is already live stays live until the new site cuts over.
- **Agree the `CORE: ` prefix with the website developer before publishing any CORE
  series under it** (Open Question 1). Task 3.4 isolates the convention so a different
  answer costs one function, but republishing under a changed convention costs manual
  YouTube Studio work.
- Phase 3 is the only phase that adds significant code. Phases 1–2 are independently
  valuable and leave the tree in a working state.

## Completion Checklist

- [ ] All phases complete
- [ ] All quality gates passed
- [ ] Open Questions in `proposal.md` either answered or explicitly carried forward
- [ ] Ready for `/openspec-archive`

---

## Verified against the live spreadsheet (2026-10-03)

- [x] 4.4  `check` reports valid, `peek` reads all tabs, `dump` returns 2677 messages
      and 346 series with zero `audio` keys and 2677 `video` keys intact
- [x] Google Sheets scope narrowed to read-only and confirmed still able to read

Lookup ambiguity was measured here rather than assumed: for 2024+, 134 of 494
messages sit in an ambiguous (date, type) group and only 21 of the 60 groups have
distinct non-zero tracks. The track tie-breaker was therefore replaced with an
interactive chooser - see commit b7fa4b4.

## Blocked: needs real media, credentials, and spend

All code is implemented. These remaining tasks cannot be done from here - they need
real message videos, live credentials, and API calls that cost money. They are
evaluation and validation, not implementation.

- [ ] 2.9  Evaluate `tiny.en` vs `base.en` vs `small` on 2-3 real messages
- [ ] 2.18 Compare `claude-opus-5` against `claude-haiku-4-5` at full transcript
- [ ] 2.23 Validate risk notes on real F&F messages (one you would publish, one you
      would not). **Drop the feature if it flags ordinary preaching.**
- [ ] 2.24 Confirm the combined prompt has not degraded title/summary quality
- [ ] 4.6  End-to-end run on one real message, timed against the old workflow

Defaults chosen pending measurement: `whisper-model: tiny.en`,
`anthropic-model: claude-opus-5`. Both are single config keys - change without a
rebuild once you have evidence.

To run the end-to-end check:

```
online audio "2026-03-08-v Some Message.mp4" --verbose
```

Expect: an upload packet printed, and nothing left in `~/.wolm/scratch`.
