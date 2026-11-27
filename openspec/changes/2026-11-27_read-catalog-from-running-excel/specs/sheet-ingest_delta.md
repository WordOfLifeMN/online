# Delta: Sheet Ingest

**Change ID:** `read-catalog-from-running-excel`
**Affects:** `sheet/microsoft/` (new COM reader; `Open` tries it first),
`cmd/sheetsource.go` (what the operator is told)

---

## ADDED

### Requirement: Read the Workbook from the Running Excel When It Is Open

When `sheet-id` names a workbook and desktop Excel has it open, the catalog is read
from Excel itself, which picks up browser edits within seconds. Otherwise the local
file is read, as before.

#### Scenario: Open in Excel
- GIVEN desktop Excel is running with `MediaCatalog.xlsx` open, from SharePoint
- WHEN any command reads the catalog
- THEN it is read from Excel, and stderr includes
  `Reading MediaCatalog.xlsx from Excel (live)`

#### Scenario: A browser edit is seen at once
- GIVEN the workbook is open in desktop Excel
- AND a cell was changed in Excel Online 5 seconds ago
- WHEN the catalog is read
- THEN the change is in the catalog, though the local file doesn't have it yet

#### Scenario: Excel not running
- GIVEN Excel is not running
- WHEN the catalog is read
- THEN the local file is read, with the usual `includes edits up to …` line, and
  Excel is **not** started

#### Scenario: Excel running without the workbook
- GIVEN Excel is running with other workbooks open, but not `MediaCatalog.xlsx`
- WHEN the catalog is read
- THEN the local file is read, and the workbook is not opened in Excel

#### Scenario: Two candidate workbooks
- GIVEN Excel has two workbooks named `MediaCatalog.xlsx` open, from different places
- WHEN the catalog is read
- THEN the local file is read, with a warning that the open workbook was ambiguous

#### Scenario: Excel is busy
- GIVEN the operator is typing in a cell in desktop Excel, so COM calls are rejected
- WHEN the catalog is read
- THEN the tool retries for about 2 seconds, then reads the local file, saying Excel
  was busy

---

### Requirement: Excel and the File Give the Same Rows

A workbook reads the same whether it comes from Excel or from the file.

#### Scenario: Whole numbers
- GIVEN a `Track` cell holding 1
- WHEN it is read from Excel
- THEN it reads `1`, as it does from the file

#### Scenario: Dates
- GIVEN a `Date` cell holding the serial 46353, displayed as `11/27/2026`
- WHEN it is read from Excel
- THEN it reads `2026-11-27`

#### Scenario: Same catalog
- GIVEN the local file has synced with what Excel has open
- WHEN the catalog is dumped once from Excel and once from the file
- THEN the two are equal apart from `created`

---

### Requirement: Excel Is Only Read

Reading through Excel changes nothing, in the workbook or in Excel.

#### Scenario: No trace
- GIVEN Excel has the workbook open, showing the WOL tab, with cell B10 selected and
  no unsaved changes
- WHEN the catalog is read from it
- THEN Excel still shows the WOL tab with B10 selected, and still has no unsaved
  changes

---

## MODIFIED

### Requirement: The Operator Can See How Current the Workbook Is

When the workbook is read from Excel, the line says so instead of giving a save time.
When the file is read because Excel couldn't be used, the line gives the reason first.

#### Scenario: Fallback reason
- GIVEN Excel was busy
- WHEN the catalog is read from the file instead
- THEN stderr shows `Excel is busy; reading the file instead`, followed by the
  `includes edits up to …` line

---

## REMOVED

(None.)
