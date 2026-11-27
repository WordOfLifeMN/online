# Implementation Tasks: Read the Catalog from the Running Excel

**Change ID:** `read-catalog-from-running-excel`

Start only after `add-excel-catalog-source` has been reviewed and merged.

---

## Phase 1: Spike (throwaway, in scratch)

- [x] 1.1 With go-ole, attach to the running Excel and find `MediaCatalog.xlsx`. Read
      the WOL tab's `UsedRange.Value2` and print its shape, the Go types of a few cells
      (date, track, text, empty), and the time taken for every tab. This confirms the
      SAFEARRAY handling and that bulk reads are fast (target: well under 2 s for all
      tabs). ✓ 2026-11-27
  - **Fast:** attaching took 5 ms, and reading all six tabs took 51 ms (WOL, 2738 × 13,
    took 30 ms). The workbook's `FullName` is the SharePoint URL, as expected.
  - **Layout:** a 2-D SAFEARRAY (rows × columns), stored **column-major**: cell (r, c)
    is element `r + c*rows`. The header reads correctly this way. Every tab's
    `UsedRange` starts at A1 here, but the padding is still needed in general.
  - **Types:** dates arrive as `float64` serials (`36892`), text as `string`, empty
    cells as `nil`.
  - **Trailing empty rows:** WOL's `UsedRange` runs to row 2738, past the data,
    because the data validation rules extend that far. They must be trimmed, as the
    file reader does.
  - **go-ole gaps** (v1.3.0):
    - `SafeArrayConversion.ToValueArray` walks only the first dimension, so a 2-D range
      comes back wrong. Instead, read with `SafeArrayAccessData` from `oleaut32.dll`
      and view the data as `[]ole.VARIANT`.
    - `GetDimensions()` returns the dimension count cast to a pointer (0x2). Go's GC
      aborts on it ("bad pointer in frame"). Never call it; use `TotalElements(1)` and
      `TotalElements(2)`.
    - Free the `Value2` VARIANT with `VARIANT.Clear()`, which also frees the array and
      its strings. Don't also call `SafeArrayConversion.Release()`, which would free it
      twice.
- [x] 1.2 In the same spike, make sure nothing about Excel changes: the active sheet,
      the selection, and `Saved` are the same before and after. ✓ 2026-11-27
  - Before and after: `active=WOL selection=$M$2728 saved=true`. Unchanged.
- [x] 1.3 See what COM returns while the operator is mid-edit in a desktop cell
      (`RPC_E_CALL_REJECTED`, or a hang?). That decides how the busy handling and
      timeouts are built. ✓ 2026-11-27
  - Idle: 5 of 5 reads succeeded in about 22 ms each.
  - With the operator typing "Partial edit" in WOL `M2728`, not yet entered: 5 of 5
    attempts failed **at once** (0.5–11 ms) with `0x80010001` `RPC_E_CALL_REJECTED`
    ("Call was rejected by callee"). **No hang**; the 15 s watchdog never fired.
  - So busy handling is a plain retry loop. Retry `RPC_E_CALL_REJECTED` every
    100–200 ms for about 2 s, then fall back to the file with "Excel is busy". There is
    no need for a COM message filter. A watchdog is still cheap insurance against
    other dialogs.

**Gate:** PASSED. The approach is confirmed, with two adjustments for go-ole (read the
array through `SafeArrayAccessData`; never call `GetDimensions`).

---

## Phase 2: Conversion (no Excel needed)

- [x] 2.1 Move the Date-serial rule out of `sheet/microsoft/excel.go` into a helper
      both readers call. ✓ 2026-11-27
  - `serialDate(serial, date1904)` in the new `sheet/microsoft/values.go`.
- [x] 2.2 `cellText(v any) string` for `Value2` values: string; float64, whole numbers
      without a decimal point; bool → `TRUE`/`FALSE`; error codes → `#N/A`, `#VALUE!`
      and so on; nil → `""`. ✓ 2026-11-27
  - Other numbers are rounded to 15 significant digits, so 0.1+0.2 reads `0.3`.
  - Error values are a `cellError` type, so they can't be mistaken for numbers.
- [x] 2.3 `gridToRows(grid [][]any, firstRow, firstCol int, date1904 bool) [][]string`:
      pad so the title row is row 0 and column A is column 0, read the Date column by
      value, and trim trailing empty cells and rows. ✓ 2026-11-27
  - Added `fromColumnMajor`, which turns COM's column-by-column order into rows.
- [x] 2.4 Table tests for both, including a `UsedRange` that starts at B3. ✓ 2026-11-27
  - No test for a 1-based SAFEARRAY offset: the array is read whole through
    `SafeArrayAccessData` (see 1.1), so lower bounds never come into it.
  - Also tested: the Date column not first, trailing empty rows like WOL's, the 1904
    date system, and an empty range.

**Quality Gate:** PASSED
- [x] `go vet ./...` clean
- [x] `make test` passes

---

## Phase 3: COM reader

