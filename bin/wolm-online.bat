ECHO off
ECHO "Word of Life Ministries catalog generator"

SET CACHE=%USERPROFILE%\.wolm\online.cache.json
SET CATALOG=%USERPROFILE%\.wolm\catalog

ECHO:
ECHO Getting online content...
C:\Users\WordofLifeMNMedia\Go\github.com\WordOfLifeMN\online\online.exe --output %CACHE% dump

ECHO:
ECHO Validating online content...
C:\Users\WordofLifeMNMedia\Go\github.com\WordOfLifeMN\online\online.exe --input %CACHE% check
IF %errorlevel% NEQ 0 (
    pause
    EXIT /b %errorlevel%
)

ECHO:
ECHO Generating and uploading online catalog...
C:\Users\WordofLifeMNMedia\Go\github.com\WordOfLifeMN\online\online.exe -v --input %CACHE% --output=%CATALOG% catalog
aws s3 sync --size-only --delete --acl=public-read %CATALOG% s3://wordoflife.mn.catalog

pause
