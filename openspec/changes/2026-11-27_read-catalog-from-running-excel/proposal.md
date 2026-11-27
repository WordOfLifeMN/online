# Proposal: Read the Catalog from the Running Excel

**Change ID:** `read-catalog-from-running-excel`
**Created:** 2026-11-27
**Status:** Implementation Complete
**Completed:** 2026-11-27
**Depends on:** `add-excel-catalog-source` (the `sheet.Source` interface and the
`sheet/microsoft` workbook reader)

---

## Problem Statement

`add-excel-catalog-source` reads the catalog from the local, OneDrive-synced copy of
`MediaCatalog.xlsx`. The operator edits the workbook in the browser (Excel Online).
That matches the old Google workflow and is how others will edit it. But measurement
showed the local file can't keep up with an "edit a row, run the tool" loop:

| | Trial 1 | Trial 2 |
|---|---|---|
| Edit in the browser to local file | **4¾ min** | **13¾ min** |

The server had each edit within about half a minute. All the rest is OneDrive taking
its time to bring the new version down, and the delay varies from run to run. The tool
now says honestly which edits a copy includes (`includes edits up to 12:28:26`), but
the operator still has to wait, for an unknown length of time.

**Desktop Excel, by contrast, is current within seconds.** With the workbook open in
desktop Excel and edits made in the browser, polling the running Excel over COM saw
each edit 1–5 s after it was typed (the operator saw 0–2 s on screen). The local file
received none of those edits during the test. Even desktop Excel's own write to it
held an older state.

Microsoft Graph, which reads the SharePoint copy directly, would also be current. But
it needs IT to register an app, and the operator expects that to take a lot of back and
forth, so it is on hold.

## Proposed Solution

When `sheet-id` names a workbook:

1. **If desktop Excel is running and has that workbook open, read it from Excel**,
   over COM.
2. **Otherwise, read the local file**, exactly as `add-excel-catalog-source` does.

The operator's routine becomes: keep the workbook open in desktop Excel in the
background, edit in the browser, run the tool. A run that finds Excel closed still
works; it just reads the local file and says how old that copy is.

### Finding the workbook in Excel

- Attach to an Excel that is **already running** (`GetActiveObject("Excel.Application")`).
  **Never start Excel**: no `CreateObject`. If Excel isn't running, use the file.
- **Matching:** Excel reports a workbook opened from OneDrive by its SharePoint URL,
  not the local path. The session that prepared this proposal saw
  `https://wordoflifeministries.sharepoint.com/sites/VideoDownloads/Shared Documents/WOLCatalog/MediaCatalog.xlsx`.
  So a workbook matches when its `Name` equals the file name in `sheet-id`, and
  either:
  - its `FullName` is a URL whose **last folder and file name** match the `sheet-id`
    path's (`…/WOLCatalog/MediaCatalog.xlsx` against `…\WOLCatalog\MediaCatalog.xlsx`),
    or
  - its `FullName` is the `sheet-id` path itself.

  *(Changed during implementation: the first draft accepted any URL with the right
  name. That would let any `MediaCatalog.xlsx` path, such as a backup copy elsewhere,
  read the live workbook.)*
- **If more than one open workbook matches**, the tool reads the file and warns, rather
  than guessing.

### Reading from Excel

- For each worksheet, read `UsedRange.Value2` in a single call: one COM call per tab,
  not one per cell. A per-cell `Range.Text` read would take tens of thousands of calls.
- `Value2` gives raw values, so they are converted to the same text the file reader
  produces:
  - strings as they are;
  - **whole numbers without a decimal point** (`Track` 1, not `1.0`). Other numbers in
    their shortest form;
  - **the `Date` column converted from date serial to `yyyy-mm-dd`**, reusing the file
    reader's rule;
  - booleans as `TRUE` / `FALSE`, error values as Excel shows them (`#N/A`), empty
    cells as `""`.
- Rows are padded so the title row is row 0 and column A is column 0, even when
  `UsedRange` starts lower or further right.
- **If Excel is busy**, for instance the operator is typing in a cell in desktop Excel
  (COM returns `RPC_E_CALL_REJECTED`), retry for about 2 s. After that, fall back to
  the file with a warning.
