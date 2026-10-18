# Add YouTube support

## Goals

Make this tool work directly against YouTube instead of only printing a packet to paste into YouTube Studio:

1. **`online youtube get`** reads a video or playlist from YouTube and shows it as a *ytPackage*. This is one shared description of a YouTube video (`ytpkg.Message`) or playlist (`ytpkg.Series`), used everywhere in the tool. The `audio` command's output becomes a `ytpkg.Message` too.
2. **`online youtube sync`** builds the package the spreadsheet says a video should have, diffs it against the live video, and, with approval, makes YouTube match the sheet. That covers the video, its playlist membership, and the playlists it touched.
3. **`audio` uploads.** At the end of the audio workflow, the proposed package is shown and, with approval, the video is uploaded, given its thumbnail, added to its playlist (creating the playlist if approved), and published.

A large part of this is safety. The sheet is the system of record and stays read-only. Every YouTube write is a read-modify-write that preserves the fields it doesn't manage. Destructive changes need a typed `y`. Re-running anything should do as little damage as possible (Q17).

## Requirements

### Authentication and channels
- YouTube uses a user OAuth flow (an installed app), not the Sheets service account. The client ID and secret are the yt-api client's, copied into `online-config.yaml` as `youtube-client-id` and `youtube-client-secret`. Nothing reads `~/.yt-api/` at runtime. (Q1) `redactIfSecret` already hides the secret in the verbose config dump, because its key contains "secret".
- One token per channel, with the full `youtube` scope (`youtube.YoutubeScope`), stored at `~/.wolm/youtube-token-<channel>.json` with mode 0600. The token is never logged. (Q2)
- The first time a channel is used without a token, the tool prints the consent URL, tries to open it in the browser, and waits on `http://localhost:8080` for the redirect, the same address yt-api uses. It uses PKCE (`oauth2.GenerateVerifier` / `S256ChallengeOption`) and a random `state`. Refreshed tokens are saved back to the file.
- Tokens are keyed by channel: WOL, Faith & Freedom, TBO. Only WOL is set up and tested in this change. Expected channel IDs come from the config map `youtube-channel-ids`, which defaults to WOL = `UCZF_jDrfP6OltpviYmj-mkA`. A message for a channel with no token, or with no configured ID, is reported as "not authenticated for channel X" and skipped. It is not an error for the run. (Q3)
- After the token loads, the authenticated channel is read with `channels.list mine=true` and must equal the expected ID. Before any write to a video or playlist, its `snippet.channelId` must equal the authenticated channel. If it doesn't, the video or playlist is reported and skipped (lesson D1 of `etc/yt-api/core-retitle.yt-api.txt`).
- The Google Sheets scope in `gclient/client.go` stays `sheets.SpreadsheetsReadonlyScope`. Nothing in this change writes to the sheet. (Q18)

### ytPackage (`ytpkg`)
- New package `ytpkg` with `ytpkg.Message` (a video) and `ytpkg.Series` (a playlist). `cmd/publish.go`'s `UploadPacket` and its builder functions move into `ytpkg`, and `UploadPacket` is deleted. `MessageInfo.Packet` becomes a `*ytpkg.Message`. (Q4)
- A package holds the union of what YouTube tracks and what this tool generates. YouTube's side: ID, channel ID, title, description, privacy, category, tags, default language, default audio language, made-for-kids, license, embeddable, publish date, thumbnail URL, playlist memberships. The tool's side: channel name, alternative title, local thumbnail path.
- **Formatting is decided in one place: `ytpkg/format.go`.** (Q7) This covers the title's components and their order, the separator (` | `), the CORE marker, the description layout (text, a blank line, then one name and URL pair per resource), the series description, and playlist names. Nothing else in the codebase builds these strings. The file is laid out so that a per-ministry format, or new description metadata such as speakers, is a change to this file only.
- CORE ministries (`isCoreMinistry`: core and its four sub-ministries) get a leading `CORE` component. Video title: `CORE | <title> | <speaker> | <date>`. Playlist: `CORE | <series>`. This matches the live channel after the 2026-10-12 retitle. The old `corePlaylistPrefix = "CORE: "` is removed. (Q7)
- House defaults applied to every package built from the sheet, also in `format.go`: category `29` (Nonprofits & Activism), default language `en`, default audio language `en`, made-for-kids `false`. (Q5)
- **Playlists for a message:** one per series in which the message has a non-zero track, at that track's position. Track 0 never puts a message in a playlist. Ask the Pastor messages go only into "Ask the Pastor", with no position. (Q10, track-0 rule) Synthetic stand-alone series (ID prefix `SAM-`, made by `createStandAloneMessageSeries`) never become playlists.
- **Privacy** (Q8):
  - *New item, or live privacy inconsistent with the sheet:* public → public, private → unlisted, partner → unlisted.
  - *Consistent live privacy is kept:* sheet public ↔ YouTube public; sheet private ↔ YouTube private **or** unlisted; sheet partner ↔ YouTube unlisted.
  - Raw is refused, as today.
- YouTube's limits are checked before any write: a title of at most 100 characters, a description of at most 5000 bytes, and no `<` or `>` in either. A package that fails is reported and not written.

