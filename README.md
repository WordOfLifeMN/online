Application to manage the Word Of Life Ministries media workflow.

# Status

This is a GoLang application that prepares recorded messages for publication to
YouTube. Given an edited video file it extracts the audio, transcribes it,
generates a suggested title and description, and assembles everything needed to
upload the message by hand.

The website builds its media catalog by querying YouTube, so this application no
longer generates a website. The last version that did is tagged `v1-static-site`.

# The workflow

```
  edited .mp4
      │
      ├─ ffmpeg ──────────► .mp3        (temporary)
      ├─ faster-whisper ──► .txt        (kept beside the .mp4)
      ├─ Claude ──────────► title + description
      ├─ Google Sheet ────► series, track, ministry, visibility, resources
      └─ assemble ────────► UPLOAD PACKET, printed for copy-paste
```

Upload itself is manual: paste the packet into YouTube Studio.

The transcript is kept beside the video, with the same name and a `.txt`
extension. It is never published, but transcribing is slow and the file is tiny,
so it is kept for later use: regenerating descriptions when the rules change,
keywords, searching. Its first line records how it was made:

```
# wolm-transcript app=faster-whisper model=small transcribed=2026-10-18
```

A video with a transcript beside it skips extraction and transcription entirely.
To transcribe it again, delete the `.txt`.

The `.mp3` is an intermediate. It is kept in the scratch directory for 24 hours,
so a message transcribed again within that window does not need its audio
extracted again. Each run sweeps whatever is already older than that;
`--keep-intermediates` disables the sweep entirely.

```
online audio "2026-03-08-v Walking in Faith.mp4"
```

Or run `bin/wolm-audio.bat`, which launches the same thing and prompts for the
video file so it can be dragged into the window.

A folder can be given or dragged in wherever a video can. It stands for every
`.mp4` directly inside it, in name order; sub-folders are not searched.

The message is matched to a spreadsheet row by **date**, and by nothing else.

Where a date carries several rows - a service usually produces at least a prayer
and a message - the choice is offered rather than guessed. The candidates are
ranked by how closely the title in the file name resembles the spreadsheet name,
with the closest preselected when it beats the rest outright, so accepting it is
one keystroke. `--track` picks a row by its track number without being asked.

Type and speaker used to be read out of the file name as well, and no longer
are. Both now come from the spreadsheet row, which is the record of what a
message is and who preached it. The markers were optional in practice, so their
absence could not be trusted to mean anything - a prayer whose file name omitted
the `p` had its row eliminated and the message beside it returned instead,
silently.

So a file name is read as:

```
  <date>[<discriminator>][-<obsolete markup>] <title>.<ext>

  2026-10-11 Servanthood.mp4
  2026-10-11b-mp Learning at Moriah.mp4
  2026-10-04 The Works of God-v.mp4
```

Only the date and the title mean anything. The discriminator is a single letter
that keeps two files of one date apart on disk, for your filing rather than for
this program, and the old speaker and prayer markers are tolerated wherever they
appear so that existing files still work.

The speaker comes from the spreadsheet row and is not confirmed. Use
`--speaker` to override it; you are only asked when no row names one.

A thumbnail is found in the message folder and reported in the packet as a local
path, for attaching by hand - it is never uploaded anywhere. An image sharing the
video's name wins; otherwise any image in the folder with "thumb" in its name,
which is how one series thumbnail covers every message in a series folder;
otherwise the generic artwork in `thumb-dir`.

# Commands

| Command | Purpose |
|---------|---------|
| `audio` | The full pipeline: video in, upload packet out |
| `audio extract` | Extract the audio only |
| `audio transcribe` | Transcribe an existing .mp3 only |
| `audio summarize` | Generate a title and description from an existing transcript |
| `dump` | Write the spreadsheet contents to a local JSON file |
| `check` | Validate the spreadsheet for internal consistency |
| `peek` | Check that the Google Sheet can be read |

# Configuration

Configuration is read from `online-config.yaml` in the current working
directory, then from `~/.wolm/online-config.yaml`. Command line parameters
override the configuration file.

| Key | Meaning | Default |
|-----|---------|---------|
| `sheet-id` | Google spreadsheet holding the series and messages | - |
| `anthropic-api-key` | API key for the church's Anthropic Console account | - |
| `anthropic-model` | Model used for titles and descriptions | `claude-haiku-4-5` |
| `whisper-model` | Transcription model | `small` |
| `whisper-exe` | Path to the faster-whisper executable | (see `audio_transcribe.go`) |
| `scratch-dir` | Where intermediate audio, and transcripts before they are copied beside the video, are written | `~/.wolm/scratch` |
| `thumb-dir` | Where the generic fallback thumbnails live | (none) |

**Never set `whisper-model` to `tiny.en`.** Measured over a real 80 minute
service, it writes an empty transcript while reporting success - so the failure
surfaces later, as a summary of nothing, rather than as an error. `small` takes
about 3 minutes per service and summarises identically to `large-v3`, which takes
18. The full comparison is in the `defaultWhisperModel` comment in
`cmd/audio_transcribe.go`.

## Credentials

**Google Sheets.** Create a Service Account and download the credentials file.
Copy it to `~/.wolm/credentials.json`, then share the spreadsheet with that
service account's email address.

Reference: https://github.com/juampynr/google-spreadsheet-reader

**Anthropic.** Put the church's API key in `~/.wolm/online-config.yaml` as
`anthropic-api-key`. It is handed to this process only.

Do **not** set `ANTHROPIC_API_KEY` as a user-wide environment variable. It would
shadow the credentials that Claude Code and other tools on the machine use, which
is how the church's key ends up paying for unrelated work. There is deliberately
no command line flag for the key either, so it stays out of shell history and the
process list.

If the config value is absent the SDK falls back to its normal resolution
(`ANTHROPIC_API_KEY`, then an `ant auth login` profile), so a machine set up that
way still works.

# Requirements

- `ffmpeg` on the path
- `faster-whisper` installed (see `whisper-exe` above)

# Testing

```
make test
```

Download the spreadsheet to a local JSON file:

```
make win-dump
```
