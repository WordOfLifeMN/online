# Delta: Sheet Ingest

**Change ID:** `add-excel-catalog-source`
**Affects:** `cmd/root.go`, `cmd/catalogcache.go`, `cmd/peek.go`; `gclient/` (moved to
`sheet/google/`, with row parsing moved to `sheet/`); new `sheet/microsoft/`

---

## ADDED

### Requirement: `sheet-id` Selects the Source

The `sheet-id` setting names either an Excel workbook or a Google Sheet. The value
decides which one, and no other setting is involved.

#### Scenario: Workbook path
- GIVEN `sheet-id` is
  `C:\Users\WordofLifeMNMedia\OneDrive - Word of Life Ministries\Documents\WOLCatalog\MediaCatalog.xlsx`
- WHEN any command reads the catalog
- THEN the catalog is read from that workbook, and Google credentials are never loaded

#### Scenario: Home-relative path
- GIVEN `sheet-id` is `~/Documents/catalog.xlsx`
- WHEN the catalog is read
- THEN `~` is expanded and that workbook is read

#### Scenario: Bare file name with an Excel extension
- GIVEN `sheet-id` is `catalog.xlsx`, which contains no path separator
- WHEN the catalog is read
- THEN it is treated as a workbook path relative to the working directory, not as a
  Google Sheet ID

#### Scenario: Google Sheet ID
- GIVEN `sheet-id` is `1z4XIiEPMFPpeRgGpdhshiQpmY7A45KzCyZzQ7Ohe85E`
- WHEN the catalog is read
- THEN the Google Sheet with that ID is read, exactly as before this change

#### Scenario: Missing workbook never falls back
- GIVEN `sheet-id` is a path, and no file exists there
- WHEN the catalog is read
- THEN the command fails with an error that names the path, and Google is not contacted

#### Scenario: Unsupported file type
- GIVEN `sheet-id` is `C:\data\catalog.csv`
- WHEN the catalog is read
- THEN the command fails, saying that only `.xlsx` and `.xlsm` workbooks are supported

#### Scenario: URL
- GIVEN `sheet-id` is `https://docs.google.com/spreadsheets/d/1z4X.../edit`
- WHEN the catalog is read
- THEN the command fails, explaining that `sheet-id` takes a sheet ID or a file path,
  not a URL

---

### Requirement: Workbook Is Readable While Open in Excel

The tool reads the workbook while Excel has it open for editing, including with
AutoSave on, and does not disturb Excel's own saves.

#### Scenario: Excel has the workbook open
- GIVEN `MediaCatalog.xlsx` is open in Excel with AutoSave on
- WHEN `online dump` runs
- THEN the catalog is read successfully, with no need to close Excel

#### Scenario: Excel keeps saving
- GIVEN the tool has just read the workbook
- WHEN the operator edits a cell and AutoSave saves it
- THEN the save succeeds. The tool's file handle was closed before parsing began

#### Scenario: Brief contention
- GIVEN the file is momentarily unavailable while OneDrive replaces it
- WHEN the tool tries to read it
- THEN it retries for about two seconds before failing, and the error names the file

---

### Requirement: Workbook Is Never Modified

The workbook is the system of record, like the Google Sheet before it, and the tool
only ever reads it.

#### Scenario: Read leaves no trace
- GIVEN a workbook with known bytes and modification time
- WHEN the catalog is read from it
- THEN the bytes and the modification time are unchanged

---

### Requirement: Dates Are Read by Value

An Excel cell that holds a date is read as that date, whatever its display format.

#### Scenario: ISO display format
- GIVEN a `Date` cell holding serial 44254, displayed as `2021-02-27`
- WHEN the row is read
- THEN the message date is 2021-02-27

#### Scenario: US display format
- GIVEN a `Date` cell typed as `11/27/2026`, which Excel stores as a date serial and
  displays as `11/27/2026`
- WHEN the row is read
- THEN the message date is 2026-11-27

#### Scenario: Date typed as text
- GIVEN a `Date` cell holding the text `2026-11-27`
- WHEN the row is read
- THEN the message date is 2026-11-27, through the existing parser

---

### Requirement: Cell Text Is Authoritative

Cell values are read as displayed. Hyperlinks attached to cells are ignored, as they
are when reading the Google Sheet.

#### Scenario: Stale hyperlink
- GIVEN a `Video` cell that displays `-` but carries a hyperlink to a YouTube video
- WHEN the row is read
- THEN the video is `-`, meaning deliberately none, and the hyperlink is ignored

#### Scenario: Whole numbers
- GIVEN a `Track` cell holding the number 1
- WHEN the row is read
- THEN the track is read from the text `1`, never `1.0`

---

### Requirement: The Operator Can See How Current the Workbook Is

Edits reach the local file only when OneDrive syncs it: measured at 7 seconds to over
a minute with desktop Excel open, and about five minutes for edits made in the
browser. So every read shows which edits the copy includes. It uses the save time
Excel records inside the workbook, not the file's modification time, which OneDrive
sets to the server's time and which was seen to go backwards. The line goes to
stderr, so it can't corrupt the JSON that `online dump -o stdout` writes.

#### Scenario: Last-saved line
- GIVEN the workbook records that it was saved at 11:04:12, 40 seconds ago
- WHEN any command reads the catalog from it
- THEN stderr includes
  `Reading MediaCatalog.xlsx (includes edits up to 11:04:12, 40 s ago)`

#### Scenario: File time is not trusted
- GIVEN the workbook records a save at 12:05:51, but OneDrive has dated the file
  12:07:04
- WHEN the catalog is read
- THEN the line shows 12:05:51

#### Scenario: Save time not recorded
- GIVEN a workbook without a recorded save time, whose file is dated 11:04:42
- WHEN the catalog is read
- THEN stderr includes `Reading MediaCatalog.xlsx (file dated 11:04:42, 10 s ago)`

#### Scenario: Dump to stdout stays clean
- GIVEN `sheet-id` names a workbook
- WHEN `online dump -o stdout` runs
- THEN stdout holds only the catalog JSON

---

## MODIFIED

### Requirement: Catalog Cache for the YouTube Commands

The one-hour catalog cache (`catalog-cache.json`) applies only to the Google Sheet. A
workbook is read fresh on every run, because reading it takes well under a second and a
cached copy would hide edits made since.

#### Scenario: Workbook source ignores the cache
- GIVEN `sheet-id` names a workbook, and a `catalog-cache.json` ten minutes old exists
- WHEN `online youtube sync` runs without `--refresh`
- THEN the workbook is read, and the cache is neither used nor rewritten

#### Scenario: Google source keeps the cache
- GIVEN `sheet-id` is a Google Sheet ID, and a `catalog-cache.json` ten minutes old
  exists
- WHEN `online youtube sync` runs without `--refresh`
- THEN the cached copy is used, as before

---

### Requirement: `peek` Inspects Either Source

`peek` lists the tabs, column titles and first rows of whichever source `sheet-id`
names.

#### Scenario: Peek a workbook
- GIVEN `sheet-id` names a workbook
- WHEN `online peek` runs
- THEN it prints the workbook's file name, then each tab's column titles and first
  rows, limited by `--columns` and `--rows`

---

## REMOVED

(None.) Google Sheets support stays. Only its row-to-catalog translation moves to
`sheet/`, behind the `Source` interface.
