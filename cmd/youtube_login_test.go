package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/WordOfLifeMN/online/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tokenFile(t *testing.T, content string) string {
	path := filepath.Join(t.TempDir(), "youtube-token-wol.json")
	if content != "" {
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	return path
}

func readFile(t *testing.T, path string) string {
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

// the sign-in runs with no saved token, so it signs in afresh, and its token replaces
// the old one
func TestReplaceSignIn_Success(t *testing.T) {
	path := tokenFile(t, "old")

	err := replaceSignIn(path, func() error {
		assert.False(t, util.DoesPathExist(path), "the old token is out of the way")
		return os.WriteFile(path, []byte("new"), 0o600)
	})

	require.NoError(t, err)
	assert.Equal(t, "new", readFile(t, path))
	assert.False(t, util.DoesPathExist(path+".previous"), "the old token is discarded")
}

// a failed sign-in - cancelled, timed out, the wrong channel - puts the old token back
// and removes anything the attempt left
func TestReplaceSignIn_FailureRestores(t *testing.T) {
	path := tokenFile(t, "old")

	err := replaceSignIn(path, func() error {
		os.WriteFile(path, []byte("wrong channel"), 0o600)
		return errors.New("the YouTube sign-in is for another channel")
	})

	require.ErrorContains(t, err, "is for another channel")
	assert.ErrorContains(t, err, "the previous sign-in has been kept")
	assert.Equal(t, "old", readFile(t, path))
	assert.False(t, util.DoesPathExist(path+".previous"))
}

func TestReplaceSignIn_NoPreviousToken(t *testing.T) {
	path := tokenFile(t, "")

	require.NoError(t, replaceSignIn(path, func() error {
		return os.WriteFile(path, []byte("new"), 0o600)
	}))
	assert.Equal(t, "new", readFile(t, path))

	// and a failure with nothing to restore leaves nothing behind
	other := tokenFile(t, "")
	err := replaceSignIn(other, func() error {
		os.WriteFile(other, []byte("half"), 0o600)
		return errors.New("timed out")
	})
	assert.EqualError(t, err, "timed out")
	assert.False(t, util.DoesPathExist(other))
}
