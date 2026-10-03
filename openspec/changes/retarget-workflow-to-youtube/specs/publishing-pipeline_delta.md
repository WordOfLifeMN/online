# Delta: Publishing Pipeline

**Change ID:** `retarget-workflow-to-youtube`
**Affects:** `cmd/audio.go`, `cmd/audio_extract.go`, `cmd/audio_transcribe.go`, `cmd/audio_summarize.go`, `cmd/publish.go` (new), `cmd/root.go`

---

## ADDED

### Requirement: Upload Packet

After processing a video, the tool produces an **upload packet**: the complete set of
metadata needed to publish that message to YouTube by hand. The packet carries channel,
title, description, playlist name, position in playlist, privacy status, and thumbnail
file path.

#### Scenario: Packet produced for a public message
- GIVEN an edited video for a message that exists in the sheet with ministry `wol`,
  visibility `public`, series "Walking in Faith", and track 3
- WHEN the operator runs `online audio <video.mp4>`
- THEN a packet is printed naming the WOL channel, with title
  `Name | Speaker | Mon D, YYYY`, the generated summary and resource links as the
  description, playlist "Walking in Faith", position 3, privacy `public`, and the local
  path to the thumbnail

#### Scenario: Message not found in the sheet
- GIVEN a video whose date and speaker match no row in the sheet
- WHEN the pipeline reaches packet assembly
- THEN the tool reports which date and speaker it searched for, and produces the title
  and description it was able to generate without the sheet-derived fields

---

### Requirement: Message Type Inferred From the Filename

Because the sheet lookup keys on date **and type**, the type must be derived from the
video file. The existing filename conventions already encode it: a `p` adjacent to the
date marks a prayer (the patterns `-MM-DD-pV `, `-MM-DDp `, and `-MM-DD-Vp ` are
already recognized by `getSpeakerFromFileName()`). Anything else is a message.

#### Scenario: Prayer inferred
- GIVEN a video named `2026-03-08-pv Opening Prayer.mp4`
- WHEN the type is inferred
- THEN it is `prayer`

#### Scenario: Message inferred
- GIVEN a video named `2026-03-08-v Walking in Faith.mp4`
- WHEN the type is inferred
- THEN it is `message`

#### Scenario: Operator overrides the inference
- GIVEN a video whose filename does not follow the convention, or which is one of the
  less common types (`testimony`, `training`, `song`, `special-event`, `word`,
  `ministry-time`)
- WHEN the operator passes an explicit type flag
- THEN that type is used and no inference is attempted

---

### Requirement: Copy-Paste-Ready Output

Because upload is manual, every packet field is emitted so it can be selected and
copied in one gesture. Each field occupies its own delimited block with the label
*outside* the delimiters and the bare value inside. No decoration, prefix, or
indentation appears inside the copyable region.

#### Scenario: Copying a multi-line description
- GIVEN a packet whose description spans several lines and contains resource links
- WHEN the operator selects the text between the block delimiters
- THEN the selection contains exactly the description text, with no border
  characters, labels, or leading whitespace, and pastes into YouTube Studio unedited

---

### Requirement: Visibility Maps to YouTube Privacy

The catalog's four-valued `View` is flattened onto YouTube's privacy states. Only two
values are reachable, because `partner` and `private` are not distinguishable on
YouTube — which matches what the current site does in practice.

| `View` | YouTube privacy |
|--------|-----------------|
| `public` | `public` |
| `partner` | `unlisted` |
| `private` | `unlisted` |
| `raw` | *refuse — never enters the pipeline* |

#### Scenario: Partner message
- GIVEN a message with visibility `partner`
- WHEN the packet is assembled
- THEN the privacy field reads `unlisted`

#### Scenario: Private message
- GIVEN a message with visibility `private`
- WHEN the packet is assembled
- THEN the privacy field reads `unlisted`, the same as a partner message

#### Scenario: Raw footage
- GIVEN a message with visibility `raw`
- WHEN the packet is assembled
- THEN the tool refuses, explaining that raw footage is the livestream, is already on
  YouTube as `private`, and should be referenced from the sheet rather than uploaded

---

### Requirement: Ministry Determines Channel and Playlist Name

Three YouTube channels exist. `Ministry` decides which one a message belongs to, and —
within the shared WOL channel — how its playlist must be named so the website can tell
the ministries apart.