### `online youtube get`
- `online youtube get <ref>...` takes one or more video or playlist references. A ref can be an ID; a `youtu.be/ID`, `watch?v=ID`, `live/ID` or `shorts/ID` link; one of the sheet's malformed `https://youtu.beID` links; or a playlist ID or `playlist?list=` link. A `watch?v=…&list=…` link means the video unless `--playlist` is given.
- Output is a `util/box` box per package (Q6). Video: every field above, plus the thumbnail URL (`maxres`, falling back to `high`, Q15), and the playlists it belongs to if they can be found (see "Playlist lookup"). Playlist: its fields, then one line per item: position, video ID, privacy and title.
- `--json` prints the packages as indented JSON instead.

### Spreadsheet cache (Q14)
- The `youtube` commands read the catalog through a cache: `<scratch-dir>/catalog-cache.json`, in `online dump` format. It is used when it is less than 1 hour old, judged by the file's modification time. Otherwise the sheet is re-read and the cache is rewritten (written to a temporary file, then renamed). `--refresh` forces a re-read. An explicit `--input` bypasses the cache entirely.
- `audio` keeps reading the sheet fresh every time, because the row has usually just been edited.
- The scratch sweep only deletes `.mp3` and `.text`, so the cache survives it.

### Matching
- A sheet row matches a YouTube video when the video ID parsed from the row's `Video` column is the video's ID. If several rows carry the same ID, you're asked to choose (the existing chooser, generalised). A video with no row is reported ("not in the spreadsheet") and skipped.
- **Playlist lookup** is by exact title among the channel's own playlists (`playlists.list mine=true`, all pages, fetched once per run). The sheet stores no playlist IDs, so the title *is* the key. As a result, `sync` never changes a playlist's title (Q22 note). When no playlist has the expected title, the near-matches are shown: those whose normalised titles are equal or one contains the other, as in `correlate.ps1`. These are shown before any offer to create a new one, so that a renamed playlist isn't duplicated by accident.

### `online youtube sync`
- `online youtube sync <ref>...` takes one or more video or playlist references. A playlist expands to every video in it. Each video is diffed and confirmed in turn. (Q11)
- **Fields compared and written:** title, description, privacy, category, default language, default audio language, made-for-kids, and playlist membership. (Q5) Every other field is preserved exactly by the read-modify-write.
- **Description:** always synced from the sheet's `Description` column plus resource links, even when that removes YouTube text. (Q9) It is *destructive* when YouTube's current description is not empty and any of its non-blank lines are missing from the new one. Adding resource links doesn't count. A destructive change prints a warning with the full text that will be lost, as a copy line.
- **Diff display:** a box per video. Short fields show as `now:` and `new:` labeled lines, unchanged fields are hidden, and the description shows as a unified line diff. Warnings get their own section. A video with nothing to change prints a one-line "in sync".
- **Approval** (Q13, Q19): `Apply these changes? [Y/n]`, defaulting to Yes. `--yes` answers Yes without asking. But when the video has a destructive warning, the prompt is `[y/N]`, defaults to No, and `--yes` does **not** skip it. Answering No skips that video and the run continues.
- **Thumbnails:** never compared. `--thumbnail <path>` uploads that image as part of the same approval. It is only allowed when exactly one video is being synced. (Q15)
- **Playlist membership** (Q12): the video is added to each expected playlist it isn't already in, at the end of the playlist. It is never removed from any playlist. A missing playlist can be created, with its title from `format.go`, its description from the series, and its privacy mapped from the series visibility. Creation always gets its own `[y/N]` prompt, defaulting to No, that `--yes` does not skip. Every playlist that was added to, or created, is recorded as *touched*.
- **End-of-run sweep** over the touched playlists, plus any playlist named on the command line:
  1. *Playlist fields* (Q22): the description (series description plus booklets) and privacy (the Q8 rules, using the series visibility) are diffed and approved the same way as a video, including the destructive-description rule. The title is never synced.
  2. *Order* (Q12, Q21): the desired order puts the sheet-numbered videos first, in track order. Everything else follows in its current relative order. Only the moves actually needed are made: the items in the longest increasing subsequence of the current order stay where they are, and the rest are moved. The current and proposed orders are shown, with `[Y/n]` approval and `--yes` honoured. Each move is a `playlistItems.update` at 50 quota units, so the number of moves is shown.
  3. *Video ordering must be "Manual"* (added after the rounds, at your request). Every synced playlist should have Studio's **Video ordering** set to *Manual*. The Data API has no field for this (`PlaylistSnippet` and `PlaylistStatus` have none), so the tool can't set it or read it directly. Instead:
     - The API rejects a positioned `playlistItems.update` or `insert` on a playlist that isn't manually sorted, with the reason `manualSortRequired`. `ytclient` turns that into a typed error, `ErrManualSortRequired`. The sweep then stops reordering that playlist and shows a box section: `Set Video ordering to "Manual" in Studio: <playlist title>`, then the Studio link `https://studio.youtube.com/playlist/<id>/edit` as a copy line, then `then re-run sync`. The run continues with the next playlist.
     - Every playlist the tool *creates* (in sync or in the audio flow) gets the same reminder, with its link, as soon as it's created. Before trusting the default, check at the Phase 4 checkpoint whether a new playlist starts as Manual.
     - The end-of-run summary lists every playlist that needs the setting, so none are missed when several were touched.
- Errors on one video are reported, and the run continues with the next.

### Upload from `audio` (Phase 6)
- After the per-message boxes, each message that has a package is offered for publishing:
  - **Already uploaded** (Q17): if the sheet row's `Video` column holds a YouTube URL, no upload is offered. The `sync` flow runs for that video instead.
  - **Duplicate guard** (Q24, both checks): a local record `~/.wolm/youtube-uploads.json` maps the absolute video path and file size to the uploaded video ID. The channel's 50 most recent uploads (its `UU…` uploads playlist) are also checked for the exact title. A hit on either means no upload: the sync flow runs for the found video, and its URL is printed for the sheet.
  - Otherwise you see the proposed package and plan: the steps, any playlist that would be created (gated as in sync), and the reorder moves. Then `Upload to YouTube? [Y/n]`.
