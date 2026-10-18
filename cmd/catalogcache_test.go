package cmd

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSheet stands in for the spreadsheet, counting how often it is read
type fakeSheet struct {
	name  string
	reads int
}

func (f *fakeSheet) read(context.Context) (*catalog.Catalog, error) {
	f.reads++
	return &catalog.Catalog{Messages: []catalog.CatalogMessage{{Name: f.name}}}, nil
}

// withCatalogConfig points the scratch directory at a temporary one and sets the
// cache flags for one test
func withCatalogConfig(t *testing.T, input string, refresh bool) {
	viper.Set("scratch-dir", t.TempDir())
	viper.Set("input", input)
	viper.Set("refresh", refresh)
	t.Cleanup(func() {
		viper.Set("scratch-dir", "")
		viper.Set("input", "")
		viper.Set("refresh", false)
	})
}

func TestCatalogCache_UsedInsideTTL(t *testing.T) {
	withCatalogConfig(t, "", false)
	sheet := &fakeSheet{name: "first"}

	cat, err := readCatalogCachedFrom(context.Background(), sheet.read)
	require.NoError(t, err)
	assert.Equal(t, "first", cat.Messages[0].Name)

	sheet.name = "second"
	cat, err = readCatalogCachedFrom(context.Background(), sheet.read)
	require.NoError(t, err)
	assert.Equal(t, "first", cat.Messages[0].Name, "served from the cache")
	assert.Equal(t, 1, sheet.reads)
}

func TestCatalogCache_RefreshedAfterTTL(t *testing.T) {
	withCatalogConfig(t, "", false)
	sheet := &fakeSheet{name: "first"}
	_, err := readCatalogCachedFrom(context.Background(), sheet.read)
	require.NoError(t, err)

	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(catalogCachePath(), old, old))

	sheet.name = "second"
	cat, err := readCatalogCachedFrom(context.Background(), sheet.read)
	require.NoError(t, err)
	assert.Equal(t, "second", cat.Messages[0].Name)
	assert.Equal(t, 2, sheet.reads)

	// and the cache was rewritten
	sheet.name = "third"
	cat, err = readCatalogCachedFrom(context.Background(), sheet.read)
	require.NoError(t, err)
	assert.Equal(t, "second", cat.Messages[0].Name)
}

func TestCatalogCache_RefreshForcesARead(t *testing.T) {
	withCatalogConfig(t, "", false)
	sheet := &fakeSheet{name: "first"}
	_, err := readCatalogCachedFrom(context.Background(), sheet.read)
	require.NoError(t, err)

	viper.Set("refresh", true)
	sheet.name = "second"
	cat, err := readCatalogCachedFrom(context.Background(), sheet.read)
	require.NoError(t, err)
	assert.Equal(t, "second", cat.Messages[0].Name)
	assert.Equal(t, 2, sheet.reads)
}

// an explicit --input is read directly, and neither uses nor writes the cache
func TestCatalogCache_InputBypasses(t *testing.T) {
	withCatalogConfig(t, "test.json", false)
	sheet := &fakeSheet{name: "from input"}

	for range 2 {
		cat, err := readCatalogCachedFrom(context.Background(), sheet.read)
		require.NoError(t, err)
		assert.Equal(t, "from input", cat.Messages[0].Name)
	}
	assert.Equal(t, 2, sheet.reads)

	_, err := os.Stat(catalogCachePath())
	assert.True(t, os.IsNotExist(err))
}
