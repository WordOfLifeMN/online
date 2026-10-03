# Delta: Catalog Model

**Change ID:** `retarget-workflow-to-youtube`
**Affects:** `catalog/catalogmessage.go`, `catalog/catalogseri.go`, `catalog/resource.go`

---

## ADDED

*(None — this is a subtractive change. The model's remaining types are unchanged.)*

---

## MODIFIED

### Requirement: The Model Describes Content, Not Presentation

The catalog model retains everything the new workflow needs to decide *where a message
belongs* — ministry, visibility, series membership, track order, and attached
resources — and sheds everything that existed only to render HTML or to locate files
hosted on S3.

#### Scenario: Model survives the deletion intact
- GIVEN the static site generator has been removed
- WHEN the catalog is loaded from the sheet and validated
- THEN `Ministry`, `View`, `SeriesReference`, `OnlineResource`, `DateOnly`, and every
  check in `validate.go` behave exactly as before

#### Scenario: Consistency checks still run
- GIVEN a sheet where a message references a series that does not exist
- WHEN `online check` runs
- THEN the inconsistency is reported, as it is today — catalog validation is not
  web-specific and is not affected by this change

---

### Requirement: Online Resources Are Links, Not Widgets

An `OnlineResource` carries a URL, a display name, and optional metadata. It no longer
knows how to render itself as an icon, thumbnail, or embedded player, because nothing
renders HTML any more. Resources now feed the link list in a YouTube description.

#### Scenario: Resource parsing unchanged
- GIVEN a sheet cell containing `Study Guide|https://example.com/guide.pdf`
- WHEN the resource is parsed
- THEN the name is "Study Guide" and the URL is `https://example.com/guide.pdf`, as
  today — all three input formats (raw URL, Markdown, wiki-pipe) and the embedded JSON
  metadata continue to work

---

## REMOVED

### Requirement: Transcript Discovery *(Removed)*

`HasTranscript()`, `LoadTranscriptsCache()`, `LoadTranscriptCacheForYear()`,
`GetTranscriptURL()`, and the package-level transcript cache.

Reason: transcripts are no longer preserved or displayed. This also eliminates the
`aws s3 ls` call that ran once per year from 2005 to the present on first use.

### Requirement: Audio as Published Content *(Removed)*

The `CatalogMessage.Audio` field, `HasAudio()`, and `GetAudioSize()`.

Reason: audio is now a transient intermediate. `GetAudioSize()` existed solely to fill
the RSS enclosure length for the retired podcast.

### Requirement: HTML Rendering Helpers *(Removed)*

`OnlineResource.GetThumbnail()`, `GetIcon()`, `GetClassifier()`, `GetEmbeddedURL()`,
`GetEmbeddedVideo()`, and the unused `thumbnail` / `classifier` fields.

Reason: no HTML is generated. These returned paths into `templates/static/`, which is
also deleted.

### Requirement: Web Page Naming *(Removed)*

`CatalogSeri.GetViewID()` and `CatalogSeri.GetCatalogFileName()`.

Reason: these produced hash-obscured HTML filenames for non-public views. With no pages
to name, the scheme has no purpose.

*Note: this is the mechanism that gated partner content behind an unguessable URL. Its
replacement — YouTube "unlisted" — is weaker, and `partner` and `private` become
indistinguishable. This was reviewed and accepted as consistent with current practice;
see "Resolved During Review" in `proposal.md`.*
