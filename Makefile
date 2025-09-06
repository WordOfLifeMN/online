
default: clean test build

## Cleans the intermediate and output files
clean:
	rm -f online
	go clean -testcache

## Run the tests
test:
	go test ./...

## Build the executable
build:
	go build

dryrun-test:
	go run main.go -i testdata/small-catalog.json catalog -o /tmp/t -v

dryrun: dryrun-dump dryrun-catalog

dryrun-dump:
	go run main.go -v --sheet-id=1z4XIiEPMFPpeRgGpdhshiQpmY7A45KzCyZzQ7Ohe85E dump >/tmp/online-catalog.json

dryrun-catalog:
	go run main.go -v -i /tmp/online-catalog.json catalog -o /tmp/t

run-refresh:
	# Add this to catalog.pre-content.html: <meta http-equiv="refresh" content="5" />
	{ find templates -type f && find . -name '*.go'; } | entr make dryrun-catalog

win-build:
	go build -o online.exe

win-dump: ## Downloads the spreadsheet to a local JSON file
	go run main.go -v --sheet-id=1z4XIiEPMFPpeRgGpdhshiQpmY7A45KzCyZzQ7Ohe85E dump

win-local: ## Processes the cache created by win-dump and creates a local website in ~/.wolm/catalog
	go run main.go -v -i  C:\Users\WordofLifeMNMedia\.wolm\online.cache.json catalog -o  C:\Users\WordofLifeMNMedia\.wolm\catalog

win-test-local: ## Processes the testdata/small-catalog.json and creates a local website in ~/.wolm/catalog-test
	go run main.go -v -i  C:\Users\WordofLifeMNMedia\Go\github.com\WordOfLifeMN\online\testdata\small-catalog.json catalog -o  C:\Users\WordofLifeMNMedia\.wolm\catalog-test
