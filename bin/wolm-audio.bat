ECHO off
ECHO "WOL, F&F, and CORE Message Video-to-Audio Converter"

C:\Users\WordofLifeMNMedia\Go\github.com\WordOfLifeMN\online\online.exe --verbose audio
pause

GOTO:eof

REM ===================================================================================
REM Everything below is for refernece only, all operations were moved into Go
REM ===================================================================================

REM Get the source file (without quotes)
SET source=%1
IF x%source% == x set /P source=Enter video file to convert: 
CALL :dequote %source%
SET source=%retval%

REM Verify an input file was provided
IF "%source%" EQU "" (
  ECHO No input file
  GOTO pexit
)

REM Verify source file is mp4
IF "%source:.mp4=%" EQU "%source%" (
  ECHO Input file is not .mp4
  GOTO pexit
)

REM Change to the target directory
FOR %%i IN ("%source%") DO (SET dir=%%~pi)
CD "%dir%"
CD
FOR %%i IN ("%source%") DO (SET mp4Name=%%~nxi)

REM Get the target file names and paths
SET mp3Name=%mp4Name:mp4=mp3%
SET mp3Key=s3://wordoflife.mn.audio/%mp3Name:~0,4%/%mp3Name%
SET mp3Url=https://s3.us-west-2.amazonaws.com/wordoflife.mn.audio/%mp3Name:~0,4%/%mp3Name: =+%

REM Get the trim length
SET trim=9.9
IF "%mp3Name: FF =%" NEQ "%mp3Name%" SET trim=9.8
IF "%mp3Name: CORE =%" NEQ "%mp3Name%" SET trim=30.0

ECHO "+-----------------------------------------------------------------------"
ECHO "| Extracting audio from video"
ECHO "+-----------------------------------------------------------------------"
ECHO Converting: %mp4Name%
ECHO         to: %mp3Name%
ECHO   trimming: %trim% seconds
ECHO  uploading: %mp3Key%
ECHO         as:
ECHO %mp3Url%
ECHO: 

REM skip if mp3 already exists
IF EXIST "%mp3Name%" GOTO upload-mp3-prompt

ffmpeg -hide_banner -loglevel warning -stats -i "%mp4Name%" -ss %trim% "%mp3Name%"

:upload-mp3-prompt
ECHO:
ECHO "+-----------------------------------------------------------------------"
ECHO "| Upload audio to S3 bucket s3://wordoflife.mn.audio
ECHO "+-----------------------------------------------------------------------"

REM Prompt for upload
REM -- skip this and always upload
IF true EQU true GOTO upload-mp3

ECHO   Upload: %mp3Name%
ECHO       to: %mp3Key%
SET /P doUpload="Confirm?: [Y/n/q] " || SET doUpload=y
IF /I "%doUpload%" EQU "q" GOTO pexit
IF /I "%doUpload%" NEQ "y" GOTO transcribe

:upload-mp3
aws s3 cp "%mp3Name%" "%mp3Key%"

:transcribe
ECHO:
ECHO "+-----------------------------------------------------------------------"
ECHO "| Transcribe and upload transcription
ECHO "+-----------------------------------------------------------------------"

REM / for files that will be used for *ix tools, \ for windows files
SET txtName=xscript/%mp3Name:.mp3=.txt%
SET vttName=xscript/%mp3Name:.mp3=.vtt%
SET srtName=xscript\%mp3Name:.mp3=.srt%
SET tsvName=xscript\%mp3Name:.mp3=.tsv%
SET jsonName=xscript\%mp3Name:.mp3=.json%

SET txtKey=s3://wordoflife.mn.audio/%mp3Name:~0,4%/%txtName%
SET txtUrl=https://s3.us-west-2.amazonaws.com/wordoflife.mn.audio/%mp3Name:~0,4%/%txtName: =+%
ECHO Transcribe: %mp3Name%
ECHO         to: txt (%txtName%)
ECHO         to: vtt (%vttName%)
ECHO      model: small
ECHO   language: english
ECHO     S3 URL: %txtKey%
ECHO  HTTPS URL: %txtUrl%
SET /P doTranscript="  Confirm: [Y/n/q]? " || SET doTranscript=y
IF /I "%doTranscript%" EQU "q" GOTO pexit
IF /I "%doTranscript%" NEQ "y" GOTO pexit

IF EXIST "%txtName%" GOTO upload-xscript
REM Generate transcript
REM - whisper '.\2024-03-17 Ministry is Work.mp3' -o . --model small -f all --language en
whisper "%mp3Name%" --output_dir ./xscript --fp16 False --model small --output_format all --language English
IF EXIST "%srtName%" (
  DEL "%srtName%" "%tsvName%" "%jsonName%"
)

:upload-xscript
ECHO:
aws s3 cp "%txtName%" "%txtKey%"
aws s3 cp "%vttName%" "%txtKey:.txt=.vtt%"

:pexit
pause
GOTO:eof

:dequote
REM The tilde in the next line is the really important bit.
SET retval=%~1
GOTO:eof
