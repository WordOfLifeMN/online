package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/WordOfLifeMN/online/ytpkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGetRefs(t *testing.T) {
	refs, err := parseGetRefs([]string{
		"01LiNfRzMo8",
		"https://www.youtube.com/watch?v=PksSjMFA6N0&list=PLmN8yVd0Kk4Ec6",
		"https://www.youtube.com/playlist?list=PLmN8yVd0Kk4Ec6",
		"https://rumble.com/v1abcde-some-message.html",
	}, false)

	require.Len(t, refs, 3)
	assert.Equal(t, "01LiNfRzMo8", refs[0].videoID)
	assert.Equal(t, "PksSjMFA6N0", refs[1].videoID, "a watch link means the video")
	assert.Empty(t, refs[1].playlistID)
	assert.Equal(t, "PLmN8yVd0Kk4Ec6", refs[2].playlistID)
	assert.ErrorContains(t, err, "rumble.com")
}

func TestParseGetRefs_AsPlaylist(t *testing.T) {
	refs, err := parseGetRefs([]string{
		"https://www.youtube.com/watch?v=PksSjMFA6N0&list=PLmN8yVd0Kk4Ec6",
		"01LiNfRzMo8",
	}, true)

	require.Len(t, refs, 1)
	assert.Equal(t, "PLmN8yVd0Kk4Ec6", refs[0].playlistID)
	assert.Empty(t, refs[0].videoID)
	assert.ErrorContains(t, err, "is not a YouTube playlist")
}

// a channel with no configured ID is skipped before any sign-in is attempted
func TestNewYouTubeClient_UnconfiguredChannel(t *testing.T) {
	_, err := newYouTubeClient(context.Background(), ytpkg.ChannelTBO)

	var notAuth *notAuthenticatedError
	require.True(t, errors.As(err, &notAuth))
	assert.Contains(t, err.Error(), "not authenticated for channel TBO")
}

func TestPrintLiveVideo_OtherChannel(t *testing.T) {
	var buf bytes.Buffer
	video := &ytpkg.Message{ID: "VXHmO7546ps", ChannelID: "UCother", Title: "T"}

	printLiveVideo(&buf, video, "UCZF_jDrfP6OltpviYmj-mkA", nil)

	out := buf.String()
	assert.True(t, strings.HasPrefix(out, "╭─ Video VXHmO7546ps "))
	assert.Contains(t, out, "├─ NOTE ")
	assert.Contains(t, out, "belongs to channel UCother")
	assert.True(t, strings.HasSuffix(out, "╯\n\n"))
}

func TestPrintLiveVideo_OwnChannel(t *testing.T) {
	var buf bytes.Buffer
	video := &ytpkg.Message{ID: "01LiNfRzMo8", ChannelID: "UCZF_jDrfP6OltpviYmj-mkA", Title: "T"}

	printLiveVideo(&buf, video, "UCZF_jDrfP6OltpviYmj-mkA", errors.New("quota"))

	out := buf.String()
	assert.NotContains(t, out, "NOTE")
	assert.Contains(t, out, "│ Could not look up playlists: quota ")
}