- **Sequence** (Q16): upload as **private** (resumable, with `madeForKids` and the house defaults set), set the thumbnail, create the playlist if approved, add the video to its playlists, reorder them, and finally set the target privacy. A failure stops the remaining steps and leaves the video private. The video ID is written to the local upload record straight after step 1, so a re-run can't upload it a second time.
- **Progress** (Q23): a numbered step list, one line per step, each finished as it completes. The upload line updates in place with a percentage. There are no emoji. A failed step shows as `FAILED: <reason>` and the steps after it as `not run`:
  ```
  Publishing "Pay Attention | Mary Peltz | October 4, 2026"
    1/6  Upload video (1.2 GB) ......... done  4m12s
    2/6  Set thumbnail ................. done
    3/6  Create playlist "Prayer" ...... skipped (exists)
    4/6  Add to playlist ............... done
    5/6  Reorder playlist .............. 3 moves, done
    6/6  Set privacy: public ........... done
  ```
- **After a successful upload** (Q18, Q20): the new video URL and the description are printed as copy lines under the row they belong to (`DescribeForChoice`), for you to paste into the sheet's `Video` and `Description` columns. Until the description is pasted, a later `sync` shows the destructive-description warning for that video, which is the intended safety net.
- An upload costs about 1,600 of the project's 10,000 daily quota units, and that budget is shared with yt-api. A `quotaExceeded` error is reported in plain words.

## Out of scope
- **Writing to the spreadsheet** of any kind, including writing back video URLs or descriptions. This needs a reliable backup plan for the sheet first, and IT wants to move it to OneDrive/Excel anyway. (Q20 note)
- **Playlist IDs or links in the sheet.** Without them a playlist can only be matched by title, so playlist titles can't be synced. That's a follow-up. (Q22 note)
- **Messages in several series:** this change uses one playlist per numbered series. Sorting out the edge cases is a follow-up. (Q10 note)
- **Setting up the Faith & Freedom and TBO channels.** The per-channel design supports them, but only WOL is authenticated here. (Q3)
- Removing a video from a playlist, deleting videos or playlists, and changing a playlist's title.
- New title formats for other ministries, or speakers in the description. `format.go` is shaped for them, but they aren't implemented.
- Comparing thumbnails. Only the link to the live one is shown.
- Setting a playlist's Video ordering automatically. The Data API doesn't expose it, so the tool detects a wrong setting (`manualSortRequired`) and tells you to change it in Studio.
- Tags (preserved, never written), captions, scheduled publishing, recording location and date.

## Architecture

### New package `ytpkg` (no network, fully unit-tested)

```go
// ytpkg/channel.go
type Channel string                  // ChannelWOL, ChannelFaithAndFreedom, ChannelTBO, ChannelUnknown (moved from cmd/publish.go)
func ChannelForMinistry(m catalog.Ministry) Channel
func IsCoreMinistry(m catalog.Ministry) bool

// ytpkg/privacy.go
type Privacy string                  // PrivacyPublic, PrivacyUnlisted, PrivacyPrivate
func PrivacyForView(v catalog.View) (Privacy, error)          // new/inconsistent mapping; refuses raw
func IsPrivacyConsistent(v catalog.View, live Privacy) bool  // Q8 table
func ResolvePrivacy(v catalog.View, live Privacy) (Privacy, error) // keep live when consistent, else map

// ytpkg/message.go
type PlaylistRef struct {
	Title    string `json:"title"`
	ID       string `json:"id,omitempty"`       // empty until looked up or created
	Position int    `json:"position,omitempty"` // track; 0 = no position (Ask the Pastor)
}
type Message struct {
	ID, ChannelID        string
	Channel              Channel
	Title, Alternative   string
	Description          string
	Privacy              Privacy
	CategoryID           string
	Tags                 []string
	DefaultLanguage      string
	DefaultAudioLanguage string
	MadeForKids          bool
	License              string
	Embeddable           bool
	PublishedAt          time.Time
	ThumbnailURL         string // live, largest available
	ThumbnailPath        string // local file to upload
	Playlists            []PlaylistRef
}                                     // all fields json-tagged, omitempty where sensible

// ytpkg/series.go
type PlaylistItem struct{ ItemID, VideoID, Title string; Privacy Privacy; Position int }
type Series struct {
	ID, ChannelID string
	Channel       Channel
	Title         string
	Description   string
	Privacy       Privacy
	ThumbnailURL  string
	Items         []PlaylistItem
}

// ytpkg/catalog.go - the sheet side
func NewMessageFromCatalog(msg *catalog.CatalogMessage, cat SeriesFinder,
	generatedTitle, summary, thumbnailPath string) (*Message, error)   // replaces NewUploadPacket
func NewSeriesFromCatalog(seri *catalog.CatalogSeri) (*Series, error)
type SeriesFinder interface{ FindSeriByName(string) (*catalog.CatalogSeri, bool) } // *catalog.Catalog satisfies it

// ytpkg/format.go - THE single place for strings that reach YouTube
const titleSeparator = " | "
const coreMarker = "CORE"
const titleDateFormat = "January 2, 2006"
var speakerTitles = []string{"Pastor"}
const DefaultCategoryID = "29"; const DefaultLanguage = "en"
func FormatTitle(msg *catalog.CatalogMessage, name string) string   // [CORE] | name | speakers | date
func AlternativeTitle(msg *catalog.CatalogMessage, generated string) string
func PlaylistTitle(m catalog.Ministry, seriesName string) string   // "Ask the Pastor" handled by caller
func MessageDescription(msg *catalog.CatalogMessage, seri *catalog.CatalogSeri, summary string) string
func SeriesDescription(seri *catalog.CatalogSeri) string           // description + booklets
func Validate(title, description string) error                     // 100 chars, 5000 bytes, no < >
// moved here unchanged: stripSpeakerTitle, getTitleSpeakers, collectResources

// ytpkg/ref.go
func ParseVideoRef(s string) (id string, ok bool)
func ParsePlaylistRef(s string) (id string, ok bool)

// ytpkg/diff.go
type Change struct{ Field, Now, New string; Destructive bool }
func DiffMessage(live, want *Message) []Change   // managed fields only (Q5); playlists handled separately
func DiffSeries(live, want *Series) []Change     // description, privacy
func IsDestructiveDescription(now, new string) bool
func DescriptionDiff(now, new string) string     // unified line diff via github.com/pmezard/go-difflib (already indirect via testify; becomes direct)
func PlanReorder(items []PlaylistItem, tracks map[string]int) []Move   // Q21 order, LIS-minimal moves
type Move struct{ ItemID, VideoID, Title string; From, To int }

// ytpkg/print.go
func (m *Message) Print(b *box.Box)     // replaces UploadPacket.Print: same sections, one Playlist line and copy line per playlist
func (m *Message) PrintLive(b *box.Box) // `get` view: all fields, thumbnail URL
func (s *Series) Print(b *box.Box)
func PrintChanges(b *box.Box, changes []Change, descDiff string)
```

