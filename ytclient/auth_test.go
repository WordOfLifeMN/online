package ytclient

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/WordOfLifeMN/online/ytpkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestTokenPath(t *testing.T) {
	dir := filepath.Join("home", ".wolm")
	assert.Equal(t, filepath.Join(dir, "youtube-token-wol.json"), TokenPath(dir, ytpkg.ChannelWOL))
	assert.Equal(t, filepath.Join(dir, "youtube-token-faith-freedom.json"),
		TokenPath(dir, ytpkg.ChannelFaithAndFreedom))
	assert.Equal(t, filepath.Join(dir, "youtube-token-tbo.json"), TokenPath(dir, ytpkg.ChannelTBO))
}

func TestToken_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "youtube-token-wol.json")
	expiry := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	token := &oauth2.Token{
		AccessToken:  "access",
		RefreshToken: "refresh",
		TokenType:    "Bearer",
		Expiry:       expiry,
	}

	require.NoError(t, saveToken(path, token))

	loaded, err := loadToken(path)
	require.NoError(t, err)
	assert.Equal(t, "access", loaded.AccessToken)
	assert.Equal(t, "refresh", loaded.RefreshToken)
	assert.Equal(t, "Bearer", loaded.TokenType)
	assert.True(t, expiry.Equal(loaded.Expiry))

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}

	// nothing is left behind from the temporary write
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestToken_Missing(t *testing.T) {
	_, err := loadToken(filepath.Join(t.TempDir(), "nope.json"))
	assert.True(t, errors.Is(err, os.ErrNotExist))
}

func TestToken_Corrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "youtube-token-wol.json")
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))

	_, err := loadToken(path)
	assert.ErrorContains(t, err, "delete it")
}

// fixedSource hands out whatever token it holds
type fixedSource struct{ token *oauth2.Token }

func (f *fixedSource) Token() (*oauth2.Token, error) { return f.token, nil }

// a refreshed token is written back; an unchanged one is not rewritten
func TestSavingTokenSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "youtube-token-wol.json")
	base := &fixedSource{token: &oauth2.Token{AccessToken: "first", RefreshToken: "r"}}
	source := &savingTokenSource{base: base, path: path, last: "first"}

	_, err := source.Token()
	require.NoError(t, err)
	_, err = os.Stat(path)
	assert.True(t, errors.Is(err, os.ErrNotExist), "an unchanged token is not rewritten")

	base.token = &oauth2.Token{AccessToken: "second", RefreshToken: "r"}
	_, err = source.Token()
	require.NoError(t, err)

	loaded, err := loadToken(path)
	require.NoError(t, err)
	assert.Equal(t, "second", loaded.AccessToken)
}

func TestHTTPClient_RequiresClientCredentials(t *testing.T) {
	_, err := HTTPClient(context.Background(), AuthConfig{TokenDir: t.TempDir()}, ytpkg.ChannelWOL)
	assert.ErrorContains(t, err, "youtube-client-id")
}

// with a saved token there is no consent flow, and so no listener or browser
func TestHTTPClient_UsesSavedToken(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, saveToken(TokenPath(dir, ytpkg.ChannelWOL), &oauth2.Token{
		AccessToken: "a", RefreshToken: "r", Expiry: time.Now().Add(time.Hour),
	}))

	client, err := HTTPClient(context.Background(),
		AuthConfig{ClientID: "id", ClientSecret: "secret", TokenDir: dir}, ytpkg.ChannelWOL)
	require.NoError(t, err)
	assert.NotNil(t, client)
}
