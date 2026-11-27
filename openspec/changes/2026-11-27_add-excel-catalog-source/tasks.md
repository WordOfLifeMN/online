# Implementation Tasks: Read the Catalog from an Excel Workbook on OneDrive

**Change ID:** `add-excel-catalog-source`

---

## Phase 0: Verify assumptions (manual, about 10 minutes, before any code)

- [x] 0.1 Measure the OneDrive sync delay. With Excel open, edit one harmless cell,
      e.g. a `Notes` cell. Note the clock, then watch the local file's
      `LastWriteTime` until it changes. Run this 2–3 times and record the typical delay
      in this file. It decides whether the last-saved line is enough, or whether a live
      read (COM/Graph) needs to be planned soon. ✓ 2026-11-27
  - **Result: the local file's delay varies widely.** Three trials editing in Excel
    desktop: the local file updated after 7 s, then not within 1 minute, then after
    about 30 s. Excel Online, open in Chrome as an observer, showed each edit within
    1 s every time, so the cloud copy is current almost at once and the delay is all
    in the step that updates the local file.
  - **Browser editing: the local file is minutes behind, and erratic** (2026-11-27,
    desktop Excel closed, polled every 5 s):

    | | Trial 1 | Trial 2 |
    |---|---|---|
    | Last edit in Excel Online | 12:27:36 (an edit at 12:26:25, then reverted) | 12:35:25 (WOL `M2728` = "IMHERE") |
    | Save recorded in the workbook | 12:28:26 | 12:35:59 |
    | Server's time, stamped on the local file | 12:29:05 | 12:35:59 |
    | **Arrived in the local file** | ~12:32:19 | ~12:49:15 |
    | **Edit to local file** | **4¾ min** | **13¾ min** |

    The server had each edit within about half a minute. Almost all the delay is
    OneDrive noticing the new version. In trial 1, the version between the two edits
    never arrived locally: OneDrive brings down only whatever is current when it
    syncs. Trial 2's file was checked for "IMHERE", so the contents really changed.
  - **Desktop Excel open, edits made in the browser: Excel had them in 1–5 s.** The
    operator typed 0, 5, 10 … 30 into WOL `M2728` in Excel Online, every 5 s from
    13:14:00. The running Excel was polled over COM (reading only) once a second:

    | Typed | 0 @ :00 | 5 @ :05 | 10 @ :10 | 15 @ :15 | 20 @ :20 | 25 @ :25 | 30 @ :30 |
    |---|---|---|---|---|---|---|---|
    | In Excel's memory | :02.4 | :07.4 | :11.5 | :16.6 | :21.6 | :25.6 | :34.8 |

    This includes up to 1 s of polling. The operator saw 0–2 s on screen. **The local
    file got none of these values.** Desktop Excel wrote it once at 13:14:15.757 (the
    fraction of a second shows it was Excel, not OneDrive), but that write held an older
    state, recorded as saved at 13:13:39, with `M2728` blank, while Excel's memory
    already had "15".
  - **Decision:** when Excel has the workbook open, read it from the running Excel over
    COM. Otherwise read the local file. Microsoft Graph is on hold. See the follow-up
    change `2026-11-27_read-catalog-from-running-excel`.
  - **The file's modification time is the server's time, not the arrival time.** The
    file that arrived at 12:32 was dated 12:29:05. In an earlier run, it went backwards
    from 12:21:01 to 12:07:04 when OneDrive replaced a local save with the server's
    copy. So 3.3 now uses the save time Excel records inside the workbook
    (`docProps/core.xml`), which travels with the contents.
  - The operator has decided to edit in the browser, which matches the Google
    workflow and how others will edit the file. Desktop Excel can stay open in the
    background, since it picks up browser edits within seconds (see below).
  - **Consequence:** the last-saved line (3.3) is necessary, not just a nicety. The
    operator will regularly see a stale time and have to wait. A live read that skips
    the local file is now a real follow-up rather than a "maybe later" (see Follow-ups
    below). It doesn't block this change.
