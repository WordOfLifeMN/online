@ECHO OFF
REM ===================================================================================
REM Word of Life Ministries - prepare a message for publication to YouTube
REM
REM Prompts for the edited video file (drag it into the window), asks anything it
REM cannot work out from the file name, then extracts the audio, transcribes it,
REM generates a title and description, and prints the upload packet to paste into
REM YouTube Studio.
REM
REM This is only a launcher. All of the logic lives in the Go application - see
REM cmd/audio.go. Rebuild it with "go build -o online.exe" after changing the code.
REM ===================================================================================

TITLE WOLM - Prepare message for YouTube

SET ONLINE=%~dp0..\online.exe

IF NOT EXIST "%ONLINE%" (
  ECHO Cannot find %ONLINE%
  ECHO.
  ECHO Build it first:
  ECHO     cd %~dp0..
  ECHO     go build -o online.exe
  ECHO.
  PAUSE
  EXIT /b 1
)

"%ONLINE%" --verbose audio

ECHO.
PAUSE
