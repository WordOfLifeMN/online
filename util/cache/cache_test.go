package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cached struct {
	Name string `json:"name"`
}

func TestCache_UsedInsideTTL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "cache.json")
	require.NoError(t, Write(path, cached{Name: "fresh"}))

	var got cached
	age, ok := Read(path, false, &got)
	assert.True(t, ok)
	assert.Equal(t, "fresh", got.Name)
	assert.Less(t, age, time.Minute)

	// nothing is left behind from the temporary write
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestCache_ExpiredAfterTTL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	require.NoError(t, Write(path, cached{Name: "stale"}))
	old := time.Now().Add(-TTL - time.Minute)
	require.NoError(t, os.Chtimes(path, old, old))

	var got cached
	_, ok := Read(path, false, &got)
	assert.False(t, ok)
	assert.Empty(t, got.Name)
}

func TestCache_RefreshForcesARead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	require.NoError(t, Write(path, cached{Name: "fresh"}))

	var got cached
	_, ok := Read(path, true, &got)
	assert.False(t, ok)
}

func TestCache_MissingOrCorruptIsAbsent(t *testing.T) {
	dir := t.TempDir()

	var got cached
	_, ok := Read(filepath.Join(dir, "missing.json"), false, &got)
	assert.False(t, ok)

	corrupt := filepath.Join(dir, "corrupt.json")
	require.NoError(t, os.WriteFile(corrupt, []byte("{not json"), 0o644))
	_, ok = Read(corrupt, false, &got)
	assert.False(t, ok)
}

// the cache rewrite replaces an existing file
func TestCache_Overwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	require.NoError(t, Write(path, cached{Name: "first"}))
	require.NoError(t, Write(path, cached{Name: "second"}))

	var got cached
	_, ok := Read(path, false, &got)
	assert.True(t, ok)
	assert.Equal(t, "second", got.Name)
}
