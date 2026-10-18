# Open questions: round 2

Fill in each `Answer:` with an option number, plus any notes. Tell Claude when you're done.

Settled from round 1 notes, for the record. These go into the plan:
- **Q1:** the yt-api OAuth client's ID and secret are copied into `online-config.yaml` as `youtube-client-id` and `youtube-client-secret`. Tokens go in `~/.wolm/`. Sign-in goes through `http://localhost:8080`, the same as yt-api.
- **Q7:** title and description rules live in one place, `ytpkg/format.go`. That file holds the components (CORE marker, title, speaker, date), the separator, the description layout, and the playlist name. Nothing else builds these strings. CORE ministries, including the sub-ministries, get a leading `CORE` component in both video titles and playlist names.
- **Q8:** if YouTube's privacy is consistent with the sheet, it's kept. Otherwise, or for a new video or playlist, it's mapped from the sheet. Consistent means: sheet public = YouTube public; sheet private = YouTube private or unlisted; sheet partner = YouTube unlisted. The mapping for a new or inconsistent item is public → public, private → unlisted, partner → unlisted.
- **Q15:** `get` and `sync` show the link to YouTube's largest current thumbnail (`maxres`, falling back to `high`), so you can open it and compare.

## Q19. Approval default when a description would be overwritten

Follows Q9 and Q13. Q13 makes the per-video prompt default to Yes, with `--yes` skipping it. Q9 makes an overwritten description a warning you can abort on. With both as they stand, pressing Enter, or passing `--yes`, would delete YouTube text without you reading the warning.

Here, "would be overwritten" means YouTube's current description is not empty and some of its lines are missing from the new one. Adding resource links doesn't count.

1. For a video with this warning, the prompt defaults to No and `--yes` doesn't skip it. You must type `y`. All other videos keep the default of Yes.  **(recommended)**
2. Default No for that video, but `--yes` still applies it.
3. Keep the default of Yes everywhere. The warning is information only.

Answer: 1

## Q20. The generated summary vs. the sheet's description

Follows Q9 and Q18. In Phase 3, the audio flow uploads with the Claude summary as the description. That summary exists only on YouTube. Under Q9, the next `sync` of that video replaces it with the sheet's `Description` column, which is usually empty for a new message. So the next sync would wipe a summary you've just approved.

1. After the upload, print the summary as a copy line beside the new video URL (Q18), for you to paste into the sheet's `Description` column. Until it is pasted, `sync` shows the Q19 warning for that video.  **(recommended)**
2. Same, and the audio flow also refuses to finish until you confirm you've pasted both.
3. `sync` keeps YouTube's description when the sheet's `Description` is empty, which reverses part of Q9.

Answer: 1. I would like to get to the point where this can be written to the spreadsheet by the app, but I'm having trust issues - the sheet is the system of record and a mistake there could destroy a lot of important data. This is out of scope for this plan, but a reliable backup plan is needed before I'm willing to allow this. Not to mention our IT dept **really** wants the sheet migrated to OneDrive and Excel :-(

## Q21. Reordering a playlist: videos the sheet doesn't place

Follows Q12. Your end-of-run sweep is the plan: syncing a video only adds it to its playlist, and each playlist that was touched is then reordered once, with its own approval showing the current and proposed order. Only the moves needed are made, because each move costs 50 quota units. But a playlist can hold videos the sheet doesn't give a position to: a track-0 extra, a video from another series, or one not in the sheet at all.

1. Videos the sheet numbers come first, in track order. All others follow, in their current relative order.  **(recommended)**
2. Leave the others where they are, and only reorder the numbered videos among the slots those videos already occupy.
3. Don't reorder a playlist that holds anything the sheet doesn't number. Report it instead.

Answer: 1

## Q22. Syncing the playlist itself

Follows Q12 and your Q8 note ("videos/series"). A playlist has its own title, description and privacy. The sheet side for those is the series name (via the Q7 rules), the series row's `Description` plus booklets, and the series visibility.

1. Yes. Each touched playlist gets the same diff and approval, during the end-of-run sweep, before it's reordered. The Q8 and Q19 rules apply.  **(recommended)**
2. Only when a playlist was named on the command line. Playlists touched through a video are just reordered.
3. No. The playlist's own fields aren't synced in this change.

Answer: 1. However, the current sheet does not record any playlist IDs and relies on the title alone, so syncing titles is impossible since a changed title will be interpretted as a new playlist. Go ahead and implement this, but add another out of scope note to add playlist links or ids to the sheet somehow

## Q23. Progress through the upload (Phase 3)

Follows Q16. You asked for a recommendation. This is what I'd suggest: a numbered step list, one line per step, each finished as it completes. The upload line updates in place with a percentage. No emoji, because their width varies (the reason `⚠` was dropped in the unify-audio-output change). If a step fails, the line says so and the remaining steps show as not run. For example:

```
Publishing "Pay Attention | Mary Peltz | October 4, 2026"
  1/6  Upload video (1.2 GB) ......... done  4m12s
  2/6  Set thumbnail ................. done
  3/6  Create playlist "Prayer" ...... skipped (exists)
  4/6  Add to playlist ............... done
  5/6  Reorder playlist .............. 3 moves, done
  6/6  Set privacy: public ........... done
```

1. As shown.  **(recommended)**
2. As shown, inside a `util/box` box like the rest of the audio output.
3. Simpler: one plain line per step as it finishes, with no numbering and no in-place updates.

Answer: 1

## Q24. A duplicate upload before the sheet has the URL

Follows Q17. Q17 avoids a second upload when the sheet row already has a `Video` URL. But under Q18, you paste that URL in by hand. If the audio flow is run again before you've pasted it, for example after a later step failed, nothing would stop a second upload of the same message.

1. Before uploading, check the channel's 50 most recent uploads (1 quota unit) for the exact title. If there's a match, don't upload; switch to the sync path for that video, and print its URL for the sheet.  **(recommended)**
2. Keep a local record in `~/.wolm/` of each video file uploaded and its video ID, and check that.
3. Both.

Answer: 3
