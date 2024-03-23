echo off
echo "WOL, F&F, and CORE Message Video-to-Audio Converter"

REM Get the source file
SET source=%param%
IF "%~1" == "" set /P source=Enter video file to convert: 
IF x%source%==x (
  ECHO No input file
  pause
  EXIT /b 1
)

REM Test source file is mp4
IF x%source:.mp4=%==x%source% (
  ECHO Input file is not .mp4
  pause
  EXIT /b 1
)

REM Get the target file
SET target=%source:mp4=mp3%
SET target=%target:(video)=(audio)%
REM SET target=%target:(Video)=(Audio)%

REM Get the trim length
SET trim=9.9
IF NOT x%source: FF =%==x%source% SET trim=9.8
IF NOT x%source: CORE =%==x%source% SET trim=30.0

ECHO Converting: %source:C:\Users\WordofLifeMNMedia\Documents\WOLMessages=...%
ECHO         to: %target:C:\Users\WordofLifeMNMedia\Documents\WOLMessages=...%
IF %trim%==9.9  ECHO   trimming: 9.9 seconds for Word Of Life Ministries
IF %trim%==9.8  ECHO   trimming: 9.8 seconds for Faith and Freedom
IF %trim%==30.0 ECHO   trimming: 30.0 seconds for C.O.R.E

REM ECHO ffmpeg -hide_banner -loglevel warning -stats -i %source% -ss %trim% %target%
"C:\Program Files\ffmpeg\ffmpeg-master-latest-win64-gpl\bin\ffmpeg.exe" -hide_banner -loglevel warning -stats -i %source% -ss %trim% %target%

pause
