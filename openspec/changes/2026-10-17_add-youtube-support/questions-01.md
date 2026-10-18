# Open questions: round 1

Fill in each `Answer:` with an option number, plus any notes. Tell Claude when you're done.

Some things are already settled from the code and your earlier rules, so they aren't asked here. They'll go into the plan:
- The API client is `google.golang.org/api/youtube/v3`. It's already in the module graph through `google.golang.org/api v0.189.0`, so no new top-level dependency is needed.
- Every write is a read-modify-write: fetch the live `snippet` and `status`, change only the fields being synced, and send the whole thing back. This is the lesson from `etc/yt-api/CLAUDE.md`, where `videos.update` blanks any mutable field the request body leaves out.
- Before any write, check that `snippet.channelId` is the authenticated channel. A video on another channel is reported, not attempted (the two TBO-owned CORE videos, D1 in `core-retitle.yt-api.txt`).
- A video reference can be an ID, a `youtu.be/…` link, a `watch?v=` link, or a `live/` or `shorts/` link. The parser also accepts the three malformed `https://youtu.beXXXX` URLs in the sheet. A `watch?v=…&list=…` link means the video unless `--playlist` is given.
- Track 0 is never a playlist position, and never puts a message in that series' playlist (your track-0 rule). Note that today's `getPlaylistName` uses `Series[0]` even when its track is 0, so this also changes the audio packet.
- A sheet row is matched to a YouTube video by the video ID in its `Video` column. When several rows carry the same ID, you're asked to choose, using the existing chooser.
- The Google Sheets scope stays read-only.

## Q1. Where the YouTube OAuth client comes from

The current Google credential (`~/.wolm/credentials.json`) is a service account. A service account can't act as a YouTube channel, so YouTube needs a user OAuth flow: a browser consent page the first time, then a saved refresh token. yt-api already has an OAuth client ID and secret in `~/.yt-api/config.yaml`, with a token in `~/.yt-api/tokens.json`. That token was granted the full `youtube` scope.

1. Our own files: an OAuth client JSON downloaded from Google Cloud Console to `~/.wolm/youtube-client.json`, and our own tokens under `~/.wolm/`. It can be the same Cloud project yt-api uses.  **(recommended)**
2. Read yt-api's `config.yaml` for the client ID and secret, but keep our own tokens in `~/.wolm/`.
3. Reuse yt-api's client and its `tokens.json` directly.

Answer: 1, using the yt-api client's ID and secret copied into online-config.yaml

## Q2. YouTube OAuth scopes

You narrowed the Sheets credential to read-only on purpose. The same idea can apply here. `youtube get`, and the diff part of `sync`, only need `youtube.readonly`. Writing metadata needs `youtube.force-ssl`, and uploading needs `youtube.upload`.

1. Two tokens: a read-only token for `get` and diffs, and a separate write token (`force-ssl` and `upload`). The write token is only requested, with its own consent page, the first time you approve a write.  **(recommended)**
2. One token with the full `youtube` scope, the way yt-api does it.
3. One token with exactly `force-ssl` and `upload`, used for both reads and writes.

Answer: 2

## Q3. Which channels

An OAuth token belongs to one channel. WOL hosts wol, core and ask-pastor. Faith & Freedom and TBO are separate channels, and each would need its own consent and token.

1. Build tokens per channel (keyed WOL / Faith & Freedom / TBO), but only set up and test WOL now. Messages for the other channels are reported as "not authenticated for this channel".  **(recommended)**
2. Set up all three channels in this change.
3. WOL only, with no per-channel design.

Answer: 1. The volume on the other channels is low enough that I probably will continue to operate those manually, but let's be forward-looking just in case

## Q4. ytPackage types and where they live

The prompt suggests a ytMessage and a ytSeries. The audio flow's `UploadPacket` (`cmd/publish.go:44`) is already a package in all but name. Its comment says it is a struct "so that automating the upload later means serializing it to the YouTube Data API".

1. New package `ytpkg` with `ytpkg.Message` and `ytpkg.Series`. `UploadPacket` and the builder functions (`getUploadTitle`, `getPlaylistName`, and the rest) move into it and `UploadPacket` is deleted, so the audio flow prints a `ytpkg.Message`.  **(recommended)**
2. Same types, but in `cmd/`, next to the code that uses them.
3. Keep `UploadPacket` for the audio flow and add separate YouTube types beside it.

Answer: 1

## Q5. Which fields `sync` manages

The package holds every field YouTube has (title, description, privacy, playlist and position, category, tags, languages, made-for-kids, license, embeddable, publish date, thumbnails, channel, ID), plus the fields this tool generates (alternative title, risk notes, local thumbnail path). Only some of these come from the spreadsheet.