- [x] 0.2 Check that the export kept the data validation dropdowns for `Type`,
      `Visibility` and `Ministry`, which are backed by `_validate`. Recreate any that
      are missing. This is for editing comfort only; the tool doesn't use them. ✓ 2026-11-27
  - All present on every tab. `Type` and `Visibility` point at `_validate`. CORE's
    `Ministry` list is typed into the rule itself.
  - Each rule covers a fixed range of rows, e.g. WOL `D1:D2738`, with 2729 rows used
    now. Extend the ranges in Excel, or new rows past the end get no dropdown.
  - The export dropped some formatting: background colors, borders and similar. Nothing
    the tool depends on. The operator will clean it up separately.
- [x] 0.3 Set `MediaCatalog.xlsx` to "Always keep on this device" in OneDrive. ✓ 2026-11-27

---

## Phase 1: Source abstraction (refactor only, behavior unchanged)

- [x] 1.1 Move the package with `git mv gclient sheet/google`, so history follows the
      files, and rename it `package google`. Alias the `golang.org/x/oauth2/google`
      import in `client.go` as `googleoauth`. Leave `sheetsScope` and its comment
      exactly as they are. ✓ 2026-11-27
- [x] 1.2 Create `sheet/source.go` with the `Source` interface: `Title() string`,
      `Tabs() ([]string, error)`, and `Rows(tab string) ([][]string, error)` (header is
      row 0). ✓ 2026-11-27
- [x] 1.3 Move the row translation from `sheet/google/catalog.go` to `sheet/catalog.go`
      as `NewCatalog(src Source)`. Switch `[]any` to `[]string`. Keep the column
      constants, required-column check, `_` tab skip, ministry-from-tab default and
      Series/Booklet split exactly as they are. ✓ 2026-11-27
  - One deliberate difference: an empty tab is now reported as "sheet is empty" and
    skipped. Before, indexing its missing title row would have panicked.
- [x] 1.4 What's left of `sheet/google/catalog.go` becomes the Google `Source`
      (`google.Open(ctx, id)`), using the existing `GetSheetService` and `Values.Get`
      calls. `sheet` must not import `sheet/google`. ✓ 2026-11-27
  - `Rows` reads each tab with one `Values.Get` of `1:80000`. Before, it took two: one
    for the title row and one for `2:80000`.
- [x] 1.5 Split `catalog_test.go`: ✓ 2026-11-27
  - [x] Move the row-parsing tests to `sheet/catalog_test.go`, with an in-memory fake
        `Source`.
    - Added `TestNewCatalog`, `TestNewCatalog_EmptyTab` and `TestGetIndexOfColumns`.
  - [x] Keep the four live-document tests in `sheet/google`.
    - `TestReadMessagesFromDocument` and `TestNewCatalogFromSheet` covered the same
      thing, so they became a single `TestNewCatalog`.
    - `TestReadMessageSheet` reads only the `Messages` tab, through a one-tab wrapper.
    - Added `TestToStrings`.
- [x] 1.6 Point `cmd/root.go` and `cmd/peek.go` at the new packages:
      `readOnlineContentFromInput` calls `sheet.NewCatalog(google.Open(...))`. Make sure
      no `gclient` import is left anywhere. ✓ 2026-11-27
  - Also updated the comment in `ytclient/auth.go` that named `gclient`.
- [x] 1.7 Compare `./online.exe dump` output before and after this phase against the
      Google Sheet. It must be identical apart from `created`. ✓ 2026-11-27
  - Identical apart from `created`.
  - The config's `sheet-id` already names the workbook, so both runs passed
    `--sheet-id 1z4X…85E` explicitly.

**Quality Gate:** PASSED
- [x] `go vet ./...` clean
- [x] `make test` passes, including the live Google tests
- [x] Google `dump` identical before and after the refactor

---

## Phase 2: Excel source

- [x] 2.1 Add the `github.com/xuri/excelize/v2` dependency. ✓ 2026-11-27
  - v2.11.1, pure Go. It moved several `golang.org/x/*` modules up to newer versions
    (crypto, net, sync, sys, term, text). Those are routine, compatible updates.
- [x] 2.2 Write `sheet/microsoft/open_windows.go`: `readShared(path) ([]byte, error)`
      using `CreateFile(GENERIC_READ, FILE_SHARE_READ|WRITE|DELETE, OPEN_EXISTING)`,
      then read everything and close. Retry on a sharing violation for about 2 s. Write
      `sheet/microsoft/open_other.go` as the non-Windows version, using `os.ReadFile`.
      ✓ 2026-11-27
  - It also retries `ERROR_LOCK_VIOLATION`. It polls every 100 ms.