- [x] 3.1 Add `github.com/go-ole/go-ole` (v1.3.0). ✓ 2026-11-27
- [x] 3.2 Write `sheet/microsoft/live_windows.go`, which returns a `sheet.Source` read
      from the running Excel, or a reason it couldn't. ✓ 2026-11-27
  - [x] attach with `GetActiveObject` only, never `CreateObject`;
  - [x] **changed:** a workbook matches on its file name **and parent folder**, not
        the name alone. A SharePoint URL ending `…/WOLCatalog/MediaCatalog.xlsx`
        matches a local path ending `…\WOLCatalog\MediaCatalog.xlsx`; a workbook opened
        locally must be at the exact path. On the name alone, any `MediaCatalog.xlsx`
        path (a backup copy, a test's temp file) would have read the live workbook.
        More than one match means fall back. The rule is `matchWorkbook` in `live.go`,
        table-tested;
  - [x] read every tab's `UsedRange.Value2`, `Row` and `Column`, plus `Date1904`, up
        front. Every property VARIANT is cleared when the attempt ends;
  - [x] lock the OS thread, `CoInitializeEx` (treating `S_FALSE` as success) and
        `CoUninitialize`;
  - [x] retry `RPC_E_CALL_REJECTED` and `RPC_E_SERVERCALL_RETRYLATER` every 150 ms for
        2 s. The whole attempt runs on its own goroutine and is abandoned after 10 s
        if a call hangs.
  - [x] **Added:** when `GetActiveObject` fails but an `EXCEL.EXE` process exists, the
        reason is "Excel is running but can't be reached (is one of them running as
        Administrator?)". This is the elevation case from the proposal.
- [x] 3.3 Write `sheet/microsoft/live_other.go`: never live, on other platforms.
      ✓ 2026-11-27
- [x] 3.4 `microsoft.Open(path) (sheet.Source, Reading, error)` tries live first, then
      the file. `Reading{Live, Fallback}` says how it was read. ✓ 2026-11-27
  - The old file-only `Open` is now `OpenFile`, and the shared extension check is
    `checkExtension`.
  - `Fallback` is empty when Excel isn't running or doesn't have the workbook open,
    which is the normal case and needs no explanation. Otherwise it gives the reason.
- [x] 3.5 Extend the read-only comment in `sheet/microsoft` to cover COM: property
      reads only. ✓ 2026-11-27
  - It's in `excel.go`'s CRITICAL comment, and there's a matching one at the top of
    `live_windows.go`.
- **Tests:**
  - `TestMatchWorkbook` and `TestFolderAndName`, plus the `liveWorkbook` methods.
  - `TestOpen_SameNameElsewhereReadsTheFile`: a `MediaCatalog.xlsx` in a temp folder is
    read from its file even while the real one is open in Excel.
  - Errors for `.csv` and missing files.
  - The file tests now call `OpenFile`, so they never touch Excel.

**Quality Gate:** PASSED
- [x] `go vet ./...` clean
- [x] `make test` passes
- [x] `make build`
- Smoke run against the real Excel, reading only:
  - `./online.exe check` printed `Reading MediaCatalog.xlsx from Excel (live)` and
    "Online content is valid", in 0.54 s in all.
  - `peek` showed WOL as 2,728 rows with dates as `2001-01-01`. The trailing empty
    rows were trimmed, matching the file reader.

---

## Phase 4: Command output

- [x] 4.1 `cmd/sheetsource.go`: print `Reading MediaCatalog.xlsx from Excel (live)`
      when live. On a fallback, print the reason and then the existing file line.
      ✓ 2026-11-27
  - `printWorkbookReading`. The fallback line reads `<reason>; reading the file
    instead`, for example `Excel is busy; reading the file instead`.
- [x] 4.2 Tests for the three cases: live, file because Excel isn't running or the
      workbook isn't open (no reason shown), and file after a fallback (reason shown).
      ✓ 2026-11-27
  - `TestPrintWorkbookReading`.

**Quality Gate:** PASSED
- [x] `go vet ./...` clean
- [x] `make test` passes

---

## Phase 5: Check against the real workbook (with the operator)

- [x] 5.1 Workbook open in desktop Excel: edit a cell in the browser, wait about 5 s,
      run `./online.exe dump -o stdout`, and see the edit. ✓ 2026-11-27 (operator)
- [x] 5.2 Once OneDrive has synced, compare that `dump` with one read from the file
      (workbook closed in Excel). They should be equal apart from `created`.
      ✓ 2026-11-27 (operator)
- [x] 5.3 Excel closed: the file line appears, as before. ✓ 2026-11-27 (operator)
- [x] 5.4 Typing in a desktop Excel cell during a run: the tool falls back within a few
      seconds and says why. ✓ 2026-11-27 (operator)
- [x] 5.5 Run `youtube sync` against a row just edited in the browser, live.
      ✓ 2026-11-27 (operator)
- [x] 5.6 Docs: the README ("keep the workbook open in desktop Excel in the
      background"), and the `project.md` row for `sheet/microsoft`. ✓ 2026-11-27

**Quality Gate:** PASSED (operator confirmed the checks; tests pass)

---

## Completion Checklist

- [x] All phases complete
- [x] All quality gates passed
- [x] Documentation synced
- [x] Ready for `/openspec-archive`
