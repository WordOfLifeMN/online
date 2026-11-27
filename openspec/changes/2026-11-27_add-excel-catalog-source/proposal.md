# Proposal: Read the Catalog from an Excel Workbook on OneDrive

**Change ID:** `add-excel-catalog-source`
**Created:** 2026-11-27
**Status:** Implementation Complete (pending the operator's check in task 4.3)
**Implemented:** 2026-11-27

---

## Problem Statement

The catalog's system of record is a Google Sheet. IT wants it moved to an Excel
workbook stored in OneDrive/SharePoint. A test copy, exported from the sheet, already
exists and is synced to this machine:

```
C:\Users\WordofLifeMNMedia\OneDrive - Word of Life Ministries\Documents\WOLCatalog\MediaCatalog.xlsx
```

A previous attempt at this was abandoned. While the workbook was open in Excel, nothing
could read it, so the operator had to close Excel before every `online` run and reopen
it afterwards. The operator edits a row and then runs the tool, over and over, so that
cost comes up all day.

The tool can only read Google Sheets today. Every catalog read goes through one function,
`readOnlineContentFromInput` in `cmd/root.go`, which always calls the Sheets API.

## What was found while preparing this proposal

These were measured on 2026-11-27 against the live test workbook, with Excel holding it
open. They change what the plan has to solve.

1. **The lock comes from Go's default open, not from Excel.** Restart Manager reports
   Excel (pid 7588) as the only process holding the file. Go's `os.Open` / `os.ReadFile`
   failed 60 times out of 60. Opening the same file with `CreateFile(GENERIC_READ,
   FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE)` succeeded every time and
   returned all 677,550 bytes as a valid workbook. Excel keeps delete access to the file
   because it saves by renaming. Go's `os.Open` does not offer `FILE_SHARE_DELETE`
   (golang/go#32088), so Windows refuses it. Opening with the wider share mode fixes the
   "close Excel first" problem completely.
2. **Excel opens it as a cloud document with AutoSave on.** Excel reports the workbook's
   `FullName` as
   `https://wordoflifeministries.sharepoint.com/sites/VideoDownloads/Shared Documents/WOLCatalog/MediaCatalog.xlsx`,
   with `AutoSaveOn = True`. Edits are saved to SharePoint as they are made, and the
   OneDrive client then updates the local file. The operator never needs Ctrl+S, but the
   local file trails Excel by a sync delay that has not been measured yet (task 0.1).
3. **The exported workbook fits the existing reader.** It has the same tabs as the sheet
   (`WOL`, `Faith-Freedom`, `CORE`, `Ask Pastor`, `TBO`, `_validate`) and the same header
   row, and the `Ministry` column appears only on `CORE`, just as in the sheet. Reading
   every tab with excelize took about 0.5 s.
4. **Dates are real Excel dates.** `Date` cells hold serial numbers, e.g. `36892` for
   2001‑01‑01, with a `yyyy-mm-dd` display format. The current parser only accepts
   `2006-01-02` text. It would break the first time a date is typed in Excel's default
   US format (`11/27/2026`).
5. **The export left stale hyperlinks behind.** Some cells link to something other than
   what they show. For example, `TBO` row 2 displays `-` in `Video` but links to
   `https://youtu.be/eS6NCaUUc5Q`. The tool reads only cell text, as it does with the
   Sheets API, so these links are harmless to the tool. They can still mislead someone
   clicking around in Excel.

## Proposed Solution

### 1. `sheet-id` picks the source

The existing `sheet-id` setting (config file or `--sheet-id` flag) chooses the source:

| `sheet-id` value | Source |
|---|---|
| Contains `\` or `/`, starts with `~`, or ends in `.xlsx` / `.xlsm` (any case) | Excel workbook at that path |
| Starts with `http://` or `https://` | Error that names the problem. A URL is neither a path nor a sheet ID |
| Anything else | Google Sheet ID, unchanged from today |

A Google Sheet ID is base64url (`[A-Za-z0-9_-]`), so it can never match the path rule.
A value that looks like a path is never sent to Google. If the file is missing or isn't
`.xlsx` / `.xlsm`, the tool reports that error and stops; it never falls back to Google.
A leading `~` is expanded, as `--input` does today (`util.NormalizePath`). Google credentials are not touched when the source is Excel.

### 2. One row-to-catalog translator, two sources

`gclient/catalog.go` currently mixes two jobs: fetching rows over the Sheets API and
turning rows into `catalog` types. The plan splits them, with one package per source
under a common parent:

```
sheet/              source-agnostic: Source interface, NewCatalog(Source), row parsing
sheet/google/       was gclient/: service-account auth (read-only scope) + Sheets Source
sheet/microsoft/    Excel workbook Source: share-delete open, excelize, date serials
```

- **`sheet`** holds what both sources share:
  - a small `Source` interface (`Title()`, `Tabs()`, `Rows(tab) [][]string`), with the
    header as row 0;
  - `NewCatalog(Source)`, which takes over the column checks, tab skipping (`_` prefix),
    the ministry-from-tab default and the Series/Booklet split, *moved* unchanged from
    `gclient`.
- **`sheet/google`** is `gclient` moved, with its `catalog.go` reduced to a thin Google
  `Source`. `client.go` moves as it is, read-only `sheetsScope` comment included.
- **`sheet/microsoft`** is the Excel `Source`, `microsoft.Open(path)`.

The dependencies run one way only: `sheet/google` and `sheet/microsoft` import `sheet`
for the interface, and `sheet` imports neither of them. Choosing between the two
therefore happens in `cmd`, the only place that knows about both. When Google is
retired, `sheet/google` is deleted and nothing else changes.

This is a safe move. `gclient` holds only the Sheets auth and the Sheets reading, and
only `cmd/root.go` and `cmd/peek.go` import it. The YouTube code (`ytclient/`) has its
own OAuth login and doesn't use `gclient` at all.

Both sources produce the same `[][]string` of display values. The Sheets API already
returns display values (its default `FORMATTED_VALUE`), and excelize's `GetRows` does
the same, so `Track` comes back as `1`, never `1.0`.

### 3. How the Excel source reads

- Open with `GENERIC_READ` and share mode `READ | WRITE | DELETE`
  (`sheet/microsoft/open_windows.go`; other platforms use plain `os.Open`).
- Read the whole file into memory and **close the handle before parsing**, so the
  handle is held for milliseconds and never gets in the way of Excel's or OneDrive's
  save-by-rename.
- If the open still fails, typically because OneDrive is mid-replace, retry for about
  2 s. If it still fails after that, report an error that names the file.
- A `Date` cell holding an Excel date serial is converted to that date, whatever its
  display format. Text dates still go through the existing parser.
- **Read-only, same as the Google side.** The file is never opened for write, and
  excelize's `Save` / `Write` are never called. A test confirms the workbook's bytes and
  modification time are unchanged after a read.

### 4. Friction removers

These are the answers to "edit in Excel and run the tool with the least friction":

- **Excel stays open.** This is finding 1.
- **No cache for Excel.** Reading the workbook takes about half a second, so the
  `youtube` commands read it fresh every run instead of using the one-hour
  `catalog-cache.json`. An edit appears on the next run with no `--refresh`. The cache
  stays for Google, where a read is slow.
- **Every run shows when the file was last saved.** Each read prints, for example,
  `Reading MediaCatalog.xlsx (saved 11:04:12, 40 s ago)`, to stderr so it stays out of
  `dump -o stdout`. If the operator has just
  edited a row and the time is older than the edit, OneDrive hasn't synced it yet. The
  operator waits a moment and runs again, without having to guess.
- **`peek` works on both sources**, so connectivity and header checks are the same
  whichever source is configured.

### 5. Operator recommendations (no code)

- **Leave AutoSave on.** This is the reason no save is needed before a run.
- **Set the workbook to "Always keep on this device."** Right-click it in File Explorer,
  then choose OneDrive → Always keep on this device. If the file ever becomes a
  cloud-only placeholder, the first read waits for a download, or fails when offline.
- **Optional: clear the stale hyperlinks** in the `Video`, `Thumb` and `Resources`
  columns. Select the columns, right-click, and choose Remove Hyperlinks. The tool
  ignores them, but they can send a person to the wrong video.
- **Quote the path in the config file:**
  `sheet-id: 'C:\Users\WordofLifeMNMedia\OneDrive - Word of Life Ministries\Documents\WOLCatalog\MediaCatalog.xlsx'`.
  Single quotes keep YAML from interpreting backslashes.
- **If a sync delay is ever a real bother,** two later options read the live data and
  skip the local file altogether:
  - asking the running Excel over COM;
  - reading the SharePoint copy through Microsoft Graph, which needs an IT app
    registration.

  Neither is part of this change (see Out of Scope).

## Scope

### In Scope
- Routing `sheet-id` to Excel or to Google, as above, for every command that reads the
  catalog: `dump`, `check`, `peek`, `audio`, `youtube sync` and `youtube upload`.
- Reorganizing the packages: `sheet` (shared code), `gclient` moved to `sheet/google`,
  and the new Excel source in `sheet/microsoft`.
- The share-delete open, retry, and close-before-parse.
- Converting Excel date serials.
- No catalog cache for Excel, and the last-saved time printed on every run.
- A check that the Excel and Google sources produce the same `dump` for the same data.
- Updating flag help, the README, the config comments and `project.md` layout.

### Out of Scope
- **Writing to the workbook.** It stays read-only, like the sheet. The TODO item "Let the
  app write to the spreadsheet" is still a separate decision, with its own backup plan.
- Removing Google Sheets support. That comes later, once Excel has been the source of
  record for a while.
- Renaming `sheet-id`, for example to `catalog`. Possible later, keeping `sheet-id` as an
  alias.
- Accepting `.xlsx` for `--input`. It would cost little, but nobody asked for it.
- Live reads through Excel COM or Microsoft Graph.
- `.xls` (legacy binary), `.ods` and CSV.

## Impact Analysis

| Component | Change Required | Details |
|-----------|-----------------|---------|
| Data source | Yes | New `sheet/microsoft`. `gclient` moves to `sheet/google` behind `Source`, behavior unchanged |
| Catalog model (`catalog/`) | No | Unchanged; `NewCatalog` produces the same types |
| Config | Yes, meaning only | `sheet-id` also accepts a path. No new keys |
| Cache (`cmd/catalogcache.go`) | Yes | Not used when the source is Excel |
| CLI output | Yes | Last-saved line; `peek` works for both sources; flag help reworded |
| Dependencies | Yes | Adds `github.com/xuri/excelize/v2` (pure Go, no cgo) |
| Google auth / scope | No | `sheetsScope` stays read-only and untouched |

## Architecture Considerations

- `project.md` says `gclient/` "translates sheet rows into `catalog` types". That
  translation moves to `sheet`, so it no longer depends on Google, and each source
  package only fetches rows.
- Inside `sheet/google`, `client.go` imports `golang.org/x/oauth2/google`, a different
  package with the same name. Go allows this, but the import gets an alias
  (`googleoauth`) so readers don't trip over it.
- `gclient/catalog_test.go` splits along the same line:
  - The row-parsing tests (`NewCatalogSeriFromMessageRow_*`,
    `NewCatalogMessageFromRow_*`, `GetCellString`) move to `sheet`. They now feed
    `[][]string`, which is simpler than `[]any`.
  - The four tests that read the live Google test document move to `sheet/google`.
- excelize is pure Go, so the single `online.exe` build stays the same. It is the de
  facto Go library for `.xlsx` and is actively maintained.
- The Windows share-mode open sits behind a build tag, so tests still build elsewhere.

## Success Criteria

- [ ] With `MediaCatalog.xlsx` open in Excel, `./online.exe dump` succeeds, and Excel can
      keep editing and auto-saving during and after the run.
- [ ] `dump` from the workbook and `dump` from the Google Sheet (same data) differ only in
      `created` and in rows edited in one source since the export.
- [ ] An edit made in Excel shows up in the next `youtube sync` run with no `--refresh`,
      once OneDrive has synced (the printed last-saved time confirms it).
- [ ] A `sheet-id` with no path characters still reads the Google Sheet exactly as today.
- [ ] A date typed in Excel as `11/27/2026` is read as 2026-11-27.
- [ ] Reading never changes the workbook's bytes or modification time.

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| OneDrive sync delay means a run reads the version from before the last edit | **High** (measured: 7–60 s editing in desktop Excel; **4¾ and 13¾ min** editing in the browser) | Med | Each read says which edits it includes, from the save time recorded inside the workbook. Follow-up `2026-11-27_read-catalog-from-running-excel` reads from the running Excel, which had browser edits within 1–5 s |
| OneDrive replaces the file mid-read, or opens it briefly without sharing | Low | Low | Read everything at once, close before parsing, retry for about 2 s |
| A workbook that only looks valid, e.g. a sync-conflict copy `MediaCatalog-PC.xlsx` | Low | Med | The configured path is explicit. The tool never searches for "a" workbook |
| Excel display formats that differ from Sheets (dates, numbers shown as `1.0`) | Med | Med | Convert date serials; use display values elsewhere; check against the Sheet's `dump` |
| Data validation dropdowns (`_validate`) not carried over by the export | Med | Low (editing comfort only) | Verify in task 0.2 and recreate in Excel if missing. The tool doesn't use them |
| Someone later adds a write path to the Excel source | Low | High | Read-only test; a comment like the one on `sheetsScope`; never open for write |