- [x] 2.3 Write `sheet/microsoft/excel.go`: `microsoft.Open(path)` returns a
      `sheet.Source`. It reads the bytes, then calls `excelize.OpenReader`. `Rows` uses
      `GetRows` (display values). `Title()` is the file name. Also expose the file's
      modification time for the last-saved line. ✓ 2026-11-27
  - `Open` also rejects anything that isn't `.xlsx`/`.xlsm`, and names the path when
    the file is missing or unreadable.
- [x] 2.4 Date serials: where a `Date` cell's raw value is numeric, convert it with
      `excelize.ExcelDateToTime` and emit `2006-01-02`. Otherwise pass the display text
      through unchanged. ✓ 2026-11-27
  - Honours the workbook's 1904 date system, if it uses one.
  - The column title is shared as `sheet.DateColumn`.
- [x] 2.5 Add a comment in `sheet/microsoft/excel.go` like the `sheetsScope` one:
      read-only on purpose, never open for write, never call `Save` / `Write`, and raise
      it before changing. ✓ 2026-11-27
- [x] 2.6 Tests. The fixture workbook is built by the test in a temp folder, not checked
      in. ✓ 2026-11-27
  - [x] a header row with and without `Ministry`, matching the existing parsing tests;
  - [x] a date stored as a serial with a `yyyy-mm-dd` format, one with an `m/d/yyyy`
        format, and one stored as text;
  - [x] `Track` shows as `1`, not `1.0`. The fixture stores the raw value `1.0`, as the
        export does;
  - [x] a cell whose hyperlink differs from its text: the text wins;
  - [x] the workbook's bytes and modification time are unchanged after
        `microsoft.Open` + `sheet.NewCatalog`;
  - [x] Windows only: hold the fixture open with a second handle the way Excel does,
        with `DELETE` access and share mode `READ|WRITE|DELETE`. `os.ReadFile` fails on
        it, which shows the test reproduces the problem, and `readShared` succeeds.
    - Also: a handle that shares nothing makes `readShared` retry, then fail with an
      error naming the file.
    - If a future Go makes `os.ReadFile` succeed against the hold, the test logs it
      rather than failing.
  - [x] Also tested: errors for a missing file, a `.csv`, and a file that isn't a zip.

**Quality Gate:** PASSED
- [x] `go vet ./...` clean
- [x] `make test` passes

---

## Phase 3: Routing and commands

- [x] 3.1 Write `classifySheetRef(value) (kind, normalized, error)`. It goes in `cmd`
      because that is the only package that imports both `sheet/google` and
      `sheet/microsoft`. ✓ 2026-11-27
  - It lives in the new `cmd/sheetsource.go`, together with `openSheetSource` and
    `sheetIsWorkbook`, rather than in `cmd/root.go`.
  - Excel when the value contains `\` or `/`, starts with `~`, or ends in
    `.xlsx`/`.xlsm` (any case). An error for `http(s)://` (any case). Anything else is
    Google. `util.NormalizePath` expands only `~`, not environment variables.
  - Table test: the real sheet ID, the real OneDrive path, forward slashes, `~/…`, a
    bare `catalog.xlsx`, `.XLSM`, a `.csv` path, and Google and SharePoint URLs. The
    missing-file and `.csv` cases are tested through `openSheetSource`.
- [x] 3.2 `readOnlineContentFromInput`: route Excel to `microsoft.Open`, and Google to
      `google.Open`. A path that doesn't exist, or isn't `.xlsx`/`.xlsm`, is an error
      that names the path. It never falls back to Google. ✓ 2026-11-27
- [x] 3.3 Print the last-saved line on every Excel read:
      `Reading MediaCatalog.xlsx (saved 11:04:12, 40 s ago)`. ✓ 2026-11-27
  - **Changed: it goes to stderr, not stdout,** so it can't corrupt
    `online dump -o stdout`.
  - **Changed: it uses the save time Excel records inside the workbook, not the file's
    time.** The line reads `Reading MediaCatalog.xlsx (includes edits up to 12:28:26,
    5 min ago)`. The file's time could run backwards (see 0.1). It is used only when the
    workbook records no save time, labelled `file dated …`. The source exposes this as
    `Workbook.Saved()`. Tested both ways; the "not recorded" fixture is a copy without
    `docProps/core.xml`.
  - Ages read "N s ago", "N min ago", "N h ago" (with the clock time), or a date after a
    day.