| Ministry | Channel | Playlist name |
|----------|---------|---------------|
| `wol` | WOL | series title, unprefixed |
| `core` and all CORE sub-ministries | WOL | series title, prefixed `CORE: ` |
| `ask-pastor` | WOL | `Ask the Pastor` |
| `faith-freedom` | Faith & Freedom | series title |
| `tbo` | TBO | series title |

The `CORE: ` prefix is a convention invented to make CORE series distinguishable from
WOL series on a shared channel. It is implemented in one function so it can be changed
in one place if the website developer needs a different marker.

#### Scenario: WOL series
- GIVEN a message with ministry `wol` in series "Walking in Faith"
- WHEN the packet is assembled
- THEN the channel is WOL and the playlist is `Walking in Faith`

#### Scenario: CORE series gets the prefix
- GIVEN a message with ministry `core` in series "Boundaries"
- WHEN the packet is assembled
- THEN the channel is WOL and the playlist is `CORE: Boundaries`

#### Scenario: CORE sub-ministry gets the same prefix
- GIVEN a message with ministry `core_health` in series "Eating Well"
- WHEN the packet is assembled
- THEN the playlist is `CORE: Eating Well` — the prefix is derived from the ministry
  family, not from `Ministry.Description()`, which returns `"CORE: Health Matters"` for
  sub-ministries but `"C.O.R.E."` for the parent

#### Scenario: Ask the Pastor has a fixed playlist
- GIVEN a message with ministry `ask-pastor`
- WHEN the packet is assembled
- THEN the playlist is `Ask the Pastor` regardless of any series the message names

#### Scenario: Non-WOL ministries route to their own channels
- GIVEN a message with ministry `faith-freedom`, and another with `tbo`
- WHEN each packet is assembled
- THEN the first names the Faith & Freedom channel and the second names the TBO channel

---

### Requirement: Intermediates Are Temporary

Extracted audio and generated transcripts are working files. They are written to a
scratch directory rather than beside the source video, and deleted once they have
served their purpose.

#### Scenario: Successful run cleans up
- GIVEN a video processed end to end
- WHEN the summary has been generated successfully
- THEN the `.mp3` and the transcript are deleted, and the source video is untouched

#### Scenario: Failed summary preserves evidence
- GIVEN a video whose summary generation fails
- WHEN the pipeline exits
- THEN the `.mp3` and transcript remain on disk so the step can be retried without
  re-extracting and re-transcribing

#### Scenario: Operator keeps intermediates deliberately
- GIVEN the operator passes `--keep-intermediates`
- WHEN the pipeline completes successfully
- THEN no intermediate file is deleted

---

## MODIFIED

### Requirement: Audio Extraction

Audio is extracted from the video as an intermediate only.

#### Scenario: Extract audio
- GIVEN an `.mp4` video file
- WHEN the pipeline runs
- THEN `ffmpeg` extracts an `.mp3` into the scratch directory, applying the existing
  per-ministry trim lengths, and **no upload to S3 occurs**

---

### Requirement: Transcription

Transcription exists only to feed title and description generation, so it runs at the
cheapest setting that still yields usable output.

#### Scenario: Transcribe for summarization
- GIVEN an extracted `.mp3`
- WHEN the pipeline transcribes it
- THEN faster-whisper produces text output only — no `.vtt`, `.srt`, `.tsv`, or `.json`
  — using the model named by the `whisper-model` config key

#### Scenario: Model is configurable
- GIVEN the operator sets `whisper-model` in `~/.wolm/online-config.yaml`
- WHEN the pipeline transcribes
- THEN that model is used, with no code change required

---

### Requirement: Title and Summary Generation

Titles and summaries are generated by Claude rather than OpenAI, using structured
outputs so the response is schema-constrained and always parseable. The summarizer
receives the **entire** transcript.

#### Scenario: Summary generated
- GIVEN a complete transcript and the speaker's name
- WHEN the summarizer runs
- THEN Claude returns a title of at most six words and a three-sentence summary in a
  casual voice, as valid structured data requiring no string repair

#### Scenario: Full transcript is sent
- GIVEN a transcript of any length produced by the transcription step
- WHEN the summarizer runs
- THEN the whole file is sent — no slice, no token budget, no sentence-boundary
  trimming

#### Scenario: Long message keeps its opening and closing
- GIVEN a typical message of 70–120 minutes, which the old 12,000-token window would
  have clipped at both ends
