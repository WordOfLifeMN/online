# Goal
Preserve created transcripts

# Background
Creating a transcript is expensive, requiring extracting the audio track and running transcription software. 
Currently, transcripts are created in the scratch folder and discarded after 24 hours. 
However, transcripts may be useful for reprocessing later (i.e. backfilling if policies change), extracting keywords, searching, etc.
Transcripts are very small (compared to the video file), so saving them even if never used is a minor cost.

# Requirements
- Transcripts, once created, are preserved with with .mp4 used to create them.
- The name of the transcript is the same as the .mp4 file, but with a different extension (.txt, .vtt, .srt? Whatever we currently use).
- For the `audio` processing, an existing transcript prevents the audio extraction and transcription steps from running.
  - Specifically, AI processing (title, summary, risk analysis, keyword extraction, etc) uses existing transcripts rather than creating a new one whenever possible.

# Out of Scope
- Regenerating a transcript from scratch will be managed by the user deleting the existing transcription file before running and does not require special "refresh" or "override" support.
- Transcriptions are not uploaded anywhere (either S3 bucket or YouTube).
- 
