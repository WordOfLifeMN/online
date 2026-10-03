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

Upload itself is manual: paste the packet into YouTube Studio. The audio and
transcript are intermediates and are deleted once the summary is generated
(pass `--keep-intermediates` to keep them).

```
online audio "2026-03-08-v Walking in Faith.mp4"
```

Or run `bin/wolm-audio.bat`, which launches the same thing and prompts for the
video file so it can be dragged into the window.

The message is matched to a spreadsheet row by **date and type**. A service
usually produces both a prayer and a message on the same date, so type is what
tells them apart; it is inferred from the file name (`p` next to the date means
prayer) and can be set with `--type`. If two rows share a date and type, pass
`--track` to say which.

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
| `anthropic-model` | Model used for titles and descriptions | `claude-opus-5` |
| `whisper-model` | Transcription model | `tiny.en` |
| `whisper-exe` | Path to the faster-whisper executable | (see `audio_transcribe.go`) |
| `scratch-dir` | Where intermediate audio and transcripts are written | `~/.wolm/scratch` |

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
