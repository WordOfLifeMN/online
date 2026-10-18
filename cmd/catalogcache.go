package cmd

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/util/cache"
	"github.com/spf13/viper"
)

// catalogCacheName is the cached copy of the spreadsheet in the scratch directory, in
// the same format as 'online dump'
const catalogCacheName = "catalog-cache.json"

// catalogCachePath is where the cached spreadsheet is kept
func catalogCachePath() string {
	return filepath.Join(getScratchDir(), catalogCacheName)
}

// readCatalogCached reads the spreadsheet for the youtube commands, through a cache.
//
// A cached copy younger than an hour is used as is, and otherwise the sheet is read
// and the cache rewritten. --refresh forces a read. An explicit --input is read
// directly and never cached, since it is already a local file.
//
// The catalog is returned as read, not initialized, as 'online dump' writes it.
//
// 'audio' does not use this: its row has usually just been edited, so it reads the
// sheet fresh every time.
func readCatalogCached(ctx context.Context) (*catalog.Catalog, error) {
	return readCatalogCachedFrom(ctx, readOnlineContentFromInput)
}

// readCatalogCachedFrom is readCatalogCached with the source made replaceable, for tests
func readCatalogCachedFrom(
	ctx context.Context,
	read func(context.Context) (*catalog.Catalog, error),
) (*catalog.Catalog, error) {
	if viper.GetString("input") != "" {
		return read(ctx)
	}

	path := catalogCachePath()
	var cached catalog.Catalog
	if age, ok := cache.Read(path, viper.GetBool("refresh"), &cached); ok {
		log.Printf("Using the cached spreadsheet in %s, %s old (--refresh to re-read)",
			path, age.Round(time.Second))
		return &cached, nil
	}

	fmt.Printf("Reading the spreadsheet...\n")
	cat, err := read(ctx)
	if err != nil {
		return nil, err
	}
	if err := cache.Write(path, cat); err != nil {
		// the next run reads the sheet again, which is slow but not wrong
		log.Printf("WARNING: %s", err)
	}
	return cat, nil
}
