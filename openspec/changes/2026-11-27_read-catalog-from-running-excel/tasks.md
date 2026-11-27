# Implementation Tasks: Read the Catalog from the Running Excel

**Change ID:** `read-catalog-from-running-excel`

Start only after `add-excel-catalog-source` has been reviewed and merged.

---

## Phase 1: Spike (throwaway, in scratch)

- [ ] 1.1 With go-ole, attach to the running Excel and find `MediaCatalog.xlsx`. Read
      the WOL tab's `UsedRange.Value2` and print its shape, the Go types of a few cells
      (date, track, text, empty), and the time taken for every tab. This confirms the
      SAFEARRAY handling and that bulk reads are fast (target: well under 2 s for all
      tabs).
- [ ] 1.2 In the same spike, make sure nothing about Excel changes: the active sheet,
      the selection, and `Saved` are the same before and after.
- [ ] 1.3 See what COM returns while the operator is mid-edit in a desktop cell
      (`RPC_E_CALL_REJECTED`, or a hang?). That decides how the busy handling and
      timeouts are built.

**Gate:** results recorded here; the approach confirmed or adjusted before Phase 2.

---

## Phase 2: Conversion (no Excel needed)

- [ ] 2.1 Move the Date-serial rule out of `sheet/microsoft/excel.go` into a helper
      both readers call.
- [ ] 2.2 `cellText(v any) string` for `Value2` values: string; float64, whole numbers
      without a decimal point; bool → `TRUE`/`FALSE`; error codes → `#N/A`, `#VALUE!`
      and so on; nil → `""`.
- [ ] 2.3 `gridToRows(values [][]any, firstRow, firstCol int) [][]string`: pad so the
      title row is row 0 and column A is column 0. Trim trailing empty cells and rows,
      as the file reader does.
- [ ] 2.4 Table tests for both, including a 1-based SAFEARRAY offset and a `UsedRange`
      that starts at B3.

**Quality Gate:** `go vet ./...` clean; `make test` passes

---

## Phase 3: COM reader

- [ ] 3.1 Add `github.com/go-ole/go-ole`.
- [ ] 3.2 Write `sheet/microsoft/live_windows.go`, which returns a `sheet.Source` read
      from the running Excel, or a reason it couldn't:
  - [ ] attach with `GetActiveObject` only, never `CreateObject`;
  - [ ] match `Name` plus a URL-or-exact-path `FullName`; more than one match means
        fall back;
  - [ ] read every tab's `UsedRange.Value2`, `Row` and `Column`, up front, then
        release;
  - [ ] lock the OS thread, `CoInitialize`/`CoUninitialize`, release every object;
  - [ ] retry `RPC_E_CALL_REJECTED` for about 2 s, and bound the whole attempt so a
        hang can't stall the run.
- [ ] 3.3 Write `sheet/microsoft/live_other.go`: never live, on other platforms.
- [ ] 3.4 `microsoft.Open`: try live first, then the file. Expose how it was read
      (live, or file plus the reason).
- [ ] 3.5 Extend the read-only comment in `sheet/microsoft` to cover COM: property
      reads only.

**Quality Gate:** `go vet ./...` clean; `make test` passes; `make build`

---

## Phase 4: Command output

- [ ] 4.1 `cmd/sheetsource.go`: print `Reading MediaCatalog.xlsx from Excel (live)`
      when live. On a fallback, print the reason and then the existing file line.
- [ ] 4.2 Tests for the three cases: live, file because Excel isn't running or the
      workbook isn't open (no reason shown), and file after a fallback (reason shown).

---

## Phase 5: Check against the real workbook (with the operator)

- [ ] 5.1 Workbook open in desktop Excel: edit a cell in the browser, wait about 5 s,
      run `./online.exe dump -o stdout`, and see the edit.
- [ ] 5.2 Once OneDrive has synced, compare that `dump` with one read from the file
      (workbook closed in Excel). They should be equal apart from `created`.
- [ ] 5.3 Excel closed: the file line appears, as before.
- [ ] 5.4 Typing in a desktop Excel cell during a run: the tool falls back within a few
      seconds and says why.
- [ ] 5.5 Run `youtube sync` against a row just edited in the browser, live.
- [ ] 5.6 Docs: the README ("keep the workbook open in desktop Excel in the
      background"), and the `project.md` row for `sheet/microsoft`.

**Quality Gate:** all tests pass; the Excel and file `dump` outputs are equal

---

## Completion Checklist

- [ ] All phases complete
- [ ] All quality gates passed
- [ ] Documentation synced
- [ ] Ready for `/openspec-archive`
