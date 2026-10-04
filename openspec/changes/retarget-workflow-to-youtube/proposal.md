# Proposal: Retarget the Workflow to a YouTube-Backed Website

**Change ID:** `retarget-workflow-to-youtube`
**Created:** 2026-09-27
**Status:** Implementation Complete (pending validation)
**Implemented:** 2026-10-03

---

## Problem Statement

The `online` application exists to feed a **static** website. It reads a Google
Sheet, builds a complete HTML catalog (series list pages, per-series pages,
recent-message pages, booklet and resource pages, per-ministry stylesheets), and that
bundle is uploaded to S3. A second, largely independent pipeline (`online audio`)
extracts and publishes audio and transcripts to S3 so those pages have something to
link to.

The new website consumes none of that. It builds its media catalog at request time by
querying **YouTube** for playlists (series), videos (messages), and podcasts, and
possibly OneDrive for documents. Every HTML page this application generates is about to
become dead weight, and so is most of what the audio pipeline uploads.

Concretely, the following work is now unnecessary:

- **Static site generation.** `cmd/catalog.go` (714 lines) plus 14 templates and the
  static asset tree produce pages nobody will load.
- **Audio preservation.** With the "Sunday Service" podcast retired, the `.mp3`
  uploaded to `s3://wordoflife.mn.audio/{year}/` has no consumer. Audio is now purely
  an intermediate on the way to a transcript.
- **Transcript preservation.** Transcripts will not be searchable or visible. They are
  now a disposable input to title and description generation, which means they can also
  be produced far more cheaply. `CatalogMessage.HasTranscript()` currently shells out to
  `aws s3 ls` once per year from 2005 to the present just to decide whether to show a
  link — an expensive check for a feature that is going away.

Meanwhile a real gap opens. With YouTube as the system of record, every message needs
correctly-formatted YouTube metadata: a title of the form `name | speaker | date`, a
description carrying the summary and resource links, the right playlist, the right
position in that playlist, and the right privacy setting. Today the operator derives
all of that by hand from the spreadsheet while uploading.

