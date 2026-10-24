package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/WordOfLifeMN/online/util"
	"github.com/WordOfLifeMN/online/ytclient"
	"github.com/WordOfLifeMN/online/ytpkg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// youtubeTokenDir is where the per-channel YouTube tokens are kept, beside the Sheets
// credentials
const youtubeTokenDir = "~/.wolm"

// defaultYouTubeChannelIDs are the channels this tool may act on, by Channel.Key().
// Only WOL is set up. A channel without an entry is reported as not authenticated and
// skipped, rather than sending the operator through a consent flow for a channel
// nobody has prepared.
var defaultYouTubeChannelIDs = map[string]string{
	ytpkg.ChannelWOL.Key(): "UCZF_jDrfP6OltpviYmj-mkA",
}

// youtubeCmd is the parent of the commands that work directly against YouTube
var youtubeCmd = &cobra.Command{
	Use:   "youtube",
	Short: "Read and update videos and playlists on YouTube",
	Long: `Work directly against YouTube.

The first use of a channel opens the browser to sign in to YouTube. Choose the
channel being worked on. The sign-in is kept in ~/.wolm/youtube-token-<channel>.json
and is not asked for again until it expires. Run 'online youtube login' to sign in
again then.

Requires youtube-client-id and youtube-client-secret in the configuration file.`,
}

func init() {
	rootCmd.AddCommand(youtubeCmd)

	viper.SetDefault("youtube-channel-ids", defaultYouTubeChannelIDs)

	youtubeCmd.PersistentFlags().Bool("refresh", false,
		"Re-read the spreadsheet instead of using the cached copy")
	viper.BindPFlag("refresh", youtubeCmd.PersistentFlags().Lookup("refresh"))

	youtubeCmd.PersistentFlags().Bool("yes", false,
		"Apply changes without asking, except destructive ones and playlist creation")
	viper.BindPFlag("yes", youtubeCmd.PersistentFlags().Lookup("yes"))
}

// notAuthenticatedError reports a channel this tool has not been set up for
type notAuthenticatedError struct {
	channel ytpkg.Channel
}

func (e *notAuthenticatedError) Error() string {
	return fmt.Sprintf("not authenticated for channel %s: add its ID under youtube-channel-ids "+
		"in the configuration to set it up", string(e.channel))
}

// newYouTubeClient connects to YouTube as a channel, signing in first if this is the
// channel's first use, and checks that the sign-in is for the channel expected.
//
// A wrong channel is an error rather than a warning: the consent screen lets the
// operator pick any channel the account manages, and a token for the wrong one would
// otherwise send every later write to it.
func newYouTubeClient(ctx context.Context, ch ytpkg.Channel) (*ytclient.Client, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	expected := viper.GetStringMapString("youtube-channel-ids")[ch.Key()]
	if ch.Key() == "" || expected == "" {
		return nil, &notAuthenticatedError{channel: ch}
	}

	cfg := ytclient.AuthConfig{
		ClientID:     viper.GetString("youtube-client-id"),
		ClientSecret: viper.GetString("youtube-client-secret"),
		TokenDir:     util.NormalizePath(youtubeTokenDir),
	}

	httpClient, err := ytclient.HTTPClient(ctx, cfg, ch)
	if err != nil {
		return nil, err
	}

	client, err := ytclient.New(ctx, httpClient)
	if err != nil {
		return nil, err
	}
	client.Progress = os.Stdout

	if client.ChannelID != expected {
		return nil, fmt.Errorf(
			"the YouTube sign-in for %s is for channel %s, but %s is %s. delete %s and "+
				"sign in again, choosing the %s channel",
			string(ch), client.ChannelID, string(ch), expected,
			ytclient.TokenPath(cfg.TokenDir, ch), string(ch))
	}

	return client, nil
}

// confirm asks a yes or no question, appending [Y/n] or [y/N] to show the default that
// Enter takes.
//
// honourYes lets --yes answer for the operator. It is false for the questions --yes
// must not skip: a destructive change, and creating a playlist. Closed or unreadable
// input answers no, whatever the default, so a run with nothing at the keyboard never
// changes anything by accident.
func confirm(prompt string, defaultYes bool, honourYes bool) bool {
	choices := "[y/N]"
	if defaultYes {
		choices = "[Y/n]"
	}

	if honourYes && viper.GetBool("yes") {
		fmt.Printf("%s %s y (--yes)\n", prompt, choices)
		return true
	}

	reader := getStdin()
	for {
		fmt.Printf("%s %s ", prompt, choices)
		answer, err := reader.ReadString('\n')
		if err != nil && strings.TrimSpace(answer) == "" {
			fmt.Println()
			return false
		}

		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "":
			return defaultYes
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
		fmt.Println("Please answer y or n.")
	}
}
