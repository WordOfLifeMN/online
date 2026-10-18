# Open questions: round 1

Fill in each `Answer:` with an option number, plus any notes. Tell Claude when you're done.

Settled from the code, not asked:

- **The `audio` flow is reordered.** Today `processOneAudio` (`cmd/audio.go:343`) extracts the audio whenever the scratch `.mp3` is missing, and only then looks for a transcript. It will look for the transcript first, and skip both extraction and transcription when one is found. The Timeline already shows a skipped step as `reused`.
- **`audio summarize <video>.mp4`** looks for the transcript beside the video first, then falls back to the scratch directory, as it does today.
- **A reused transcript is announced on stdout**, for example `Using existing transcript: 2026-10-12 Sun Pay Attention.text`, not only in the `--verbose` log. Since regenerating means deleting the file by hand, you need to see when it was reused.
- **The `.mp3` stays a scratch intermediate**, swept after 24 hours as today.
- **Docs are updated** to say transcripts are now kept: the `audio` and `audio transcribe` help text, the README's "temporary" and 24-hour wording, and the `scratch-dir` comment in `TODO-km-online-config.yaml`.

## Q1. File extension

The prompt says to use whatever we use now. Today that's `.text`: faster-whisper's `text` format writes `<name>.text` (`getTranscribePathFromAudioPath`, `cmd/audio.go:946`). Up to now nobody opened these files by hand. Once they sit beside the videos, people will. Windows has no default program for `.text`, so double-clicking one asks which app to use. `.txt` opens in Notepad.

1. Keep `.text`, as the prompt says  **(recommended)**
2. Rename to `.txt` beside the video. The scratch copy stays `.text`, since that's what faster-whisper writes.

Answer: 2

## Q2. How the transcript gets beside the video

faster-whisper writes to whatever `--output_dir` it's given.

1. Transcribe into the scratch directory as now, check the result isn't empty, then copy it beside the video. A failed or empty transcription never leaves a file beside the video. Because the reuse check skips transcription whenever that file exists, a bad file there would block every later run until someone deleted it by hand.  **(recommended)**
2. Point `--output_dir` straight at the video's folder. Simpler, but a failed run can leave a partial or empty file there. The existing empty-file check would delete an empty one, but not one cut short by Ctrl+C.

Answer: 1

## Q3. When the video's folder can't be written to

The video might be on a read-only share, or in a OneDrive folder that is mid-sync.

1. Warn on stdout, keep the scratch transcript, and carry on with the summary. The transcript just isn't preserved this time.  **(recommended)**
2. Fail that message with an error.

Answer: 1

## Q4. Transcripts already in the scratch directory

Any message run in the last 24 hours has its transcript in scratch but not beside its video. The same is true after a Q3 failure.

1. When there's none beside the video but there is one in scratch, use it and copy it beside the video. This covers the switchover and recovers after a Q3 failure, with no separate migration.  **(recommended)**
2. Look only beside the video. A message without one there is transcribed again.

Answer: 1

## Q5. Standalone `audio transcribe <file>.mp3`

This subcommand takes an `.mp3`, not the video, so it doesn't know where the video is. The `.mp3` is usually in the scratch directory itself.

1. Leave it as it is: write to the scratch directory, swept after 24 hours. Preserving transcripts is the `audio` pipeline's job, and `audio summarize` still finds the scratch copy.  **(recommended)**
2. Write the transcript beside the `.mp3`, wherever that is.
3. Let it take an `.mp4` too. Given a video, it extracts the audio and writes the transcript beside the video.

Answer: 1

## Q6. Recording which model produced the transcript

The reason for keeping transcripts is to reprocess them later. By then you may want to know whether a transcript came from `small` or `large-v3`. Today nothing records it, and the summarizer sends the whole file to Claude, so a header inside the file would be sent too.

1. Don't record it. The file is plain transcript text, exactly what faster-whisper wrote.  **(recommended)**
2. Add a small sidecar file beside it, such as `<name>.transcript.json`, holding the model, the date and the source video.
3. Put a header line in the transcript, and have the summarizer strip it before sending.

Answer: 3

