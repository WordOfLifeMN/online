# Open questions: round 1

Fill in each `Answer:` with an option number, plus any notes. Tell Claude when you're done.

## Q1. Box characters

Line 35 of the prompt says "ASCII or Extended ASCII", but line 45 says you prefer box-drawing characters. The rounded corners `╭ ╰` are Unicode; extended ASCII (the old DOS code page) has no rounded corners. The command already prints Unicode boxes (`cmd/audio.go:166`, `cmd/publish.go:335`), so the console handles them.

1. Unicode box-drawing characters only  **(recommended)**
2. Unicode by default, plus a plain-ASCII style (`+ - |`) chosen by a flag or setting
3. Plain ASCII only

Answer: 1

## Q2. Section title style

The prompt leaves this open (line 39). With the right edge closed (line 47), the candidates are:

1. Title inside the divider: `├─ UPLOAD PACKET ─────────────────┤`  **(recommended)**
2. Plain divider, then the title on its own line: `├─────────┤` followed by `│ UPLOAD PACKET   │`
3. Double-line divider with the title: `╞═ UPLOAD PACKET ═════════════════╡`

Answer: 1

## Q3. Where "Message #1" goes

Your mockup puts `Message #1` on the first line inside the box, under a plain header.

1. As in the mockup: plain `╭────╮` header, then `│ Message #1`, `│ Video file: …`, `│ Speaker: …`  **(recommended)**
2. In the header line itself: `╭─ Message #1 ─────╮`, then the video file and speaker lines below it

Answer: 2

## Q4. Description lines: bare or with a border?

The mockup shows `| <description>` (line 64), but line 46 says description lines have no `|` so they're easy to copy. The description is several lines: the summary, a blank line, then a name line and a URL line for each resource (`cmd/publish.go:271`). It is pasted into YouTube as a whole.

1. Bare lines, unwrapped, like the titles, so the whole block can be selected and copied  **(recommended)**
2. Bordered and wrapped at the right edge, like the other text lines (copying then picks up the `│` characters)

Answer: 1

## Q5. Thumbnail path

The thumbnail path is longer than 78 characters and contains spaces (`…\2026-q4 Prayer\2026-10-04-pm Pay Attention.jpg`), so wrapping it at spaces would split the path across lines. Do you ever copy it, for example into the YouTube file picker?

1. Show it as a bare copy line under its label, unwrapped, like the titles  **(recommended)**
2. Keep it bordered (`│ Thumb   : …`) and let only this line run past the right edge, without a closing border
3. Keep it bordered and wrap it like other text, accepting that it splits

Answer: 1

## Q6. Playlist

The mockup leaves out the playlist, but the current packet prints a `Playlist: <name>  (position N)` line and a separate copy box for the playlist name (`cmd/publish.go:340` and `:361`), when the message has a playlist.

1. Keep both: `│ Playlist: <name> (position N)` in UPLOAD PACKET, plus a "Playlist" section with the bare name to copy  **(recommended)**
2. Only the bordered line in UPLOAD PACKET; no copy line
3. Only the bare name to copy, under the Titles section

Answer: 1

## Q7. Platform risk notes

After the packet, `printRiskNotes` (`cmd/audio_summarize.go:398`) can print an advisory "⚠ Platform risk notes" list. The mockup leaves it out.

1. A "Risk notes" section inside the box, with bordered, wrapped lines; left out when there are no notes to show, as today  **(recommended)**
2. Keep it outside the box, after the footer, as it is now

Answer: 1

## Q8. When the Timeline shows

Today the Timeline goes through `log`, which is thrown away unless `--verbose` is given (`cmd/root.go:100`). That's why you see it with a timestamp, and also why you don't normally see it at all. Putting it in the box means it shows every time.

1. Always show a Timeline section in the box  **(recommended)**
2. Show the Timeline section only with `--verbose`
3. Leave it in the log as it is

Answer: 2, display the timeline better though, maybe a table with each step a row? Maybe more of a "kebab style timeline"? Maybe an emoji prefix for each operation? I'll go with your recommendation then provide feedback during testing if I have any feedback. I just want something more than a comma-delimited list.

## Q9. Timeline format

Durations currently print at full precision (`Transcribe = 2m7.9383335s`). When a step is skipped because its earlier output was reused, it shows as `0s`.

1. Round to whole seconds, one step per line (`│ Transcribe: 2m08s`), and show `reused` for a skipped step  **(recommended)**
2. Round to whole seconds on one line, as now: `│ Extract 24s · Transcribe 2m08s · Summarize 4s`
3. Keep full precision

Answer: 1

## Q10. Messages without an upload packet

`printMessageInfo` (`cmd/audio.go:674`) has two other cases: a message processed but not matched to a spreadsheet row (it prints the error, plus the generated title and summary in their own box), and a message that failed processing ("Not processed - see the error below").

1. Same box for all three: the unmatched case gets a "No upload packet" section with the reason, then Titles and Description sections holding the generated title and summary; the failed case gets a section saying it wasn't processed  **(recommended)**
2. Box only the matched case, and leave the other two as they are

Answer: 1

## Q11. The startup banner

The banner shown before the first video prompt (`printAudioBanner`, `cmd/audio.go:164`) is the other box in this command, drawn by hand in the old open-edge style.

1. Redraw it with `util/box`, so the whole command looks the same  **(recommended)**
2. Leave it as it is; this change covers only the per-message output

Answer: 1
