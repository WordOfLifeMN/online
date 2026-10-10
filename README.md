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
      ├─ faster-whisper ──► transcript  (temporary)
      ├─ Claude ──────────► title + description
      ├─ Google Sheet ────► series, track, ministry, visibility, resources
      └─ assemble ────────► UPLOAD PACKET, printed for copy-paste
```

Upload itself is manual: paste the packet into YouTube Studio.

The audio and transcript are intermediates and are never published, but they are
kept in the scratch directory for 24 hours. Re-running a message within that
window reuses them instead of extracting and transcribing again, which saves
several minutes on a full service and matters when a run fails late. Each run
sweeps whatever is already older than that; `--keep-intermediates` disables the
sweep entirely.

```
online audio "2026-03-08-v Walking in Faith.mp4"
```

Or run `bin/wolm-audio.bat`, which launches the same thing and prompts for the
video file so it can be dragged into the window.

The message is matched to a spreadsheet row by **date and type**. A service
usually produces both a prayer and a message on the same date, so type is what
tells them apart; it is inferred from the file name (`p` next to the date means
prayer).

A file name can only tell a prayer from everything else, so an inferred
`message` is read as "not a prayer" rather than literally - a date carrying both
a training and a message keeps both as candidates. An explicit `--type` is taken
literally instead.

Where several rows still match, the choice is offered rather than guessed. The
candidates are ranked by how closely the title in the file name resembles the
spreadsheet name, with the closest preselected when it beats the rest outright,
so accepting it is one keystroke. `--track` picks a row by its track number
without being asked.

The speaker comes from the spreadsheet row and is not confirmed. Use
`--speaker` to override it.

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
| `scratch-dir` | Where intermediate audio and transcripts are written | `~/.wolm/scratch` |
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