`normalizeForMatch` (`cmd/messagetype.go:79`) is needed by `AlternativeTitle` and by the near-match search. It moves to `util.NormalizeForMatch`, and `cmd` calls it from there.

### New package `ytclient` (YouTube API access, like `gclient` for Sheets)

```go
// ytclient/auth.go
type AuthConfig struct{ ClientID, ClientSecret, TokenDir string }
func TokenPath(dir string, ch ytpkg.Channel) string                 // youtube-token-wol.json etc.
func HTTPClient(ctx context.Context, cfg AuthConfig, ch ytpkg.Channel) (*http.Client, error)
	// loads the token, or runs the loopback consent flow on localhost:8080;
	// wraps the TokenSource so refreshed tokens are saved

// ytclient/client.go
type Client struct{ svc *youtube.Service; ChannelID string; playlists []*youtube.Playlist /* cached per run */ }
func New(ctx context.Context, httpClient *http.Client, opts ...option.ClientOption) (*Client, error) // reads channels.list mine
func (c *Client) GetVideos(ids ...string) ([]*ytpkg.Message, error)      // batches of 50; part snippet,status
func (c *Client) GetPlaylist(id string) (*ytpkg.Series, error)            // with items, all pages
func (c *Client) FindPlaylists(title string) (exact []*ytpkg.Series, near []*ytpkg.Series, err error)
func (c *Client) PlaylistsContaining(videoID string) ([]ytpkg.PlaylistRef, error) // over the cached own playlists
func (c *Client) RecentUploads(n int) ([]ytpkg.PlaylistItem, error)       // UU… playlist
func (c *Client) UpdateVideo(want *ytpkg.Message) error      // re-reads live, copies only managed fields, videos.update part=snippet,status
func (c *Client) UpdatePlaylist(want *ytpkg.Series) error    // same pattern; never changes title
func (c *Client) InsertPlaylist(want *ytpkg.Series) (string, error)
func (c *Client) AddToPlaylist(playlistID, videoID string) (itemID string, err error)
func (c *Client) MoveItem(playlistID string, m ytpkg.Move) error
func (c *Client) SetThumbnail(videoID, path string) error
func (c *Client) Upload(m *ytpkg.Message, videoPath string, progress func(sent, total int64)) (string, error) // resumable, private

// ytclient/convert.go
func messageFromVideo(v *youtube.Video) *ytpkg.Message
func applyManagedFields(v *youtube.Video, want *ytpkg.Message) // the only place that mutates a live resource
```

Tests run against an `httptest.Server`, using `option.WithEndpoint(srv.URL)` and `option.WithHTTPClient(srv.Client())`.

### Changes in `cmd`
- `cmd/youtube.go`: the parent `youtube` command. Shared helpers:
  - `newYouTubeClient(ctx, ytpkg.Channel) (*ytclient.Client, error)`: reads `youtube-client-id` and `youtube-client-secret` (via viper), and `youtube-channel-ids`, and checks the channel.
  - `confirm(prompt string, defaultYes, honourYes bool) bool`: on the shared `getStdin()`.
  - `--refresh` and `--yes` are persistent flags on `youtube`, bound with viper.