- WHEN the summarizer runs
- THEN the opening — where the subject is stated — and the closing — where the call to
  action lands — are both present in what Claude sees

---

### Requirement: Platform Risk Notes Are Advisory

The summarizer also returns observations about passages a YouTube reviewer might act
on, so the operator can decide where to publish. These are **observations, not
verdicts**: each names a policy area, quotes the passage verbatim, and explains the
concern in one line.

The feature is on for `faith-freedom` by default and available for other ministries by
flag.

#### Scenario: Passage of concern found
- GIVEN a Faith & Freedom transcript containing a passage that touches a YouTube policy
  area
- WHEN the summarizer runs
- THEN the packet includes a note naming the policy area, quoting the passage verbatim,
  and stating in one line why it could draw review

#### Scenario: Notes never block or score
- GIVEN any number of passages of concern
- WHEN the packet is assembled
- THEN it still contains a complete, usable upload packet; no numeric risk score is
  produced; no upload/don't-upload recommendation is made; and no alternative platform
  is suggested

#### Scenario: Transcript too unreliable to judge
- GIVEN a passage where the transcription is garbled enough that the actual claim
  cannot be determined
- WHEN the summarizer evaluates it
- THEN it says the transcript is unreliable at that point rather than guessing at what
  was said

#### Scenario: Nothing of concern
- GIVEN a transcript with no passages touching a policy area
- WHEN the summarizer runs
- THEN the notes section states plainly that nothing was flagged — it does not
  manufacture marginal concerns to appear thorough

#### Scenario: Ordinary preaching is not scanned by default
- GIVEN a message with ministry `wol`
- WHEN the pipeline runs without the flag
- THEN no platform risk notes are produced

#### Scenario: Reasoning is always shown
- GIVEN any flagged passage
- WHEN the note is rendered
- THEN it carries the explanation alongside the quote, so a judgment resting on an
  out-of-date understanding of platform policy is visible to the operator rather than
  silently applied

#### Scenario: No hand-rolled JSON recovery
- GIVEN the summarizer receives a response
- WHEN the title and summary are extracted
- THEN they are read directly from the schema-validated response — the previous
  fallback that scanned lines for `"title":` and `"summary":` and trimmed quotes by
  hand no longer exists

#### Scenario: Model is configurable
- GIVEN the operator sets the summarization model in `~/.wolm/online-config.yaml`
- WHEN the summarizer runs
- THEN that model is used, so trading cost against quality needs no code change

#### Scenario: Credentials resolve without configuration
- GIVEN the operator has authenticated with `ant auth login`, or has
  `ANTHROPIC_API_KEY` set
- WHEN the summarizer runs
- THEN the SDK picks up the credential with no entry in the config file

---

### Requirement: Command Description

#### Scenario: Help text reflects reality
- GIVEN the operator runs `online --help`
- WHEN the root description is shown
- THEN it describes preparing messages for YouTube publication, and does **not** claim
  to generate an RSS podcast or a static HTML website

---

## REMOVED

### Requirement: Static Website Generation *(Removed)*

The `catalog` command and all HTML/CSS/JS templates. The new website builds its catalog
by querying YouTube, so no page generation is needed.

*Note: this removes the ability to regenerate the site. It does not remove the site
already published to S3.*

### Requirement: Audio Publication to S3 *(Removed)*

Uploading `.mp3` files to `s3://wordoflife.mn.audio/{year}/`. The Sunday Service
podcast is retired and nothing else consumed these files.

### Requirement: Transcript Publication to S3 *(Removed)*

Uploading `.text` and `.vtt` transcripts to `s3://wordoflife.mn.audio/{year}/xscript/`.
Transcripts are no longer searchable or visible on the website.

### Requirement: Thumbnail Publication to S3 *(Removed)*

Thumbnails are uploaded to YouTube with the video. S3 no longer needs a copy.

### Requirement: Transcript Sampling *(Removed)*

`xscriptExtractSample()`, `extractSampleFromMiddle()`, `findPreviousSentenceStart()`,
`findStartOfSentence()`, and `findStartOfWord()`, together with
`cmd/audio_summarize_test.go`.

Reason: this machinery existed to fit a transcript inside GPT-3.5-turbo's context
window. At 1M tokens the constraint is gone. It was also actively harmful on long
messages — it sampled outward from the midpoint, discarding the opening and closing,
which are the most useful passages for generating a title and summary.
