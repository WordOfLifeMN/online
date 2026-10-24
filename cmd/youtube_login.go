package cmd

import (
	"fmt"
	"os"

	"github.com/WordOfLifeMN/online/util"
	"github.com/WordOfLifeMN/online/ytclient"
	"github.com/WordOfLifeMN/online/ytpkg"
	"github.com/spf13/cobra"
)

// youtubeLoginCmd signs in to YouTube afresh
var youtubeLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in to YouTube again, replacing the saved sign-in",
	Long: `Sign in to YouTube again for the WOL channel, replacing the saved sign-in in
~/.wolm/youtube-token-wol.json.

Use it when a youtube command fails because the sign-in has expired - while the
app's consent screen is in Testing, that happens every 7 days - or to switch the
sign-in to another account.

The browser opens to sign in. Choose the WOL channel when asked. The previous
sign-in is only discarded once the new one has worked and is for the right channel;
if the sign-in is cancelled, times out, or picks another channel, the previous one
is put back.`,
	Args: cobra.NoArgs,
	RunE: youtubeLogin,
}

func init() {
	youtubeCmd.AddCommand(youtubeLoginCmd)
}

func youtubeLogin(cmd *cobra.Command, args []string) error {
	initLogging()

	channel := ytpkg.ChannelWOL
	path := ytclient.TokenPath(util.NormalizePath(youtubeTokenDir), channel)

	var channelID string
	err := replaceSignIn(path, func() error {
		// with no saved sign-in, making a client runs the browser sign-in, saves the
		// token, and checks that it is for the channel expected
		client, err := newYouTubeClient(cmd.Context(), channel)
		if err != nil {
			return err
		}
		channelID = client.ChannelID
		return nil
	})
	if err != nil {
		return err
	}

	fmt.Printf("\nSigned in to YouTube for the %s channel (%s).\n", string(channel), channelID)
	fmt.Printf("Saved to %s\n", path)
	return nil
}

// replaceSignIn runs signIn with no saved sign-in at path, so that it signs in afresh,
// and keeps the previous sign-in safe until it has worked.
//
// The previous token is set aside rather than deleted. If signIn fails - the browser
// sign-in is cancelled or times out, or the new token is for the wrong channel - any
// token it left is removed and the previous one put back, so a failed attempt never
// leaves things worse than they were. On success the previous token is discarded.
func replaceSignIn(path string, signIn func() error) error {
	previous := path + ".previous"
	hadToken := util.IsFile(path)

	if hadToken {
		os.Remove(previous)
		if err := os.Rename(path, previous); err != nil {
			return fmt.Errorf("cannot set the current sign-in aside: %w", err)
		}
	}

	if err := signIn(); err != nil {
		// a token for the wrong channel, or from a half-finished sign-in, must not
		// stay to be used by the next command
		os.Remove(path)

		if hadToken {
			if restoreErr := os.Rename(previous, path); restoreErr != nil {
				return fmt.Errorf("%w\ncould not put the previous sign-in back from %s: %s",
					err, previous, restoreErr)
			}
			return fmt.Errorf("%w\nthe previous sign-in has been kept", err)
		}
		return err
	}

	if hadToken {
		os.Remove(previous)
	}
	return nil
}
