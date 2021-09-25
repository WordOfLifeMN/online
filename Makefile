
default: clean test build

## Cleans the intermediate and output files
clean:
	rm online

## Run the tests
test:
	go test ./...

## Build the executable
build:
	go build