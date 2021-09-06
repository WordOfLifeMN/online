package catalog

import (
	"encoding/json"
	"fmt"
	"os"
)

// code to read/write catalog information from/to disc

func NewCatalogFromJSON(jsonFile string) (*Catalog, error) {
	// read from the JSON file
	bytes, err := os.ReadFile(jsonFile)
	if err != nil {
		return nil, fmt.Errorf("Cannot read JSON file '%s': %w", jsonFile, err)
	}

	// unmarshal the file contents into a catalog
	var catalog Catalog
	err = json.Unmarshal(bytes, &catalog)
	return &catalog, err
}

func NewJSONFileFromCatalog(jsonFile string, catalog *Catalog) error {
	// marshal the catalog into JSON bytes
	bytes, err := json.Marshal(catalog)
	if err != nil {
		return err
	}

	// write the bytes to the flie
	err = os.WriteFile(jsonFile, bytes, 0644)
	return err
}
