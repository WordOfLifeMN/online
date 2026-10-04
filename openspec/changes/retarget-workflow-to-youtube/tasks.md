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
- [x] 2.9 Evaluate transcription models on real material; record the chosen default
      and the evidence in this file. **Done 2026-10-03** - swept eight models over a
      full 80 minute service; chose `small`. See "Transcription model chosen" below.
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
- [x] 2.18 Compare `claude-opus-5` against `claude-haiku-4-5` at full transcript.
      **Done 2026-10-03** - two runs each on a real service; chose `claude-haiku-4-5`.
      See "Summarisation model chosen" below.

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
- [ ] A full run over one real video leaves only that run's files in the scratch
      directory (intermediates are kept for 24 hours deliberately, see 2.7)
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
- [x] 4.4 Verify `dump`, `check`, and `peek` still work against the live sheet.
      **Done 2026-10-03** - see "Verified against the live spreadsheet" below.
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

## Transcription model chosen (2026-10-03)

- [x] 2.9  Swept eight models over a real 80 minute service on CUDA. Times and
      content-word error rate against large-v3:
      tiny.en 2m28s EMPTY OUTPUT | base.en 2m04s 8.5% | small 3m06s 7.0% |
      small.en 4m04s 6.7% | distil-large-v3 6m27s 6.9% | medium 6m40s 6.1% |
      large-v3-turbo 8m09s 5.6% | large-v3 17m59s reference

      Nothing met the 5% bar, but the bar was the wrong test: summarising each
      transcript produced the same title and the same three sentences all the way
      down to base.en. The error rate counts fillers, contractions and occasional
      mishearings, none of which survive summarisation.

      **small stays the default** - bundled, ~3 minutes per service, with quality
      headroom for a guest speaker or worse audio. **tiny.en must never be used**:
      it wrote an empty transcript while reporting success on both a 67 second
      excerpt and the full service.

## Summarisation model chosen (2026-10-03)

- [x] 2.18 `claude-opus-5` vs `claude-haiku-4-5`, two runs each on the same real
      transcript. Both captured the same content and neither would mislead a viewer.
      Opus wrote livelier copy but ran ~35% longer, changed its title every run, and
      one run in two emitted an undecoded unicode escape and a stray newline into the
      description. **claude-haiku-4-5 is the default** - steadier, and about a fifth
      the cost. A sanitiser now cleans the title and summary regardless of model.

## Still outstanding

Everything in the proposal is implemented. What remains needs real media, a person
to judge the result, or someone outside this repository.

### Needs a real run

These are tracked by their phase entries above; tick them there, not here.

- **2.23** Validate the advisory risk notes on real Faith & Freedom messages - at
  least one you would publish and one you would not. **Drop the feature rather than
  tune it indefinitely if it flags ordinary preaching.** A notice the operator learns
  to skip is worse than no notice.
- **2.24** Confirm the combined prompt has not degraded title or summary quality.
  Partly answered: runs with risk notes *disabled* produce clean, on-target output.
  The Faith & Freedom path with `--risk-notes` on is still untested.
- **4.6** End-to-end run over a full multi-service session. The interleaved prompts
  are verified, but the two-video processing loop has not run start to finish.
  Afterwards, confirm `~/.wolm/scratch` holds only that run's files.

```
online audio "2026-03-08-v Some Message.mp4" --verbose
```

### Needs someone else

- [ ] Agree the `CORE: ` playlist prefix with the website developer - specifically
      whether they can filter playlists on it, and whether existing CORE playlists
      get renamed retroactively. **Blocks publishing any CORE series**, since
      republishing under a changed convention means manual work in YouTube Studio.
      The convention lives in `getPlaylistName` so a different answer costs one
      function.

### Housekeeping

- [ ] Revoke the old OpenAI API key at platform.openai.com. Nothing uses it since
      the move to Claude, it sat in `~/.wolm/online-config.yaml` from April 2024
      until 2026-10-03, and it was displayed in a terminal session on that date.
- [ ] Open the pull request for this branch (`gh` is not installed on this machine;
      a prepared description was written to the session scratchpad).
