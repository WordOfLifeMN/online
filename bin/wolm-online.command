#!/bin/bash

cd ~/.wolm
which online >/dev/null 2>/dev/null || {
    PATH=$PATH:/Users/kmurray/git/go/src/github.com/WordOfLifeMN/online
}

CACHE=online.cache.json

# get a local copy of the online content
echo "Getting online content ..."
online dump >$CACHE || exit 1

# validate the catalog
echo ""
echo "Validating content ..."
online --input $CACHE check || exit 1

# create and upload the podcast
PODCAST_NAME=wolmn-service-podcast.rss.xml
echo ""
echo "Generating podcast ..."
online --input $CACHE podcast --days=180 >$PODCAST_NAME
echo "    Uploading podcast ..."
aws s3 cp --acl=public-read $PODCAST_NAME s3://wordoflife.mn.podcast/$PODCAST_NAME

