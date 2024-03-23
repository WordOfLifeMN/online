echo off
echo "WOL, F&F, and CORE Message Video-to-Audio Converter"

REM Get the source file
SET source=%param%
IF "%~1" == "" set /P source=Enter video file to convert:
SET source=%source:"=%

REM Verify an input file was provided
IF "%source%" EQU "" (
  ECHO No input file
  pause
  EXIT /B 1
)

REM Test source file is mp4
IF "%source:.mp4=%" EQU "%source%" (
  ECHO Input file is not .mp4
  pause
  EXIT /B 1
)

REM Get the target file
SET target=%source:mp4=mp3%
SET target=%target:(video)=(audio)%
SET target=%target:(Video)=(Audio)%

REM Get the trim length
SET trim=9.9
IF "%source: FF =%" NEQ "%source%" SET trim=9.8
IF "%source: CORE =%" NEQ "%source%" SET trim=30.0

ECHO Converting: %source:C:\Users\WordofLifeMNMedia\Documents\WOLMessages\=%
ECHO         to: %target:C:\Users\WordofLifeMNMedia\Documents\WOLMessages\=%
ECHO   trimming: %trim% seconds
ECHO: 

"C:\Program Files\ffmpeg-6.1.1\bin\ffmpeg.exe" -hide_banner -loglevel warning -stats -i "%source%" -ss %trim% "%target%"

ECHO:
REM Get the s3 target
FOR %%i IN ("%target%") DO (SET s3name=%%~nxi)
SET s3target=s3://wordoflife.mn.audio/%s3name:~0,4%/%s3name%
SET s3url=https://s3.us-west-2.amazonaws.com/wordoflife.mn.audio/%s3name:~0,4%/%s3name: =+%

REM Prompt for upload
ECHO Uploading: %target:C:\Users\WordofLifeMNMedia\Documents\WOLMessages\=%
ECHO        to: %s3target%
SET /P isUpload="  Confirm: [Y/n]? " || SET isUpload=y
IF /I "%isUpload%" NEQ "y" (
  ECHO Aborting upload
  pause
  EXIT /B 1
)

ECHO:
ECHO %s3url%
ECHO:
aws s3 cp "%target%" "%s3target%"

pause
EXIT /B 0