1. `sync` compares and writes only the fields the sheet determines: title, description, privacy, and playlist membership and position. Every other field is shown by `get` and left untouched by `sync`.  **(recommended)**
2. The same, plus category, language and made-for-kids, forced to house defaults (category 29 "Nonprofits & Activism", `en`, not for kids).
3. Every field that has a sheet or house-default value.

Answer: 2

## Q6. Output of `youtube get`

1. A human-readable box (`util/box`, the same style as the audio output) by default, or JSON with `--json`.  **(recommended)**
2. JSON only, like `dump`.
3. The box only.

Answer: 1

## Q7. The CORE title and playlist format doesn't match the channel

The CORE retitle you ran on 2026-10-12 made live titles `CORE | <title> | <speaker> | <date>` and playlists `CORE | <series>`. But the code that builds a package from the sheet produces `<title> | <speaker> | <date>` (no prefix, `cmd/publish.go:225`) and playlists named `CORE: <series>` (`corePlaylistPrefix`, `cmd/publish.go:36`). Left as it is, `sync` would propose reverting all 45 retitled videos and all 16 playlists.

1. Change the code to match the live channel: CORE titles get the `CORE | ` prefix, and the playlist prefix becomes `CORE | `.  **(recommended)**
2. Keep the code as it is. `sync` will propose changes on every CORE video.
3. Something else (say in the notes).

Answer: 1. I guess that's another requirement? I would like to extract the rules for creating titles (components like title, speaker, date) and the separator (currently '|'). All core messages and series must have an additional component "CORE" at the start. There may be other formats for other ministries eventually too. Same with descriptions: currently it's the description, blank line, resources, but there has been some talk of adding other metadata there too (i.e. moving speaker names into the description for example). That's a future problem, but we can prepare for it now by making sure there is a single bottleneck where we can standardize those decisions. That bottleneck can be a separate formatter package or methods in Message and/or Series, just so long as there is only one place where those decisions are made

## Q8. Private on YouTube vs. unlisted from the sheet

`getYouTubePrivacy` maps both `private` and `partner` to unlisted. But many live videos are actually *private* on YouTube (32 of the 42 track-0 rows with video, for example). A strict comparison would propose making all of them unlisted, which makes them reachable by link.

1. Treat YouTube "private" as satisfying sheet `private` and `partner`. Only flag a mismatch across the public/non-public line (sheet public but not public on YouTube, or the reverse).  **(recommended)**
2. Enforce the mapping strictly: everything that isn't public becomes unlisted.
3. Change the mapping so sheet `private` means YouTube private, and only `partner` means unlisted. This also changes what the audio flow uploads.

Answer: other? when "syncing": keep the current youtube settings if they are consistent with the sheet. If they are inconsistent, or there is no youtube setting yet, then map from the sheet. The rules are:
For inconsistent or new videos/series: sheet public -> youtube public, sheet private -> youtube unlisted, sheet partner -> youtube unlisted.
For pre-existing videos: sheet public -> youtube public, sheet private -> youtube private OR unlisted are acceptable, sheet partner -> youtube unlisted

## Q9. Description when the sheet has none

`sync` has no transcript, so the sheet-side description is the sheet's `Description` column plus the resource links. Many rows have an empty `Description`, while YouTube has some text (often just the title, e.g. `01LiNfRzMo8` is "Learned Helplessness"). A summary written by the audio flow lives only on YouTube, never in the sheet, so syncing would erase it.

1. Leave the description alone when the sheet's `Description` is empty. When it's set, sync the description plus resource links.  **(recommended)**
2. Never let sync overwrite YouTube's text. Only make sure the resource links are present at the end of it.
3. Always sync the description, even to empty.

Answer: Always sync the sheet description (and resource list) into the youtube description, even if it means deleting the youtube description. Flag these as warnings in the output (with a copy of what will be overwritten) and allow user to abort (or skip if we end up supporting lists of videos) syncing that video/playlist

## Q10. Which playlists a message belongs in

A message can name several series. Today only `Series[0]` is used (`getPlaylistName`, `getPlaylistPosition`).

1. One playlist for each series the message has a non-zero track in, at that track's position. Ask the Pastor messages still go only into "Ask the Pastor".  **(recommended)**
2. Only the first series with a non-zero track.
3. `Series[0]` as today (but never at track 0).

Answer: 1. Note that sorting out the complexities of messages being in multiple series is out of scope for this plan and needs fine tuned later

## Q11. What `sync` takes

The prompt says `sync` "takes a youtube video", but also expects many updates in one session.

1. One or more video references as arguments, each diffed and confirmed in turn. One run, one sheet load.  **(recommended)**
2. That, plus a playlist reference, which syncs every video in the playlist.
3. Exactly one video per run.

Answer: 2. Just to be clear: support multiple videos in one run, and a playlist is treated as if it was a list of every video in the playlist

