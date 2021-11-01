
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
	