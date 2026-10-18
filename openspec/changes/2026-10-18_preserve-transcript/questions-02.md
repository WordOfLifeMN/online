# Open questions: round 2

Fill in each `Answer:` with an option number, plus any notes. Tell Claude when you're done.

Settled from round 1, not asked:

- **Beside the video the transcript is `<video name>.txt`, and in scratch it stays `<name>.text`** (Q1). `audio summarize` will accept a `.txt` file as input as well as `.text`.
- **The summarizer drops the header line only when it's there**, so transcripts without one, including any written by hand, are sent unchanged (Q6).

## Q7. What the header line says and looks like

Follows Q6. The header has to be easy to recognise, so that only it gets stripped and never a first line someone actually spoke. It also has to make sense to someone who opens the file in Notepad.

1. A fixed marker, then the details, then a blank line:
   `#wolm-transcript model=small transcribed=2026-10-18`
   The model and the date are what a later reprocessing run would want to know.  **(recommended)**
2. The same, plus the source video's file name and the trim length used (`video="2026-10-12 Sun Pay Attention.mp4" trim=9.8`). The video name repeats the transcript's own file name, but it survives if the file is renamed or moved.
3. The model only: `#wolm-transcript model=small`

Answer: 1. Additionally, use "# " as the prefix (common script comment format), and include the app (faster whisper) in addition to the model in case we switch trascription software later

## Q8. When the header is written, and transcripts that predate it

Follows Q6 and Q4. A transcript copied from scratch might have been made earlier, under a different `whisper-model` setting, so the model in effect when it's copied isn't necessarily the one that made it.

1. Write the header into the scratch file as soon as transcription finishes, and copy the file beside the video unchanged. Any transcript made after this change carries its true model, wherever it's copied from, and so does one from standalone `audio transcribe`. A scratch transcript from before this change is copied without a header.  **(recommended)**
2. Add the header only when copying beside the video, naming the model configured at that moment. Simpler, but it can name the wrong model for a transcript copied from scratch.
3. Like 2, but a transcript copied from scratch gets `model=unknown`.

Answer: 1

