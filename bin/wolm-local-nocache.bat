ECHO off
ECHO "TEST: Word of Life Ministries local catalog generator (does not download new data)"

SET CACHE=%USERPROFILE%\.wolm\online.cache.json
SET CATALOG=%USERPROFILE%\.wolm\catalog

CD C:\Users\WordofLifeMNMedia\Go\github.com\WordOfLifeMN\online

ECHO:
ECHO Validating online content...
go run main.go --input %CACHE% check
IF %errorlevel% NEQ 0 (
    pause
    EXIT /b %errorlevel%
)

ECHO:
ECHO Generating and uploading online catalog...
go run main.go -v --input %CACHE% --output=%CATALOG% catalog

pause
