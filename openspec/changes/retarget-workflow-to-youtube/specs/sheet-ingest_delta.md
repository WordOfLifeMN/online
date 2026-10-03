# Delta: Sheet Ingest

**Change ID:** `retarget-workflow-to-youtube`
**Affects:** `gclient/catalog.go`

---

## ADDED

### Requirement: Look Up a Single Message

The sheet is no longer read only in bulk. Given the date and message type inferred from
a video filename, the tool finds the one message row that describes it, so the upload
packet can be filled in with series, track, ministry, visibility, and resources.

**Date and type — not speaker — are the lookup key.** A single service commonly
produces both a Prayer and a Message on the same date, frequently from the same
speaker, so speaker does not discriminate between them. Type does.

#### Scenario: Unique match on date and type
- GIVEN a sheet containing exactly one row dated 2026-03-08 with type `message`
- WHEN the tool looks up that date and type
- THEN it returns that message together with the series it belongs to

#### Scenario: Prayer and message on the same date
- GIVEN a sheet containing a `prayer` row and a `message` row both dated 2026-03-08,
  both with speaker "Pastor Vern Peltz"
- WHEN the tool looks up 2026-03-08 with type `message`
- THEN it returns the message row and does not consider the prayer row

---

### Requirement: Track Number Breaks Ties

Track number is an optional tie-breaker. It is ignored when date and type already
identify one row, and required when they do not.

#### Scenario: Track supplied but unnecessary
- GIVEN date and type match exactly one row
- WHEN the operator also supplies a track number
- THEN the track number is ignored and the single matching row is returned, even if the
  supplied track does not match that row's track

#### Scenario: Track resolves a genuine ambiguity
- GIVEN two rows share a date and type, with tracks 4 and 5
- WHEN the operator looks up that date and type with track 5
- THEN the row with track 5 is returned

#### Scenario: Ambiguity with no track supplied
- GIVEN two rows share a date and type
- WHEN the operator looks them up without a track number
- THEN the tool reports the ambiguity, lists the candidate message names and their
  track numbers, and asks for a track number — rather than silently choosing one

#### Scenario: Track supplied but matches nothing
- GIVEN two rows share a date and type, with tracks 4 and 5
- WHEN the operator supplies track 9
- THEN the tool reports that no candidate has that track, and lists the tracks that are
  available

---

### Requirement: Missing Rows Do Not Halt the Pipeline

#### Scenario: No match
- GIVEN a video whose date and type appear in no sheet row
- WHEN the tool looks up that date and type
- THEN it reports what it searched for, and the pipeline continues with the
  sheet-derived packet fields left empty — the generated title and summary are still
  produced and printed

---

## MODIFIED

### Requirement: Message Row Columns

The `Audio` column is no longer read, because audio is not published. Every other
column — `Name`, `Date`, `Speaker`, `Ministry`, `Type`, `Visibility`, `Series Name`,
`Track`, `Description`, `Thumb`, `Video`, `Resources` — is read as before.

#### Scenario: Sheet with an Audio column present
- GIVEN a sheet that still has an `Audio` column populated with S3 URLs
- WHEN the catalog is loaded
- THEN the column is ignored, its absence is not an error, and no message carries audio

#### Scenario: Sheet with the Audio column removed
- GIVEN the operator later deletes the `Audio` column from the sheet
- WHEN the catalog is loaded
- THEN loading succeeds, because `Audio` is no longer a required column

---

### Requirement: The Sheet Is a Staging Area

The sheet's role narrows: it records where a message *should* go, and the tool reads it
for that purpose only. Once a message is on YouTube, YouTube is the system of record and
the website reads YouTube, not the sheet.

#### Scenario: Bulk read still works
- GIVEN the operator runs `online dump` or `online check`
- WHEN the sheet is read
- THEN all message and series tabs are read exactly as today, including the Series-tab
  fallback for series missing from the message tabs

---

## REMOVED

*(None. No sheet-reading capability is removed — only the `Audio` column is dropped
from the set of columns consumed.)*
