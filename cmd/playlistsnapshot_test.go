package cmd

import (
	"path/filepath"
	"testing"

	"github.com/WordOfLifeMN/online/util/cache"
	"github.com/WordOfLifeMN/online/ytclient"
	"github.com/WordOfLifeMN/online/ytpkg"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlaylistSnapshotPath(t *testing.T) {
	viper.Set("scratch-dir", t.TempDir())
	t.Cleanup(func() { viper.Set("scratch-dir", "") })

	path := playlistSnapshotPath(ytpkg.ChannelWOL)
	assert.Equal(t, "youtube-playlists-wol.json", filepath.Base(path))
	assert.Equal(t, getScratchDir(), filepath.Dir(path))

	// the snapshot is a cache like any other, and survives the round trip
	snapshot := ytclient.PlaylistSnapshot{
		ChannelID: "UCZF_jDrfP6OltpviYmj-mkA",
		Playlists: []*ytpkg.Series{{ID: "PL1", Title: "Prayer",
			Items: []ytpkg.PlaylistItem{{VideoID: "01LiNfRzMo8", Position: 2}}}},
	}
	require.NoError(t, cache.Write(path, snapshot))

	var got ytclient.PlaylistSnapshot
	_, ok := cache.Read(path, false, &got)
	require.True(t, ok)
	assert.Equal(t, snapshot, got)
}

// the scratch sweep deletes only intermediates, so the caches survive it
func TestCache_SurvivesScratchSweep(t *testing.T) {
	for _, name := range []string{"youtube-playlists-wol.json", "catalog-cache.json"} {
		assert.NotContains(t, scratchExtensions, filepath.Ext(name))
	}
}
