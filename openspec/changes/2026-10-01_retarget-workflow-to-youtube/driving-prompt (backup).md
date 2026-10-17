/openspec-proposal This branch is to make changes to the application to support a new website.

Under the old website, we stored all the data in a google spreadsheet, processed a lot of things  automatically, then built a static HTML website for uploading to AWS S3. At the start of this branch, this appliacation is to implement a workflow to support that website.

The goal of this proposal is to change this workflow to support the new website. Characteristics of the new website is that it constructs the media catalog dynamically by querying for YouTube playlists (series), videos (messages) and podcasts. It also (maybe?) queries OneDrive for other resources (booklets?) as well.

What I need to explore and make a plan for is to change this application/workflow to remove a number of things that will no longer be required, preserved on the new website, as well as changing the way we do some things as well.

My preliminary list of things that I'll want to change follows:
(note that we need to probably /grill-me or something to establish a better description of these)
1. We will no longer support the "Sunday Service" podcast
  a. therefore we will no longer need to preserve or upload audio files
  b. no more support for Spotify, YouTube podcast, or Amazon Music accounts (I don't think any of that was encoded in this workflow anyway)
2. Transcripts will no longer be searchable or visible. We do not need to preserve them, and any intermediate transcripts for AI processing may be significantly lower quality (just necessary for automatic titling and descriptions
3. YouTube will become the system of record for video messages and series. Later (not in this round), we will probably want the workflow to start managing the creation of playlists and videos, but for now that will be handled manually, just be aware that the YouTube metadata will take over for a large portion of the role we currently use the Google sheet for.
4. I will continue to use the Google spreadsheet for resource tracking myself, with the evenual goal of migrating to a OneDrive Excel file (not in this round)
5. Thumbnails will continue to be uploaded to YouTube, but we will not need to upload them to S3
6. Booklets are TBD, but the current plan is to have a directory structure on OneDrive with naming-convention-driven organization to replace the "Booklet" rows in the current Google drive

I'm thinking that the workflow may still need to be: I edit the raw video and save the final message versions locally (as I do now), then the workflow will take over to 
1. extract audio from the video
2. transcribe the audio to a transcript using the smallest/fastest/cheapest transcription model that is sufficient to the job (faster whisper already installed)
3. generate title and description from the transcription (title is mostly overwritten by the speaker's preference anyway)
4. upload the video to YouTube, inserted in playlist in series order. Including thumbnail and metadata in the title ("name | speaker | date") and description (short description, links to sermon notes or booklets (need to design this, I don't think we can link to OneDrive files, so sermon notes and booklets may need to continue to be uploaded to the public S3 bucket). This step will still be manual, but I would like the workflow to prepare the data for copy-pasting into YouTube where appropriate