## Q12. What `sync` does to playlists

1. Add the video to each expected playlist, or fix its position there. Never remove it from any playlist, and never create a playlist: a missing one is reported.  **(recommended)**
2. Same, but also create a missing playlist, with privacy from the series visibility.
3. Same as 2, and also remove the video from playlists the sheet doesn't name.

Answer: 2. Although I have 2 concerns: I don't want to create arbitrary playlists because I didn't specify the requirements correctly so creating a new playlist should be gated on user approval (I will remove this requirement later if everything works fine), and "fixing it's position" implies "moving it to a new position in the playlist" which may de-sync other videos in the playlist. Would it be better to maybe have a specific command or option to "sync the order of videos in the playlist"? Maybe syncing videos means "put it in the playlist" and make a list of touched playlists, then do a sweep at the end to re-order all videos in every touched playlist to match the sheet?

## Q13. How approval works

I'll show short fields (title, privacy, playlists) as a two-column old → new table, and the description as a line diff.

1. One `Apply these changes? [y/N]` per video. The default is No, so you have to type `y`.  **(recommended)**
2. A yes/no for each changed field.
3. One yes/no per video, plus `--yes` to skip the prompt.

Answer: 3. when prompting, default to `yes` instead of `no`

## Q14. Spreadsheet cache

The cache would be an `online dump`-format JSON file in the scratch directory. The scratch sweep only deletes `.mp3` and `.text`, so it would survive. The sheet takes a couple of seconds to read.

1. Used by the `youtube` commands only, refreshed when it is more than 1 hour old, and `--refresh` forces a re-read. `audio` keeps reading the sheet fresh, because you've usually just edited the row.  **(recommended)**
2. Same, but 24 hours.
3. No cache. Read the sheet on every run.

Answer: 1

## Q15. Thumbnails in `sync`

The API can set a thumbnail but can't return the original image you uploaded, so there's no reliable way to tell whether YouTube's thumbnail is "different". The local thumbnail search (`getThumbnailPath`) needs the video file's folder, which a YouTube video doesn't point to.

1. `sync` never compares thumbnails. `--thumbnail <path>` uploads one explicitly, and it's included in the same approval.  **(recommended)**
2. Add a configured media root, find the message's folder from its date, run `getThumbnailPath`, and offer to upload whatever it finds every time.
3. No thumbnails in Phase 2 at all.

Answer: 1. However, note to follow up: can I get a link to the current thumb for comparison?

## Q16. Upload sequence in the audio flow (Phase 3)

Uploading straight to public notifies subscribers before the thumbnail and playlist are set. An upload costs about 1,600 of the default 10,000 daily quota units, so roughly six uploads a day.

1. Upload as private, set the thumbnail, add to playlists (creating the playlist if needed), then switch to the final privacy as the last step. A failure part-way leaves the video private.  **(recommended)**
2. Upload with the final privacy directly, then thumbnail and playlist.
3. Upload as private and stop there. You make it public in Studio.

Answer: 1, with status notifications. I'd like a simple, clear step-wise progress  through the whole process, but I'm not sure what that looks like yet. Recommendations welcome

## Q17. A message that already has a video

If the sheet row already has a YouTube URL in `Video`, the message has already been uploaded. A second upload would make a duplicate.

1. Don't offer an upload. Run the `sync` diff against the existing video instead.  **(recommended)**
2. Warn, and offer the upload anyway, defaulting to No.
3. Don't check.

Answer: 1. Idempotency is a goal here. There are multiple use cases, and we want to do as little damage as possible.
This should have been in the prompt, but use cases are:
UC1. fix broken things: the list of videos are known to be broken (old data, recent sheet edits, etc) and we are fixing them
UC2. hygiene check: running through some videos to to verify they are consistent with the sheet, no changes is "noraml" result so prompts are not needed - if all visible data is correct, then assume other data (i.e. thumbnails) is correct too (unless overridden by parameter: a `--thumbnail` param should always trigger an upload)
UC3. weekly uploads handled entirely by the script
UC4. weekly upload where I (or other) manually uploaded for some reason, and am really running UC1 to sync the data I didn't set during upload

## Q18. Recording the new video in the spreadsheet

After an upload, the sheet's `Video` column needs the new URL. Otherwise the next `sync`, and the website's catalog, can't connect the row to the video. The Sheets scope is read-only, and per your rule I won't widen it without your explicit approval.

1. Keep Sheets read-only. Print the new URL as a copy line, with the row it belongs to, for you to paste into the sheet.  **(recommended)**
2. Widen the Sheets scope so the tool writes the `Video` cell itself. This needs your explicit approval, and is outside the read-only rule.
3. Print it, and also append it to a log file in `~/.wolm/` so you can paste several at once.

Answer: 1