**Who is affected:** the media operator (a single user, running this locally on
Windows after editing each week's video).

## Proposed Solution

Collapse two pipelines into one. The application stops being a website generator and
becomes a **message publishing assistant**: given an edited video file, it produces
everything needed to publish that message to YouTube.

### Target workflow

```
  edited .mp4 (local)
        │
        ├─(a)─ ffmpeg ──────────────► .mp3  (temp, deleted after use)
        │
        ├─(b)─ faster-whisper ──────► transcript (temp, deleted after use)
        │        cheapest sufficient model, text output only
        │
        ├─(c)─ LLM ─────────────────► suggested title + short description
        │
        ├─(d)─ Google Sheet lookup ─► series, track, ministry, visibility,
        │                             booklet and resource links
        │
        └─(e)─ assemble ────────────► UPLOAD PACKET printed to the console
                                        · title:       "Name | Speaker | Date"
                                        · description: summary + resource links
                                        · playlist:    series name
                                        · position:    track number
                                        · privacy:     derived from visibility
                                        · thumbnail:   local file path
```

Step (e) replaces what used to be a manual derivation. **Upload itself stays manual**
this round — the operator pastes the packet into YouTube Studio.

Because the destination is a human doing copy-paste, the packet's *presentation* is a
requirement, not a detail. Each field must be emitted as an independently
copyable block, clearly delimited, with no decoration inside the block that would be
copied along with the value. The existing `╭─── … ╰───` framing in `cmd/audio.go`
already does this correctly for audio URLs and is the pattern to follow: label outside
the fence, bare value inside.

The packet is also designed so that automating the upload later means feeding the same
struct to the YouTube Data API instead of to `fmt.Printf`.

### The Google Sheet's new role

The sheet stops being the published catalog and becomes the operator's **staging
area**. The tool still reads it, but only to answer "where does this message go?" —
series, track number, ministry, visibility, and the resource links that belong in the
description. After upload, YouTube holds the catalog and the website reads YouTube.

This is a deliberate overlap during transition: the sheet says where a message *should*
go; YouTube records where it *went*.

### Key components

| Component | Change |
|-----------|--------|
| `cmd/catalog.go`, `templates/` | Delete. |
| `cmd/audio*.go` | Rework into the single pipeline above; drop all S3 upload. |
| `cmd/publish.go` *(new)* | Assembles and prints the upload packet. |
| `catalog/` model | Keep `Ministry`, `View`, series + track, booklets/resources. Delete the HTML-rendering and transcript-hosting helpers. |
| `gclient/` | Keep sheet reading. Add lookup-by-message. |

## Scope

### In Scope

- Delete the static site generator: `cmd/catalog.go`, `cmd/catalog_test.go`, the whole
  `templates/` tree (including the orphaned `podcast.xml`), `getTemplateDir()` /
  `getTemplatePath()`, and the `otiai10/copy` dependency.
- Remove audio upload to S3 (`uploadAudioToS3`, `getAudioS3URL`, `getAudioHTTPURL`).
- Remove transcript upload to S3 (`uploadTranscriptionsToS3`, `xscriptInfo`).
- Remove transcript discovery: `HasTranscript()`, `LoadTranscriptsCache()`,
  `LoadTranscriptCacheForYear()`, `GetTranscriptURL()`, and the package-level
  transcript cache. This eliminates every `aws s3 ls` call.
- Remove podcast-only model support: `GetAudioSize()`, `CatalogMessage.Audio`,
  `HasAudio()`, and the `Audio` sheet column read.
- Remove HTML-rendering helpers from `OnlineResource`: `GetThumbnail()`, `GetIcon()`,
  `GetClassifier()`, `GetEmbeddedURL()`, `GetEmbeddedVideo()`.
- Remove web page-naming helpers: `GetViewID()`, `GetCatalogFileName()`,
  `GetCatalogFileNameForSeriList()`.
- Treat audio and transcripts as **temporary**: write to a scratch directory and delete
  them once the summary is generated.
- Downgrade transcription to the cheapest sufficient setting (text output only, no
  `.vtt`), with the model selectable by config.
- Add sheet lookup: given a video file (date + message type from the filename), find
  the matching message row and return its series, track, ministry, visibility, and
  resources. Disambiguate by track number when date + type matches more than one row.
- Add the upload packet: assemble and print the YouTube channel, title, description,
  playlist, position, privacy status, and thumbnail path, each as a copy-paste-ready
  block.
- Add channel routing: map `Ministry` to one of the three YouTube channels.
- Add the CORE playlist prefix convention, isolated in one function so it can change
  after negotiation with the website developer.
- Migrate the summarizer from OpenAI to Claude: replace `sashabaranov/go-openai` with
  `anthropics/anthropic-sdk-go`, and use structured outputs to guarantee the title and
  summary come back as valid JSON.
- Send the **full transcript** to the summarizer and delete the sampling machinery
  (`xscriptExtractSample`, `extractSampleFromMiddle`, `findPreviousSentenceStart`,
  `findStartOfSentence`, `findStartOfWord`, and `cmd/audio_summarize_test.go`).
- Add advisory platform-risk notes to the summarizer output: policy area, verbatim
  quote, and reasoning for each passage of concern. Advisory only — never blocking,
  never a score, never a platform recommendation. On for `faith-freedom` by default.
- Keep thumbnails as a **local file path** in the packet. No S3 upload.
- Keep documents (booklets, sermon notes) on public S3 so descriptions have linkable
  URLs.
- Update the `Makefile` and `README.md` to match the surviving commands.

### Out of Scope

- **Automated YouTube upload.** Explicitly deferred. Requires an OAuth user-consent
  flow (see Risks) and is the obvious next change.
- **Playlist and video creation/management via API.** Manual, as today.
- **OneDrive integration of any kind** — neither booklets nor the Excel migration.
  Booklets stay on S3 this round.
- **Migrating the spreadsheet to OneDrive Excel.**
- **Backfilling existing messages.** This changes the workflow for new messages only.
- **Anything about the new website itself.** This proposal covers only the tooling that
  feeds it.

## Impact Analysis

| Component | Change Required | Details |
|-----------|-----------------|---------|
| Data model (`catalog/`) | Yes | `Audio` field and all HTML/transcript helpers removed. `Ministry`, `View`, `SeriesReference`, `OnlineResource`, `DateOnly` and `validate.go` all survive unchanged. |
| Sheet ingest (`gclient/`) | Yes | Stops reading the `Audio` column. Gains a lookup-by-date/name entry point. Series and Messages tab reading is otherwise unchanged. |
| Commands (`cmd/`) | Yes | `catalog` deleted. `audio` reworked. `publish` added. `dump`, `check`, `peek` kept. |
| Templates | Yes | Deleted entirely. |
| External tools | Yes | `ffmpeg` and `faster-whisper` still required. `aws` drops out of the message pipeline. |
| S3 | Yes | No audio, no transcripts, no thumbnails, no website. Documents only. |
| Dependencies | Yes | `otiai10/copy` removed. `sashabaranov/go-openai` replaced by `anthropics/anthropic-sdk-go`. |
| Config | Yes | New keys for the scratch directory, transcription model, and summarization model. `openai-key` removed. `sheet-id` retained. |

## Architecture Considerations

**This is mostly a deletion.** Roughly 1,400 lines of Go plus 14 templates come out;
the additions are a sheet-lookup function and a metadata-assembly function. The model
layer survives nearly intact, which is what makes the deletion safe — `validate.go`
checks catalog consistency (series exist, track numbers sequential, names unique), and
none of that is web-specific.

**The upload packet is the new seam.** Defining it as a struct now, and rendering it to
the console, means the deferred YouTube API work becomes "serialize this struct
differently" rather than a redesign.

### Visibility collapses to two states

| `View` | YouTube privacy | Note |
|--------|-----------------|------|
| `public` | `public` | |
| `partner` | `unlisted` | |
| `private` | `unlisted` | Matches current practice. |
| `raw` | *never enters the pipeline* | See below. |

`partner` and `private` become indistinguishable on YouTube. That is consistent with
what the current site already does in practice, so it is not a regression — but it does
mean the model's four-valued `View` carries a distinction YouTube cannot express. The
model keeps it (the sheet still uses it, and the website may key on something else);
the packet simply flattens it.

**`raw` never reaches this tool at all.** Raw footage is the livestream, which is
already on YouTube as `private` from the moment it streams. It is never downloaded and
never edited, so there is nothing to upload — the sheet just points at the existing
stream URL. The pipeline starts from an *edited* local video, so encountering a `raw`
message means something is wrong upstream, and the tool should say so rather than
attempt a packet.

### Ministries map to channels and playlist naming

There are three YouTube channels, and `Ministry` determines which one a message belongs
to:

| Channel | Ministries it hosts |
|---------|---------------------|
| **WOL** | `wol`, `core` (+ all CORE sub-ministries), `ask-pastor` |
| **Faith & Freedom** | `faith-freedom` |
| **TBO** | `tbo` |

Within the WOL channel, three ministries share one channel and must be told apart by
playlist name alone:

- **Ask the Pastor** is already unambiguous — every such message lives in the single
  `Ask the Pastor` playlist.
- **WOL and CORE are not.** Both use the bare series title as the playlist name, so
  nothing in YouTube distinguishes a CORE series from a WOL series.

**Proposed convention: prefix every CORE playlist with `CORE: `.** This is the piece
that needs agreement with the website developer, since it only works if they can filter
playlists on that prefix. The convention is implemented as a single function so it can
be changed in one place if the negotiation lands somewhere else.

Note that `Ministry.Description()` already returns `"CORE: Health Matters"` for the
sub-ministries but `"C.O.R.E."` for the parent — so the prefix logic cannot simply
reuse `Description()`.

**Faith & Freedom also has a Rumble channel** for videos that would violate YouTube's
terms. Which destination a given F&F message takes is a human judgment call made per
message, so the packet names the YouTube channel and the operator overrides when
needed. Automating that decision is out of scope.

### The summarizer moves from OpenAI to Claude

The title/summary generator currently calls GPT-3.5-turbo through
`sashabaranov/go-openai`, and has to defend itself: when the model returns malformed
JSON, [`generateMessageSummary()`](../../../cmd/audio_summarize.go) falls back to
scanning lines for `"title":` and `"summary":` and trimming quotes by hand.

Claude's **structured outputs** (`output_config.format`) constrain the response to a
schema, so that entire fallback is deleted rather than ported. Net effect on the file
is subtractive.

| | Now | After |
|---|---|---|
| SDK | `github.com/sashabaranov/go-openai` | `github.com/anthropics/anthropic-sdk-go` |
| Model | `gpt-3.5-turbo` | `claude-opus-5` (see below) |
| JSON safety | hand-rolled line-scanning fallback | schema-constrained, no fallback needed |
| Credential | `openai-key` config value | `ANTHROPIC_API_KEY`, or an `ant auth login` profile the SDK picks up with no config at all |

**The transcript sampler goes away.** Today the summarizer sends
`xscriptExtractSample(path, 12_000)` — about 12,000 tokens (~48,000 characters) taken
from the *middle* of the transcript, assembled by four helper functions that hunt
backwards for sentence boundaries. All of that exists to fit inside GPT-3.5-turbo's
context window. With a 1M-token context, the whole transcript goes in.

This is not only a simplification — it fixes a live defect. Messages typically run
70–120 minutes. At normal preaching pace that is roughly **11,000–24,000 tokens**, so
the 12,000-token window was clipping **nearly every message**, and on a two-hour message
it was discarding about half the content.

What it discarded matters. `extractSampleFromMiddle()` starts at `len(s)/2` and works
outward, so what gets dropped is the opening and the closing — the two highest-signal
passages in a sermon. The opening states the subject; the closing carries the call to
action. The sampler has been systematically withholding the best evidence for a title
and summary, on every message, for as long as it has existed.

Sending the full transcript should measurably improve the output, not merely preserve
it. This is the change most likely to show a visible difference in the generated titles.

Removed with it: `xscriptExtractSample()`, `extractSampleFromMiddle()`,
`findPreviousSentenceStart()`, `findStartOfSentence()`, `findStartOfWord()`, and the
whole of `cmd/audio_summarize_test.go` — 130 lines of tests that exist only to verify
sentence-boundary hunting.

### Platform risk notes (advisory only)

Faith & Freedom sometimes produces content that would not survive on YouTube, and today
that judgment is made by the operator from memory of having watched the message. Since
the full transcript is already being sent to Claude, the same call can return
**observations** about passages a YouTube reviewer might act on.

**This is deliberately not a risk score.** It returns, for each passage of concern: the
policy area it touches, a verbatim quote, and a one-line explanation. The operator reads
the quote in context and decides. Nothing is blocked, nothing is scored, and no
alternative platform is recommended — routing to Rumble stays a human call.

The design is shaped by five limits that are real and should not be papered over:

| Limit | Consequence for the design |
|---|---|
| Published policy ≠ actual enforcement, which is inconsistent | Never assert that something *will* be actioned; report only that a reviewer might look |
| Platform policies drift, and the model's knowledge is dated | Require an explanation with every flag, so a stale assumption is visible rather than silent. Do not hard-code a rulebook that rots. |
| Transcripts lose tone, framing, and visuals — a quoted-and-refuted claim reads like an asserted one | Always quote verbatim so the operator can check the surrounding context |
| A cheap transcription model garbles proper nouns, numbers, and specific claims — exactly what a judgment turns on | Must be able to say "the transcript is too unreliable here to judge" instead of guessing |
| The content is a pastor's religious and political speech; over-flagging it is quiet editorializing | Report observations, never verdicts. Precision matters more than recall: a noisy flagger gets ignored, which is worse than none. |

**Enabled for `faith-freedom` by default**, available elsewhere by flag. Weekly
policy-risk notes on ordinary WOL preaching would be noise, and noise is how this
feature dies.

**Implementation:** extend the existing summarization schema rather than making a second
call. The transcript is the dominant cost and it is already in context; a second pass
would double input tokens for no benefit. If the combined prompt degrades the title or
summary, splitting into two calls is the fallback — decide it with evidence in Phase 3.

**On model choice:** the default is `claude-opus-5`. At full transcript that is roughly
11,000–24,000 input tokens for a six-word title and three sentences out, a handful of
times a week — on the order of **$0.06–0.12 per message** at Opus pricing, or a fifth of
that on `claude-haiku-4-5`. Either is negligible against the hours of editing that
precede it, so pick on quality rather than cost. The model goes in a config key, so
switching is a one-line change with no rebuild. Worth trying Haiku on two or three real
messages before settling.

The prompt itself carries over nearly unchanged. It is already well-shaped: it names
the speaker, bounds the title at six words, and asks for three sentences in a casual
voice.

**The `catalog` package name becomes slightly wrong** once no catalog is generated. Not
worth renaming this round — the types are still the right types.

## Success Criteria

- [ ] `online audio <video.mp4>` produces a complete upload packet and leaves no `.mp3`
      or transcript behind.
- [ ] Every packet field is emitted as an independently copy-paste-ready block, with no
      decoration inside the copyable region.
- [ ] No S3 upload occurs anywhere in the message pipeline.
- [ ] No `aws s3 ls` call occurs anywhere in the codebase.
- [ ] The packet's title matches `Name | Speaker | Mon D, YYYY` exactly.
- [ ] The packet's description contains the generated summary and one link per resource
      attached to the message or its series.
- [ ] Playlist name and position match the message's series and track in the sheet.
- [ ] The packet names the correct channel for every ministry, and CORE playlists carry
      the agreed prefix.
- [ ] A `raw`-visibility message produces a refusal explaining that the livestream is
      already on YouTube.
- [ ] Summarization runs against Claude with structured outputs, and the hand-rolled
      JSON recovery path is gone.
- [ ] The summarizer receives the complete transcript; no sampling function remains.
- [ ] No OpenAI dependency remains in `go.mod`.
- [ ] `templates/` no longer exists, and the build has no unused dependencies.
- [ ] End-to-end wall-clock time per message is no worse than today (transcription is
      strictly cheaper; two S3 uploads are gone).
- [ ] `make test` and `go vet ./...` clean.

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| The cheapest transcription model is too poor for usable titles/summaries | Med | Low | Make the model a config key, not a constant. Compare `tiny.en` / `base.en` / `small` on 2–3 real messages during Phase 2 and pick from evidence. Partly self-mitigating: the summarizer now sees the *whole* transcript rather than a middle slice, so there is more signal to work with even if each word is less reliable. The speaker overrides the title anyway. |
| Deleting audio/transcripts loses something we later want | Low | High | Deletion is a separate, last step gated by a `--keep-intermediates` flag. Nothing is deleted until the summary succeeds. |
| Deleted HTML pages are still linked from somewhere | Med | Med | The old site stays on S3 untouched; this change stops *regenerating* it, it does not delete what is published. Cut over only when the new site is live. |
| The `CORE: ` playlist prefix is rejected by the website developer | Med | Low | Isolate the convention in one function. If they can only filter on something else (a description marker, a separate channel, playlist IDs enumerated by hand), only that function changes. Agree the convention before any CORE series is published under it. |
| Existing CORE playlists are already named without the prefix | High | Med | Renaming published playlists is a manual YouTube Studio operation outside this tool. Decide whether to rename retroactively or let the convention apply only to new series — and tell the website developer which, since it determines whether they can rely on the prefix alone. |
| WOL and CORE stay indistinguishable | Med | High | This is the one genuinely unsolved problem in the new architecture. If the prefix convention fails, the fallback is a separate CORE channel, which is a much larger change. Settle it early. |
| YouTube upload quota bites during any backfill | Low | Med | Out of scope this round, but note now: upload costs 1,600 units against a 10,000/day default — about 6 videos per day. |
| Service-account credentials can never upload to YouTube | High | Med | Already true and unavoidable: the YouTube Data API rejects service accounts for channel uploads. When automation lands it needs an OAuth desktop flow with a cached refresh token. Deferring upload this round sidesteps it entirely. |
| Claude summaries differ in voice from the GPT-3.5 ones | Med | Low | Compare on two or three real messages during Phase 2. The prompt is unchanged, and the speaker overrides the title anyway; only the summary voice is at stake. |
| Risk notes over-flag ordinary preaching | Med | Med | Precision over recall, enforced by task 2.23: validate on real messages and drop the feature if it cries wolf. Restricting it to `faith-freedom` by default limits the blast radius. |
| Risk notes are trusted as a compliance verdict | Low | High | The output is shaped to prevent this: no score, no recommendation, always a quote plus reasoning. It informs a human decision and never replaces one. This is not legal or policy advice, and the proposal says so. |
| Platform policy knowledge goes stale | High | Low | Every note carries its reasoning, so a judgment resting on an outdated understanding is visible rather than silent. The prompt describes categories of concern rather than encoding a rulebook. |
| `claude-opus-5` is more expensive per message than GPT-3.5-turbo | High | Low | True, but the volume is a handful of messages per week on ~12K characters of input. If it matters, `claude-haiku-4-5` is a config change. Measure before optimizing. |

## Resolved During Review

- **Privacy.** `partner` and `private` both map to `unlisted`, matching current
  practice. `raw` never enters the pipeline — the livestream is already on YouTube as
  `private` and the sheet points at it.
- **Lookup key.** Date + message type, since a Prayer and a Message commonly share a
  date and speaker. Track number is an optional tie-breaker: ignored when date + type
  is unique, required when it is not.
- **Channels.** Three — WOL (hosting `wol`, `core`, `ask-pastor`), Faith & Freedom, and
  TBO.
- **Summarizer.** Moving from OpenAI to Claude, with structured outputs replacing the
  hand-rolled JSON recovery.
- **Models (measured 2026-10-03).** Transcription uses `small` - swept eight models
  over a real 80 minute service; `tiny.en` writes an empty transcript while reporting
  success and must never be used. Summarization uses `claude-haiku-4-5`, steadier than
  `claude-opus-5` and about a fifth the cost. Full evidence is in `tasks.md` and in the
  `defaultWhisperModel` comment.

## Deferred to a Later Change

- **Booklets via OneDrive.** The intended shape is a OneDrive directory mirrored to
  this machine, with that local mirror synced onward to S3 as needed. This keeps public
  S3 URLs for YouTube descriptions while making OneDrive the authoring surface.
  Recorded here so the next change has a starting point; nothing in this round depends
  on it.
- **Automated YouTube upload**, including the OAuth desktop flow.
- **Playlist creation and management via the API.**

## Open Questions

These do not block starting, but should be settled before the new site cuts over:

1. **The CORE playlist prefix.** Needs agreement with the website developer on what
   they can actually filter on. Also needs a decision on whether existing CORE
   playlists get renamed retroactively.
2. **Do the risk notes earn their place?** Task 2.23 validates them against real F&F
   messages. If precision is poor — flagging ordinary preaching — the feature should be
   dropped rather than tuned indefinitely. A notice the operator learns to skip is
   worse than no notice.
