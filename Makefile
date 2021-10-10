
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