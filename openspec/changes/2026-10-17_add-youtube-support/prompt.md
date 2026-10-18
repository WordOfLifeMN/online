# Goal
Add YouTube SDK support to the project to enable some YouTube automation.

# Requirements

## Phase 1

- Add a new command to this project called `youtube`. This command will collect sub-commands related to YouTube (YT) operations.
- sub-command `online.exe youtube get` takes a youtube video or playlist reference and outputs a "youtube package". 
- The youtube package (which I'm going to refer to as "ytPackage") should be a general structure reused throughout the project to describe a message (video) or series (playlist) on YT
  - having a separate structure for YT messages/videos (i.e. ytMessage) and YT series/playlists (i.e. ytSeries) is fine. Herein I'm referring to both as "ytPackage" even though it may refer to either a ytMessage or ytSeries depending on the context
  - for example the current `audio` workflow should also output a ytPackage (or ytMessage & ytSeries) instead of whatever it is outputting now
  - the ytPackage should be a union of what YT tracks through it's API as well as what this tool might generate. It includes, but is not limited to: Name, Description, Visibility, Thumbnail, Playlist, etc)

## Phase 2
- sub-command `online.exe youtube sync`: 
  - takes a youtube video
  - retrieves a ytPackage from YT
  - creates a ytPackage from the spreadsheet
  - Presents a side-by-side comparison of the changes from spreadsheet -> YT (side-by-side is a stretch goal, any diff is fine initially)
  - Prompts user whether changes should be written to YT
  - If approved, write changes to YT to make it match the spreadsheet

## Phase 3
- update the `audio` workflow so that, at the end of the process, present the proposed ytPackage for the video and, if approved, upload and create the video and update or create the playlist for the series

# Implemenatation notes
_These are my ideas, feel free to ignore or propose better alternatives_

Since this may involve updating many YT videos in a session, spreadsheet may be cached in the scratch directory in the JSON 'dump' format for reuse. Only refreshing when missing or every hour (or two or once per day?)

Thumbnail may be upload only? there is already an algorithm for identifying the thumbnail for a video on the local disk, which may involve correlating the YT video with a local directory?

