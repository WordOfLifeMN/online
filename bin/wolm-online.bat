ECHO off
ECHO "Word of Life Ministries podcast and catalog generator"

SET CACHE=%USERPROFILE%\.wolm\online.cache.json
SET PODCAST=%USERPROFILE%\.wolm\online.podcast.rss.xml
SET CATALOG=%USERPROFILE%\.wolm\catalog

ECHO:
ECHO Getting online content...
C:\Users\WordofLifeMNMedia\Go\github.com\WordOfLifeMN\online\online.exe -v --output %CACHE% dump

ECHO:
ECHO Validating online content...
C:\Users\WordofLifeMNMedia\Go\github.com\WordOfLifeMN\online\online.exe -v --input %CACHE% check

ECHO:
ECHO Generating and uploading podcast...
C:\Users\WordofLifeMNMedia\Go\github.com\WordOfLifeMN\online\online.exe -v --input %CACHE% --output %PODCAST% podcast
aws s3 cp --acl=public-read %USERPROFILE%\.wolm\online.podcast.rss.xml s3://wordoflife.mn.podcast/wolmn-service-podcast.rss.xml

ECHO:
ECHO Generating and uploading online catalog...
C:\Users\WordofLifeMNMedia\Go\github.com\WordOfLifeMN\online\online.exe -v --input %CACHE% --output=%CATALOG% catalog
aws s3 sync --dryrun --size-only --delete --acl=public-read %CATALOG% s3://wordoflife.mn.catalog
pause