- COM runs on one locked OS thread, between `CoInitialize` and `CoUninitialize`, and
  every COM object is released.

### Read-only, as before

Only property *reads* are made: `Workbooks`, `Name`, `FullName`, `Worksheets`,
`UsedRange`, `Value2` and `Row`/`Column`. No setters, no `Save`, and nothing that
activates a window, changes the selection or recalculates. The comment that guards
`sheet/microsoft` is extended to cover COM.

### What the operator sees

```
Reading MediaCatalog.xlsx from Excel (live)
```

or, when Excel doesn't have it open, the existing line:

```
Reading MediaCatalog.xlsx (includes edits up to 12:28:26, 5 min ago)
```

A fallback for any other reason (Excel busy, more than one match, a COM error) says
so, then shows the file line, for example:
`Excel is busy; reading the file instead`.

## Scope

### In Scope
- A COM reader in `sheet/microsoft`. It's Windows-only; other platforms always read
  the file.
- `microsoft.Open` tries the running Excel first, then the file. Callers don't change.
- Converting values from `Value2`, with unit tests that don't need Excel.
- Matching, busy retry, fallback with a reason, and the "from Excel (live)" line.
- Checking the result against the real workbook: a `dump` from Excel against a `dump`
  from the file, once OneDrive has synced.

### Out of Scope
- Microsoft Graph (on hold, see above).
- Starting Excel, or opening the workbook in it, when it isn't already open.
- Any write through COM.
- A flag to force a file read. Closing the workbook in Excel does the same. Add one
  later if debugging calls for it.
- Reading a workbook other than the one `sheet-id` names.

## Impact Analysis

| Component | Change Required | Details |
|-----------|-----------------|---------|
| `sheet/microsoft` | Yes | New COM reader; `Open` tries it first |
| `cmd/sheetsource.go` | Small | Prints "from Excel (live)" or the reason for falling back |
| Dependencies | Yes | `github.com/go-ole/go-ole`: pure Go (syscalls, no cgo), MIT licence |
| Catalog / `sheet` | No | Same `Source` interface, same rows |
| Google source | No | |

## Architecture Considerations

- The COM reader is just another `sheet.Source`. Nothing above `sheet/microsoft`
  learns where the rows came from, apart from the line that reports it.
- Both readers turn their values into text in the same place, so a value can't read
  one way from Excel and another from the file.
- Build tags keep the COM code Windows-only, like `open_windows.go`.
- **Elevation:** COM can't see an Excel running at a different elevation (for example
  `online.exe` run as Administrator and Excel not). That case just finds no Excel and
  reads the file. The fallback line should say so if it can tell.

## Success Criteria

- [x] With the workbook open in desktop Excel, an edit made in the browser shows up in
      the next `online` run within a few seconds, with no file sync.
- [x] With Excel closed, the tool reads the local file and says how old it is, as now.
- [x] A `dump` read from Excel equals a `dump` read from the file once OneDrive has
      synced, apart from `created`.
- [x] The tool never starts Excel, and never changes the workbook or Excel's state
      (window, selection, saved status).
- [x] If Excel is busy, the tool falls back to the file within a few seconds, saying
      why, instead of hanging.

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| `Value2` text differs from the file reader's (numbers, dates, line breaks) | Med | Med | One shared conversion; unit tests; check the real `dump` against the file |
| Excel busy or showing a modal dialog, so COM calls are rejected or hang | Med | Low | Retry about 2 s, then fall back with a warning. Bound every call so a hang can't stall the run |
| The wrong workbook matches (another `MediaCatalog.xlsx` open) | Low | Med | Match on name plus URL-or-exact-path. More than one match falls back to the file |
| Reading a workbook with unsaved desktop edits | Low | Low | Intended: it is what the operator sees. Browser edits are already saved |
| go-ole SAFEARRAY handling for 2-D ranges | Med | Low | A spike before the real code (task 1.1) |
| An edit in Excel Online takes 1–5 s to reach desktop Excel | Low | Low | Far inside the operator's loop. The "live" line doesn't claim to be instantaneous |