- [x] 3.4 Have `readCatalogCached` skip the cache when the source is Excel, and update
      its doc comment and the tests in `catalogcache_test.go`. ✓ 2026-11-27
  - Added `TestCatalogCache_WorkbookBypasses` and `TestCatalogCache_GoogleSheetUsesCache`.
- [x] 3.5 Port `peek` to `Source`, so it lists tabs, headers and the first rows for
      either source. Update its help text. ✓ 2026-11-27
- [x] 3.6 Reword the help text. ✓ 2026-11-27
  - The `--sheet-id` flag; the `check` and `dump` `Use` and `Example` strings; the root
    command's description.
  - The `youtube sync` help about the cache, and the `--refresh` flag, which now says
    it applies to a Google Sheet only.
  - The "no input specified" error, which also named the wrong config file
    (`online.yaml`).

**Quality Gate:** PASSED
- [x] `go vet ./...` clean
- [x] `make test` passes
- [x] `make build` (`go build -o online.exe`)

---

## Phase 4: Check against the real data, then docs

- [x] 4.1 With Excel open on the workbook: run `./online.exe dump --sheet-id '<xlsx path>'`
      and `./online.exe dump --sheet-id <google id>`, then diff the two outputs. Explain
      every difference. ✓ 2026-11-27
  - Excel was open (pid 7588) throughout. Reading the workbook took 0.7 s in all.
  - Both dumps have 2,691 messages and 346 series. Series are identical, and every
    date matches.
  - **Only difference: 7 descriptions with line breaks.** Google returns `\r\n` and
    Excel stores `\n`. No effect on YouTube: `ytpkg/diff.go` already converts `\r\n`
    to `\n` before comparing.
- [x] 4.2 Run `check` against the workbook; it should report no new problems compared
      with the sheet. ✓ 2026-11-27
  - Both report "Online content is valid".
- [ ] 4.3 Edit a row in Excel, wait for the last-saved time to move, and run
      `youtube sync` (dry run or as usual). The edit should show with no `--refresh`.
  - **Left for the operator.** It needs an edit in Excel and a real sync. The cache
    bypass itself is covered by `TestCatalogCache_WorkbookBypasses`.
- [x] 4.4 Docs: ✓ 2026-11-27
  - [x] the README. Added a "The spreadsheet" section: the `sheet-id` rules, the quoted
        YAML path, reading while Excel is open, the sync delay and saved line, and
        "Always keep on this device". The Google credentials are marked as needed only
        for a Sheet ID;
  - [x] the config comment in `~/.wolm/online-config.yaml`, with the quoted-path
        example. The operator had already written both;
  - [x] the layout table in `openspec/project.md`.
- [x] 4.5 Memory: extend the "read-only" note to cover the Excel workbook. ✓ 2026-11-27

**Quality Gate:** PASSED (except 4.3, which is manual)
- [x] All tests pass
- [x] `go vet ./...` clean
- [x] Every difference between the Excel and Google `dump` outputs explained

---

## Follow-ups (not part of this change)

- **Live read, skipping the local file** (raised by the 0.1 result).
  - **Read from the running desktop Excel over COM (next):** written up as
    `openspec/changes/2026-11-27_read-catalog-from-running-excel/`. Desktop Excel picks
    up browser edits within 1–5 s, and this needs nothing from IT. The local file stays
    as the fallback when Excel isn't open.
  - **Microsoft Graph (on hold):** read the SharePoint copy directly, with or without
    Excel open. It needs IT to register an Azure AD app with a read-only scope. The
    operator expects a lot of back and forth with IT to get that right, so it waits.

  Either one would plug in as another `sheet.Source`, so nothing in this change would
  have to be redone.

---

## Completion Checklist

- [ ] All phases complete (all but 4.3, the operator's edit-then-sync check)
- [x] All quality gates passed
- [x] Documentation synced
- [x] Switch `sheet-id` in `~/.wolm/online-config.yaml` to the workbook path (operator's
      call). Already done, with the path quoted.
- [ ] Ready for `/openspec-archive` (after 4.3)
