Application to manage the Word Of Life Ministries online catalog.

# Status
This is a GoLang application that reads information about series and messages
from Google Sheets, and generates output files that can be uploaded to the web
to serve this content online.

# Configuration

Reference: https://github.com/juampynr/google-spreadsheet-reader

Create a Service Account and download the credentials file. Copy the credentials
file to `~/.wolm/credentials.json`. Get the email address for this service
account and share the spreadsheet with it.

Configuration is first read from `online.yaml` in the current working directory.
If that is not found, then `~/.wolm/online.yaml` is tried. Parameters on the
command line override anything in the configuration file.

# Testing

Create sample test data in /tmp/online-catalog.json
```
make dryrun-dump
```

Generate a local copy of the website
```
make dryrun-catalog
open /tmp/t/catalog.*-az-*.html
```
