# yt-api — notes for working in this directory

`yt-api` is a third-party CLI over the YouTube Data API v3, used here for **one-off bulk
cleanup** of the channel ahead of the new website. It is not part of the `online` build and
nothing in this directory is compiled or tested. The long-term plan is a YouTube SDK inside
`online` itself; until then, this is the scratch bench.

Binary: `~/go/bin/yt-api.exe` · source: `github.com/nerveband/youtube-api-cli` (v1.2.0, in the
Go module cache, readable when behaviour needs checking) · OAuth'd to **WordOfLife MN**,
channel `UCZF_jDrfP6OltpviYmj-mkA`.

---

## Use `--input` by default. Do not use `--title`.

This is the single most important rule here.

`yt-api` does **not** read the existing resource before updating. It constructs a fresh one
from whatever flags you pass (`internal/commands/update.go`, `updateVideo` / `updatePlaylist`),
and YouTube's `videos.update` **deletes any mutable property the request body omits**. So:

```
yt-api update videos --id XXX --title 'New title'     # DESTRUCTIVE
```

silently blanks `description`, `categoryId`, `tags` and `defaultLanguage`. There is no flag
for `defaultLanguage` at all, so it cannot be preserved via flags even if you try.

Instead, send a complete body — the live `snippet` and `status` echoed back with only the
field you mean to change:

```
yt-api update videos --input etc/yt-api/core-retitle/videos/XXX.json
```

### `--part snippet` does not fix it

The obvious workaround fails. `yt-api` always allocates a `status` object, so the body still
carries `status` while `--part snippet` no longer declares it, and the API rejects the call:

```
{"error":{"code":"input_error","message":"'status'"}}
Reason: unexpectedPart
```

Confirmed against video `Zq8IdYXb2nM`, 2026-10-12. A rejected call changes nothing, so this
failure mode is at least safe — unlike the silent-wipe one above.

### `--dry-run` validates almost nothing

`handleUpdateDryRun` only echoes the command-line flags. It never opens the `--input` file and
never checks `--part`. A dry-run of an `--input` call prints empty fields and proves nothing;
the broken `--part snippet` form passed its dry-run and still failed for real.

**Verify by round-trip instead:** run one call, re-`list` the resource, and diff every field
against the JSON body. Treat a clean dry-run as evidence about shell quoting only.

### Reads are eventually consistent — verify more than once

A `list` issued after a write can return the **pre-write** value, and a batched `--id a,b,c`
read can mix fresh and stale rows in one response. This is not just a batching artifact:
on 2026-10-12, three consecutive *single-id* reads of `f8n-4xM6YxA` returned the stale title,
then the new one, then the new one.

So a single read is not evidence. When verifying a bulk run, re-read anything that looks
wrong — individually, more than once — before concluding it failed. Never re-issue a write
on the strength of one stale read; that is how a correct value gets clobbered.

---

## Check channel ownership before generating writes

The catalog spans **three channels** — WOL, Faith & Freedom, and The Bridge Outreach (TBO) —
and a row's ministry does **not** guarantee the video lives on the matching channel. Two
`ministry: core` messages turned out to be on *The Bridge Outreach Media*, uploaded there by
someone else. OAuth for WOL can read them but can never update them; the only fix is a
re-upload to the right account.

Filter on `snippet.channelId` when generating any bulk write, and report the excluded rows
rather than emitting calls that cannot succeed:

```powershell
$WOL = 'UCZF_jDrfP6OltpviYmj-mkA'
$live | Where-Object { $_.snippet.channelId -ne $WOL }   # cannot be updated from here
```

## Other quirks found so far

- **`yt-api list videos --mine` can never work.** It is in the tool's own help text three
  times, but YouTube's `videos.list` has no `mine` filter — only `id`, `myRating`, `chart`.
  `--mine` is a generic flag shared by every `list` resource; it works for `channels` and
  `playlists`. To list the channel's own videos, go through the uploads playlist, which is the
  channel id with `UC` → `UU`:

  ```
  yt-api list playlist-items --playlist-id UUZF_jDrfP6OltpviYmj-mkA --part snippet --all-pages
  ```

  Then feed those ids to `yt-api list videos --id <ids> --part snippet,status`.

- **`-o table` fails on nested resources** (`cannot convert data to table format`). Use the
  default JSON and shape it in the shell.

- **`list videos --id` accepts batches**; 25 ids per call works fine.

## PowerShell quoting

Prefer single-quoted PowerShell strings (literal; double an apostrophe to escape). When a
value must span lines, use a double-quoted string with `` `n ``, and escape any embedded
double quote as `` \`" `` — a bare backtick escape is not enough, because PowerShell 5.1 hands
native commands one string that the Windows CRT re-parses, and argument splitting breaks.
`--input` avoids all of this, which is another reason to prefer it.

---

## What is in here

- `core-retitle.yt-api.txt` — 63 proposed calls (47 videos, 16 playlists) to prefix CORE
  content with `CORE | ` and move titles to `CORE | <title> | <speaker> | <date>`. Open
  questions are listed at the bottom of that file. **Not yet run.**
- `core-retitle/videos/<videoId>.json`, `core-retitle/playlists/<playlistId>.json` — the
  request bodies. Each is the live resource with only the title changed.
- `scripts/correlate.ps1` — matches catalog rows to live YouTube videos and playlists.
  Reads an `online dump` plus `yt-api list` output from `~/.wolm/scratch`, writes two CSVs
  back there.
- `scripts/gen-commands.ps1` — reads those CSVs and writes the command file and every JSON
  body. Re-run it from the repo root after a fresh dump; it rebuilds `core-retitle/` from
  scratch each time.

These scripts live here rather than in `~/.wolm/scratch` because the audio pipeline sweeps
that directory.

### Regenerating

```powershell
go run . dump -o ~/.wolm/scratch/catalog-YYYYMMDD.json   # read-only on the spreadsheet
# refresh the yt-*.json inputs in scratch if the channel has changed, then:
.\etc\yt-api\scripts\correlate.ps1
.\etc\yt-api\scripts\gen-commands.ps1
```

The scripts have the scratch paths and the dump filename hard-coded — check the top of each
before re-running.

---

## Scope reminders when selecting catalog content

- Only messages with a real `video.url` are publishable; `-` or empty means cassette-era, no
  YouTube equivalent exists. Everything dated 2013 or earlier falls in this group.
- A message that names a series with **track 0** is ancillary to the public series, not a
  numbered part of it. `omitempty` drops the zero, so those rows have **no `index` key** in
  the dump JSON — test for absence, not for `0`. Evaluate the track per series: a message can
  be numbered in one series and track 0 in another.
