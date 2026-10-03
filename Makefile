
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

win-build:
	go build -o online.exe

win-dump: ## Downloads the spreadsheet to a local JSON file
	go run main.go -v --sheet-id=1z4XIiEPMFPpeRgGpdhshiQpmY7A45KzCyZzQ7Ohe85E dump