- `cmd/youtube_get.go`: `youtube get`, with `--playlist` and `--json`.
- `cmd/youtube_sync.go`: `youtube sync`, with `--thumbnail`. Holds `type syncRun struct { cat *catalog.Catalog; yt *ytclient.Client; touched map[string]*ytpkg.Series; trackOf map[string]map[string]int }` and its methods `syncVideo`, `ensurePlaylists`, `sweepPlaylists`. `syncVideo` is also called from the audio flow.
- `cmd/catalogcache.go`: `readCatalogCached(ctx) (*catalog.Catalog, error)` and `catalogCachePath()`.
- `cmd/youtube_upload.go`: `publishMessage(run *syncRun, info *MessageInfo) error` (the step list, the duplicate guard, the upload record), plus `type stepPrinter` for the progress lines.
- `cmd/uploadrecord.go`: load and save `~/.wolm/youtube-uploads.json`.
- `cmd/audio.go`: `MessageInfo.Packet` becomes `*ytpkg.Message`. `processOneAudio` calls `ytpkg.NewMessageFromCatalog`. `printMessageInfo` calls `info.Packet.Print(b)`. After the boxes are printed, `audio()` offers publishing (Phase 6). `promptUserForMessageChoice` is generalised to `promptUserForChoice(candidates []catalog.CatalogMessage, title, heading string)` so that `sync` can reuse it for rows that share a video ID.
- `cmd/publish.go` is deleted. `cmd/publish_test.go`'s packet, title, playlist, privacy and description tests move to `ytpkg/*_test.go`. Its tests of title inference, date inference and sanitising stay in `cmd` (renamed `cmd/messagetype_test.go`).

### Sample `sync` output (one video)

```
╭─ zQ3xYk1 · Pay Attention ────────────────────────────────────────────────╮
│ Sheet   : Pay Attention  [Prayer, track 3]                               │
├─ Changes ────────────────────────────────────────────────────────────────┤
│ Privacy : now: unlisted                                                  │
│           new: public                                                    │
│ Category: now: 22                                                        │
│           new: 29                                                        │
│ Playlist: add to "Prayer" (track 3)                                      │
├─ Description ────────────────────────────────────────────────────────────┤
--- YouTube
+++ Sheet
@@ ... @@
-Pastor Mary talks about the importance of paying attention ...
+Sermon notes
+https://example.org/notes.pdf
├─ WARNING ────────────────────────────────────────────────────────────────┤
│ This removes YouTube description text that is not in the sheet:          │
Pastor Mary talks about the importance of paying attention ...
╰──────────────────────────────────────────────────────────────────────────╯
Apply these changes? [y/N]
```

## Tasks

The prompt's Phase 1 is Phases 1–2 here, its Phase 2 is Phases 3–5, and its Phase 3 is Phase 6.

### Phase 1: `ytpkg` and the formatting bottleneck (no network)
- [x] 1.1 Move `normalizeForMatch` to `util.NormalizeForMatch` (`util/string.go`) and update its callers in `cmd`. Files: `util/string.go`, `cmd/messagetype.go`, `cmd/audio.go`, `util/string_test.go`.
- [x] 1.2 Create `ytpkg/channel.go` and `ytpkg/privacy.go` (`ChannelForMinistry`, `IsCoreMinistry`, `PrivacyForView`, `IsPrivacyConsistent`, `ResolvePrivacy`), with tests.
- [x] 1.3 Create `ytpkg/format.go` (`FormatTitle` with the CORE marker, `AlternativeTitle`, `PlaylistTitle`, `MessageDescription`, `SeriesDescription`, `Validate`, the house defaults, and the moved speaker and resource helpers), with tests moved from `cmd/publish_test.go` and updated for the CORE format.
- [x] 1.4 Create `ytpkg/message.go`, `ytpkg/series.go` and `ytpkg/catalog.go` (`NewMessageFromCatalog` with the numbered-series playlist rule, Ask the Pastor and SAM exclusions; `NewSeriesFromCatalog`), with tests covering track 0, multiple series, Ask the Pastor, raw and CORE.
- [x] 1.5 Create `ytpkg/print.go` (`Message.Print`), porting the `UploadPacket.Print` tests, plus the one-line-per-playlist case.
- [x] 1.6 Switch `cmd/audio.go` to `ytpkg` (`MessageInfo.Packet`, `processOneAudio`, `printMessageInfo`). Delete `cmd/publish.go`, and keep the non-packet tests as `cmd/messagetype_test.go`. Update `cmd/audio_test.go`. `go vet ./...` and `go test ./...` pass.
- [x] **Checkpoint (you):** run `online audio` on a WOL video and on a CORE video. Check that the WOL output looks as it does today. The CORE title should read `CORE | <title> | <speaker> | <date>` and the playlist `CORE | <series>`. A track-0 message should show no playlist.

