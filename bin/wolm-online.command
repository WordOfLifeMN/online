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
BUCKET_NAME=wordoflife.mn.podcast
PODCAST_NAME=wolmn-service-podcast.rss.xml
echo ""
echo "Generating podcast ..."
online --input $CACHE podcast --days=180 >$PODCAST_NAME
echo "    Uploading podcast ..."
aws --profile=wolm s3 cp --acl=public-read $PODCAST_NAME s3://$BUCKET_NAME/$PODCAST_NAME

# create and upload the catolog
BUCKET_NAME=wordoflife.mn.catalog
echo ""
echo "Generating catalog ..."
online --input $CACHE catalog -o catalog

# NOTE: at one point, I tried to use the --size-only to minimize the number of 
#       files uploaded, but then discovered that the catalog only generates files
#       signed for 7 days, so we need to run this script every week, and upload
#       all files to ensure that they are signed for the next week.
#       Note that the signature of the URLs is required so that we can set the
#       Content-Disposition to 'attachment' (even though the file is public).
# UPDATE: Since then, I have removed the download link, so the signatures are no
#         longer required.
echo "Syncing the files to S3 ..."
#aws --profile=wolm s3 sync --size-only --acl=public-read catalog/ s3://$BUCKET_NAME/
aws --profile=wolm s3 sync --size-only --delete --acl=public-read catalog/ s3://$BUCKET_NAME/
