// Package ytclient is this application's access to the YouTube Data API, as gclient is
// its access to the Google Sheet.
//
// Unlike the Sheet, YouTube is reached as a user, not as a service account: a service
// account cannot own or manage a channel. The first use of a channel runs the
// installed-app consent flow in the browser, and the resulting token is kept in the
// configuration directory, one file per channel.
package ytclient

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/WordOfLifeMN/online/ytpkg"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/youtube/v3"
)

// AuthConfig is what the consent flow needs: the OAuth client of the installed app, and
// where tokens are kept
type AuthConfig struct {
	ClientID     string
	ClientSecret string
	TokenDir     string
}

// redirectAddress is where the browser is sent back to after consent. It is the same
// address yt-api uses, so the one OAuth client serves both.
const redirectAddress = "localhost:8080"

// consentTimeout is how long to wait for the operator to finish in the browser
const consentTimeout = 5 * time.Minute

// TokenPath is the file holding a channel's token: youtube-token-wol.json and so on
func TokenPath(dir string, ch ytpkg.Channel) string {
	return filepath.Join(dir, fmt.Sprintf("youtube-token-%s.json", ch.Key()))
}

// HTTPClient returns an HTTP client authorised for a channel.
//
// It loads the channel's token, or runs the consent flow when there is none. The
// client refreshes the token as it expires and saves each refreshed token back to the
// file, so the consent flow is needed once per channel rather than once per run.
//
// The consent screen picks the account and channel. Which channel was actually chosen
// is checked by the caller, against the channel's expected ID.
func HTTPClient(ctx context.Context, cfg AuthConfig, ch ytpkg.Channel) (*http.Client, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("youtube-client-id and youtube-client-secret must be set in the configuration")
	}
	if ch.Key() == "" {
		return nil, fmt.Errorf("unknown YouTube channel %q", string(ch))
	}

	oauthConfig := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  "http://" + redirectAddress,
		Scopes:       []string{youtube.YoutubeScope},
	}

	path := TokenPath(cfg.TokenDir, ch)
	token, err := loadToken(path)
	if errors.Is(err, os.ErrNotExist) {
		token, err = runConsent(ctx, oauthConfig, ch, os.Stdout)
		if err != nil {
			return nil, err
		}
		if err := saveToken(path, token); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	source := &savingTokenSource{
		base: oauth2.ReuseTokenSource(token, oauthConfig.TokenSource(ctx, token)),
		path: path,
		last: token.AccessToken,
	}
	return oauth2.NewClient(ctx, source), nil
}

// loadToken reads a saved token
func loadToken(path string) (*oauth2.Token, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("cannot read the YouTube token in %s: %w. delete it to sign in again",
			path, err)
	}
	return &token, nil
}

// saveToken writes a token readable by its owner only. It goes to a temporary file
// first and is then renamed, so an interrupted write cannot leave a truncated token
// that forces a fresh consent.
func saveToken(path string, token *oauth2.Token) error {
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("cannot create %s: %w", filepath.Dir(path), err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("cannot save the YouTube token: %w", err)
	}
	defer os.Remove(tmp.Name()) // a no-op once renamed

	if err := tmp.Chmod(0o600); err != nil && runtime.GOOS != "windows" {
		tmp.Close()
		return fmt.Errorf("cannot protect the YouTube token: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("cannot save the YouTube token: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("cannot save the YouTube token: %w", err)
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("cannot save the YouTube token: %w", err)
	}
	return nil
}

// savingTokenSource saves each new token its base source hands out, so a refreshed
// access token survives the run that refreshed it
type savingTokenSource struct {
	base oauth2.TokenSource
	path string

	mu   sync.Mutex
	last string // the access token last saved
}

func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	token, err := s.base.Token()
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if token.AccessToken != s.last {
		// a failed save costs a refresh on the next run, not this request
		if err := saveToken(s.path, token); err == nil {
			s.last = token.AccessToken
		}
	}
	return token, nil
}

// runConsent sends the operator to Google's consent screen and waits on the loopback
// address for the redirect that carries the authorisation code.
//
// It uses PKCE and a random state, so a code intercepted on the way back is useless to
// anyone else, and a redirect that this run did not start is refused.
func runConsent(
	ctx context.Context,
	cfg *oauth2.Config,
	ch ytpkg.Channel,
	out io.Writer,
) (*oauth2.Token, error) {
	// listen before sending anyone to the browser, so a port in use fails here
	// rather than after the operator has clicked through the consent screen
	listener, err := net.Listen("tcp", redirectAddress)
	if err != nil {
		return nil, fmt.Errorf("cannot listen on %s for the YouTube sign-in (is yt-api signing in?): %w",
			redirectAddress, err)
	}
	defer listener.Close()

	state, err := randomState()
	if err != nil {
		return nil, err
	}
	verifier := oauth2.GenerateVerifier()

	authURL := cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		// forces a refresh token even when this account has consented before
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.S256ChallengeOption(verifier))

	fmt.Fprintf(out, "\nSign in to YouTube for the %s channel. Opening your browser at:\n\n%s\n\n",
		string(ch), authURL)
	fmt.Fprintf(out, "Choose the %s channel when asked. Waiting for the sign-in to finish...\n",
		string(ch))
	openBrowser(authURL)

	type result struct {
		code string
		err  error
	}
	results := make(chan result, 1)

	server := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			if query.Get("state") == "" && query.Get("code") == "" && query.Get("error") == "" {
				// the browser asking for a favicon, or similar
				http.NotFound(w, r)
				return
			}

			var res result
			switch {
			case query.Get("state") != state:
				res.err = fmt.Errorf("the YouTube sign-in returned an unexpected state; try again")
			case query.Get("error") != "":
				res.err = fmt.Errorf("the YouTube sign-in was not completed: %s", query.Get("error"))
			case query.Get("code") == "":
				res.err = fmt.Errorf("the YouTube sign-in returned no authorisation code")
			default:
				res.code = query.Get("code")
			}

			if res.err != nil {
				fmt.Fprintf(w, "Sign-in failed: %s\n", res.err)
			} else {
				fmt.Fprintf(w, "Signed in. You can close this window and return to the terminal.\n")
			}

			select {
			case results <- res:
			default:
			}
		}),
	}
	go server.Serve(listener)
	defer server.Close()

	ctx, cancel := context.WithTimeout(ctx, consentTimeout)
	defer cancel()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("gave up waiting for the YouTube sign-in after %s", consentTimeout)
	case res := <-results:
		if res.err != nil {
			return nil, res.err
		}
		token, err := cfg.Exchange(ctx, res.code, oauth2.VerifierOption(verifier))
		if err != nil {
			return nil, fmt.Errorf("cannot complete the YouTube sign-in: %w", err)
		}
		fmt.Fprintf(out, "Signed in to YouTube.\n")
		return token, nil
	}
}

// randomState returns an unguessable value to tie the redirect to this run
func randomState() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("cannot generate a sign-in state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// openBrowser tries to open a URL in the default browser. Failure is quiet: the URL has
// already been printed to be opened by hand.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