### Phase 2: YouTube auth, reading, and `youtube get`
- [x] 2.1 Create `ytpkg/ref.go` (`ParseVideoRef`, `ParsePlaylistRef`), with tests covering every URL form found in the sheet, including `https://youtu.beID` and the `watch?v=&list=` precedence.
- [x] 2.2 Create `ytclient/auth.go`: the loopback OAuth flow with PKCE and state, token load and save with refresh write-back, per-channel token paths. Test token round-trip and paths. The browser flow itself is tested manually.
- [x] 2.3 Create `ytclient/client.go` and `ytclient/convert.go`: `New` (with the channel check), `GetVideos`, `GetPlaylist`, `FindPlaylists`, `PlaylistsContaining`, `messageFromVideo`. Tests against `httptest`, including batching over 50 IDs and paging.
- [x] 2.4 Create `ytpkg.Message.PrintLive` and `ytpkg.Series.Print`, with tests.
- [x] 2.5 Create `cmd/youtube.go` (parent command, `newYouTubeClient`, flags) and `cmd/youtube_get.go` (`--playlist`, `--json`). Copy the yt-api client ID and secret into `~/.wolm/online-config.yaml`. That's your step; the plan's manual list says so.
- [x] **Checkpoint (you):** add `youtube-client-id` and `youtube-client-secret` to `online-config.yaml`. Run `online youtube get 01LiNfRzMo8`: the browser consent should appear once, then a box with the CORE title and its thumbnail link. Run it again and there should be no consent. Then try `online youtube get --json <a playlist link>`, and a TBO-owned video such as `VXHmO7546ps`, which should still be readable.
- [x] 2.6 Read playlist items in parallel (8 workers) in `ytclient` (`PlaylistsContaining`, mutex on the item cache), with `log.Printf` status lines (shown with `--verbose`): one before the scan, one after with the playlist and video counts and the time taken. Add `ytclient.PlaylistSnapshot`, `Client.SnapshotPlaylists` and `Client.UseSnapshot`. Tests: the snapshot is complete, and once a snapshot is used, `PlaylistsContaining` makes no requests. (Deviation 1)
- [x] 2.7 Create `util/cache` (`cache.Read` and `cache.Write`: 1-hour `cache.TTL` on the file's mtime, a `refresh` argument the caller takes from `--refresh`, write to a temporary file then rename) for 2.6 and 3.1 to share. `youtube get` caches the playlist snapshot at `<scratch-dir>/youtube-playlists-<channel>.json`, ignoring a cached snapshot for a different channel. Tests use a temporary dir and `os.Chtimes`. (Deviation 1)
- [x] **Checkpoint (you):** run `online youtube get -v 01LiNfRzMo8`. The scan should take a few seconds, with the status lines. Run it again: no scan, and a line saying the cache was used. Run it with `--refresh`: the scan runs again.

### Phase 3: cache, matching, and the read-only `sync` diff
- [x] 3.1 Create `cmd/catalogcache.go` (`readCatalogCached`, built on `cache.Read` and `cache.Write` from 2.7, plus the `--input` bypass), with tests using a temporary scratch dir and a fake reader.
- [x] 3.2 Create `ytpkg/diff.go` (`DiffMessage`, `DiffSeries`, `IsDestructiveDescription`, `DescriptionDiff`) and `PrintChanges`. Promote `github.com/pmezard/go-difflib` to a direct dependency. Tests cover every managed field, the Q8 privacy cases (private kept, partner private → unlisted) and the destructive rule (adding links isn't destructive; dropping a line is).
- [x] 3.3 Generalise `promptUserForMessageChoice` into `promptUserForChoice` (`cmd/audio.go`), keeping the audio behaviour and its tests.
- [x] 3.4 Create `cmd/youtube_sync.go`: the `syncRun` setup, video-ID index over the catalog, playlist-ref expansion, `syncVideo` up to and including the diff display and the approval prompt (Q13/Q19 defaults). In this phase an approved video prints `(writes not enabled yet)`. Tests for the index (duplicate IDs, malformed URLs) and for the prompt defaults.
- [x] **Checkpoint (you):** run `online youtube sync` on two or three CORE videos and one WOL video, and on a CORE playlist. CORE titles should show no title change. Review whether the diffs and warnings make sense, especially the descriptions. Nothing is written.

### Phase 4: `sync` writes for videos and playlist membership
- [x] 4.1 Add `ytclient.UpdateVideo` and `applyManagedFields` (re-read live, change managed fields only, `part=snippet,status`), `SetThumbnail`, `AddToPlaylist` and `InsertPlaylist`. The `httptest` tests assert that the update body still carries the unmanaged `tags`, `license`, `embeddable` and `publicStatsViewable`, and that the channel check refuses another channel's video.
- [x] 4.2 In `syncVideo`: apply approved changes; handle `--thumbnail`; `ensurePlaylists` (exact match, or near-match display and gated creation, then add and record as touched). Report errors per video and continue.
- [x] **Checkpoint (you):** make a private test video on WOL, and a copy of the dump (`online dump -o test.json`) with a fake row that points at it, in a series that has no playlist. Run `online youtube sync --input test.json <test video>`. Check the diff, approve it, then confirm in Studio that the title, description and privacy changed and that tags and language were preserved. Check that you were asked separately before the playlist was created, that the video is in it, and that a "Set Video ordering to Manual" reminder was shown for the new playlist. In Studio, note what Video ordering the new playlist started with. Re-run: it should report "in sync".

### Phase 5: end-of-run playlist sweep
- [x] 5.1 Add `ytpkg.PlanReorder` (Q21 order, LIS-minimal moves), with tests: already ordered (no moves), one out of place (one move), unnumbered items kept after the numbered ones in their current relative order.
- [x] 5.2 Add `ytclient.UpdatePlaylist` (never changes the title) and `MoveItem`, and `ErrManualSortRequired`, which `MoveItem` and `AddToPlaylist` return when the API's error reason is `manualSortRequired`. Test both with `httptest`, including that error.
- [x] 5.3 Add `syncRun.sweepPlaylists`: the playlist-field diff and approval, then the reorder display and approval, for touched playlists and those named on the command line. On `ErrManualSortRequired`, stop reordering that playlist and show the "Set Video ordering to Manual" section with its Studio link. Show the same reminder for every playlist created this run. List all of them in an end-of-run summary.
- [x] **Checkpoint (you):** add a second fake row (track 1) for another private test video, in the same test series. Sync both, and check that the sweep proposes and applies one reorder, with the track-1 video first. Edit the series description in your test JSON and re-sync: the playlist description diff should appear. In Studio, set the test playlist's Video ordering to *Date added (newest)*, swap the two tracks in the test JSON, and re-sync: the reorder should stop with the "Set Video ordering to Manual" message and a working Studio link. Set it back to Manual, re-sync, and the reorder should go through. Then run `sync` on a real CORE playlist and decline everything, as a read-only check that it proposes nothing surprising.

### Phase 6: upload from `audio`
- [x] 6.1 Create `cmd/uploadrecord.go` (load and save `~/.wolm/youtube-uploads.json`, keyed by absolute path and size), with tests.
- [x] 6.2 Add `ytclient.Upload` (resumable, private, house defaults, progress callback) and `RecentUploads`, with `httptest` tests.
- [x] 6.3 Create `cmd/youtube_upload.go`: the `stepPrinter` (numbered lines, in-place percentage, `FAILED` and `not run`), and `publishMessage` (the already-uploaded check, duplicate guard, proposal with gated playlist creation and planned moves, `[Y/n]`, the six steps, upload record written after step 1, then copy lines for the URL and description). Tests for `stepPrinter` output and for the order of the guard decisions.
- [x] 6.4 In `audio()`: after the boxes, call `publishMessage` for each message that has a package, sharing one `syncRun`, so the end-of-run sweep covers every touched playlist. A message for an unauthenticated channel is reported and skipped.
- [x] **Checkpoint (you):** process a short test video whose row points at a test series, with privacy `partner` so the final state is unlisted. Watch the steps, then check in Studio that the thumbnail, playlist, unlisted privacy and description are right. Copy the printed URL and description into your test JSON (or the sheet, for a real message), then re-run `audio` on the same file: no second upload, a sync diff instead. Re-run once more *before* pasting the URL, using a fresh test JSON without the URL: the duplicate guard should catch it.

## Test plan

### Automated (Claude)
- [x] `ytpkg`: CORE titles and playlists carry `CORE | `; WOL ones don't; sub-ministries are covered; honorifics are stripped from titles only.
- [x] `ytpkg`: track 0 gives no playlist; two numbered series give two playlists with their positions; Ask the Pastor gives only "Ask the Pastor"; SAM series give none.
- [x] `ytpkg`: privacy mapping for new items and the consistency table (public, private ↔ private or unlisted, partner ↔ unlisted); raw is refused.
- [x] `ytpkg`: `Validate` rejects a 101-character title, a description over 5000 bytes, and `<` or `>`.
- [x] `ytpkg`: `ParseVideoRef` and `ParsePlaylistRef` handle every URL form in the sheet, including `youtu.beID`; rumble and S3 links are rejected.
- [x] `ytpkg`: `DiffMessage` covers only the managed fields; `IsDestructiveDescription` is false when only links are added and true when a line is dropped.
- [x] `ytpkg`: `PlanReorder` gives minimal moves and keeps unnumbered items after the numbered ones, in their current order.
- [x] `ytpkg`: `Message.Print` keeps exact copy lines, as the old `UploadPacket.Print` tests did.
- [x] `ytclient`: the update request preserves unmanaged fields; another channel's video is refused before any write; `GetVideos` batches at 50; playlist listing pages.
- [x] `ytclient`: an API error with reason `manualSortRequired` comes back as `ErrManualSortRequired`; `cmd` turns it into the Studio reminder, and the run continues.
- [x] `ytclient`: the token file round-trips and is written with mode 0600 (where the OS honours it).
- [x] `cmd`: the cache is used inside the TTL, refreshed after it, bypassed by `--input`, and forced by `--refresh`.
- [x] `cmd`: approval defaults: Yes normally, honouring `--yes`; No for a destructive change or playlist creation, ignoring `--yes`.
- [x] `cmd`: the video-ID index handles duplicates (chooser) and malformed URLs.
- [x] `cmd`: the upload guard order (URL in the sheet → sync; record hit → sync; recent-title hit → sync; otherwise upload), and the `stepPrinter` lines for done, skipped, failed and not run.
- [x] `gclient/client.go` still has `sheetsScope = sheets.SpreadsheetsReadonlyScope`.
- [x] `go vet ./...`, `go test ./...` and `make build` pass.

### Manual (you)
- [x] Copy the yt-api client ID and secret into `~/.wolm/online-config.yaml` (Phase 2).
- [x] In Cloud Console, check that the OAuth consent screen's publishing status is "In production", not "Testing". Otherwise tokens expire every 7 days, for yt-api too. *(2026-10-17: left in Testing by choice for now; you'll re-authenticate weekly.)*
- [x] Phase 1 checkpoint: the audio output for CORE and WOL.
- [x] Phase 2 checkpoint: consent once, then `get` for a video, a playlist, JSON output, and a TBO-owned video.
- [x] Phase 3 checkpoint: read-only diffs on real CORE and WOL videos and a playlist.
- [x] Phase 4 checkpoint: writes on a private test video, with unmanaged fields preserved and gated playlist creation.
- [x] Phase 5 checkpoint: the reorder and playlist-field sweep.
- [x] Phase 6 checkpoint: the full upload, the re-run giving a sync instead of an upload, and the duplicate guard.
- [x] Afterwards, delete the test videos and test playlist in Studio.

## Deviations

1. **Playlist scan: parallel and cached (Phase 2, tasks 2.6 and 2.7).** At the Phase 2 checkpoint, `youtube get` on one video paused about 20 seconds after signing in. `PlaylistsContaining` was reading the items of all 164 channel playlists one at a time, at roughly 165–200 quota units, before printing anything. At your request, the scan now:
   - runs with 8 workers;
   - reports its start and its counts in `--verbose`;
   - is cached as `<scratch-dir>/youtube-playlists-<channel>.json`, like the catalog (1-hour TTL, `--refresh`).

   The TTL and file handling went into a shared package, `util/cache`, which 3.1 now reuses. It started as `cmd/cache.go` and was moved at your request, because it isn't specific to any command; the caller passes `--refresh` in, so the package doesn't read viper.

   **Caution for Phases 4–6:** the snapshot is up to an hour old. A write must not rely on it. Before `sync` or `audio` creates a playlist, adds to one or reorders one, it reads the live state. A run that writes to a playlist also deletes the snapshot file, so the next `get` doesn't show stale membership.

2. **An edited description line is not a loss (Phase 3, `ytpkg/diff.go`).** At the Phase 3 checkpoint, editing a sheet description from "ABC" to "AC" raised the WARNING about removing YouTube text. The Q9 rule compared lines exactly. A one-paragraph description is a single line, so any edit counted as losing the whole line. The tool can't tell an edit made in the sheet from text YouTube alone has, because it doesn't know the sheet's previous value. At your request (option 2), a YouTube line now counts as **edited**, not lost, when at least 60% of its words (`editedLineOverlap`, case and punctuation ignored) survive in one new line, or in two adjacent new lines joined, so splitting a paragraph is also an edit. An edited line shows in the diff with no WARNING, and approval stays `[Y/n]`. A line with no such counterpart still gets the WARNING, a `[y/N]` default and no `--yes`. Examples: a stand-alone "Watch live Sundays…" line, a hand-written description replaced by a different summary, or a paragraph mostly deleted.

3. **A stand-alone series is never found by name (Phase 4, `ytpkg.FindSeries`).** At the Phase 4 checkpoint, "The Importance of Voting" was given the new series "Voting", track 1, but `sync` planned no playlist. The sheet has no Series row for "Voting". It does have a message titled "Voting" (2026-10-03) in no series, and the catalog gives such a message a stand-alone series named after it, with a `SAM-` ID. Looking up "Voting" found that series, and the stand-alone rule dropped the playlist. A message that really is stand-alone has no series reference, or names `SAM`, which is checked before any lookup. So a `SAM-` series found by name is always a name collision. `ytpkg.FindSeries` now ignores stand-alone series and treats the series as undefined. Both the package builder and `sync`'s new-playlist code use it. The rule itself is unchanged: stand-alone series never become playlists.

4. **A playlist created this run is not read back before adding to it (Phase 4, `ytclient.AddToPlaylist`).** At the Phase 4 checkpoint, `sync` created the "Voting" playlist (`PLQ_IAAeTtEww`), then failed with "playlist … was not found" when adding the video. The ownership check before the add re-reads the playlist by ID, and YouTube's reads lag its writes, so the playlist created milliseconds before wasn't visible yet. It appeared a few minutes later, in the API and in Studio. The client now records the playlists it creates, and the check skips them, because they're the channel's by construction. Playlists that already existed are still read and checked.

5. **Writes YouTube asks to retry are retried (Phase 4, `ytclient/retry.go`).** With Deviation 4 in place, adding to a playlist created moments earlier then failed at the insert itself: `409 The operation was aborted`, reason `SERVICE_UNAVAILABLE`. A brand-new playlist accepts no items for a few seconds. A second run, a little later, worked. Polling until the playlist is readable was rejected: readable and accepting items aren't the same moment, and each probe costs quota. Instead:
   - `withRetry` retries a write while YouTube asks for that (409 "aborted", 5xx, `backendError`, `SERVICE_UNAVAILABLE`), waiting 1, 2, 4, 8 and then 16 seconds, about 30 seconds in all. It prints a "retrying in Ns…" line to `Client.Progress`, which `cmd` sets to stdout.
   - Any other error, such as quota, permission or a bad request, fails at once.
   - Adding to a playlist isn't idempotent, so before each retry `AddToPlaylist` re-reads the playlist, bypassing the per-run copy, and stops if the video got in after all. A failed-but-applied attempt isn't repeated.
   - Phase 5's `MoveItem` and Phase 6's create, add and reorder sequence use the same helper.

6. **Three judgement calls in the upload flow (Phase 6).** These are small decisions the plan left open. Each can be reversed if you'd rather.
   - **A playlist that isn't Manual doesn't fail the publish.** The plan says a failed step stops the rest and leaves the video private. Step 5 (reorder) getting `manualSortRequired` is reported as `skipped: set Video ordering to "Manual" (see below)`, with the Studio link, and step 6 still sets the target privacy. The video itself is fine, and the order can be fixed with a later `sync` on the playlist. Any other failure still stops the steps and leaves the video private.
   - **A new video not being found yet is retried.** Like a new playlist (Deviations 4 and 5), a video just uploaded can briefly be "not found". For videos this run uploaded only, `ytclient` retries "not found" with the same backoff, on the reads before the thumbnail and privacy steps and on adding to playlists.
   - **Only the summary is offered for the Description column.** The video's description is the summary plus resource links, but `sync` builds descriptions as the sheet's Description column plus resource links. Pasting the full description would list the links twice. The paste box offers just the generated summary, and leaves it out when the row already has it.
   - Also: a video in the upload record that YouTube no longer has, for example a deleted test upload, is dropped from the record with a note, so it doesn't block a fresh upload.
