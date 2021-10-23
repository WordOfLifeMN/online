
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

dryrun:
	go run main.go -i testdata/small-catalog.json catalog --view=public -o /tmp/t -v
	