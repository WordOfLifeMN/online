# TODO

Future work raised while planning changes, not yet planned itself.

## Today (2026-10-17)

These three move the speaker from the title into the description. They go together and are best done as one small change.

1. **Limit generated descriptions by word count, say 150 words.** The prompt asks for "3 sentences in a casual voice", with no length cap (`cmd/audio_summarize.go`, `buildSummaryPrompt` and `summarySchema`). Put the limit in both places, and decide whether an over-long reply is only reported or is rejected and retried.
2. **Remove speaker names from message titles.** The title becomes `[CORE | ]<title> | <date>`. This is a change to `FormatTitle` in `ytpkg/format.go` only. `stripSpeakerTitle`, `getTitleSpeakers` and their tests may then be removable. **Watch out:** every live video was titled with its speaker, including the 47 CORE videos retitled on 2026-10-12. Once `sync` exists, it will propose a title change for every video it touches. Decide whether that's wanted, as a bulk retitle, before running `sync` widely.
3. **Make sure speaker names are in descriptions, by enforcing it in the prompt.** Require the summary to name the speaker or speakers, honorific included (`SpeakerString`, "Pastor Vern Peltz"), and check the reply contains each name before accepting it. **Gap:** when there is no generated summary, `MessageDescription` falls back to the sheet's `Description` column, which may not name the speaker. Decide whether `format.go` adds the speaker in that case. This takes over the "moving speaker names into descriptions" idea in the formats item below.

## From `2026-10-17_add-youtube-support`

- **Colour the title and description diffs, GitHub style.** Show new text in green, deleted text in red, and changed text in blue, down to the word inside a changed line rather than only whole lines. Today `sync` prints titles as plain `now:`/`new:` pairs and descriptions as a plain unified diff (`ytpkg.PrintChanges`, `DescriptionDiff`). Notes:
  - Use ANSI colours only when stdout is a terminal, and honour `NO_COLOR`, so redirected output and copy lines stay clean.
  - Windows Terminal handles ANSI colours. The older console needs VT processing enabled (`golang.org/x/sys/windows`).
  - `util/box` counts runes for padding, so escape codes inside bordered lines would push the right border out. Either keep the colour on copy lines, or teach `box` to skip escape codes when measuring.
  - Word-level diffs need a word tokenizer on top of `go-difflib`, which only diffs sequences.
- **Let the app write to the spreadsheet.** The goal is for the tool to record new video URLs and descriptions itself, instead of printing them to paste by hand. Before that can happen, the sheet needs a reliable backup plan, because it is the system of record and a mistake could destroy a lot of important data. This is also tangled up with IT wanting the sheet migrated to OneDrive and Excel. (questions-02 Q20)
- **Put playlist IDs or links in the spreadsheet.** Today a series is matched to its playlist by title alone, so a playlist's title can never be synced: a changed title looks like a new playlist. (questions-02 Q22)
- **Sort out messages that belong to several series.** The YouTube change gives one playlist per numbered series. The edge cases need fine-tuning. (questions-01 Q10)
- **Drop the approval prompt before creating a playlist**, once `sync` has proved it builds playlists correctly. The prompt is a guard against an underspecified requirement creating arbitrary playlists. (questions-01 Q12)
- **Title and description formats for other ministries.** CORE has its own title format now. Other ministries may get theirs, and there has been talk of adding metadata to descriptions, such as moving speaker names into them. All of this goes through `ytpkg/format.go`. (questions-01 Q7)
- **Fill in missing descriptions from YouTube transcripts.** Many messages have an empty `Description`. Generating descriptions from YouTube's automatic captions would avoid running hundreds of videos through faster-whisper, reusing the existing Claude summary code. Measured on 2026-10-03: transcript quality barely changes the summary.
  - **First, measure coverage.** YouTube doesn't caption every video: poor audio, music, several speakers, possibly long videos, an unset language, and some livestreams can all mean none. Sample about 40 videos across 2014–2026 and the message types with `yt-api list captions --video-id <id>`. That costs about 2,000 quota units. Then check one `yt-api download caption`: some users report the API refuses auto-generated (`asr`) tracks even for the owner.
  - **Quota:** 250 units per video (list 50 + download 200), so about 40 videos a day. For the backlog, either ask Google for more quota or use `yt-dlp --write-auto-subs --skip-download`, which uses no quota but is unofficial, so it can be rate-limited or break when YouTube changes. `yt-dlp --list-subs` gives the exact coverage list for free.
  - **Not on YouTube:** about 68 Rumble rows and a few S3 rows have no YouTube captions and need faster-whisper regardless.
  - **Output:** with the sheet read-only, write a CSV of row and generated description to paste into the sheet's `Description` column. That ties into the sheet-writing item above.
  - Fits as its own change (say `online youtube describe`) after Phase 2 of add-youtube-support, because it needs `ytclient`.
- **Faith & Freedom and TBO channels.** Probably still handled manually because their volume is low, but the per-channel token design is ready if they're ever wanted. (questions-01 Q3)
