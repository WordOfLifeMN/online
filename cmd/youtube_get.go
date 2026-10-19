package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/WordOfLifeMN/online/util/box"
	"github.com/WordOfLifeMN/online/util/cache"
	"github.com/WordOfLifeMN/online/ytclient"
	"github.com/WordOfLifeMN/online/ytpkg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var youtubeGetCmd = &cobra.Command{
	Use:   "get <video or playlist>...",
	Short: "Show videos and playlists as they are on YouTube",
	Long: `Show one or more videos or playlists as they are on YouTube.

Each reference can be a video ID, a playlist ID, or any YouTube link to either -
youtu.be, watch?v=, live/, shorts/ or playlist?list=. A watch link that also names a
playlist means the video, unless --playlist is given.

A video is shown with the playlists it belongs to. Finding them reads every playlist
on the channel, which costs a little quota on the first video of a run.`,
	Example: `  online youtube get 01LiNfRzMo8
  online youtube get --json https://www.youtube.com/playlist?list=PL...`,
	Args: cobra.MinimumNArgs(1),
	RunE: youtubeGet,
}

func init() {
	youtubeCmd.AddCommand(youtubeGetCmd)

	youtubeGetCmd.Flags().Bool("playlist", false, "Treat every reference as a playlist")
	youtubeGetCmd.Flags().Bool("json", false, "Print the packages as JSON instead of boxes")
}

// getRef is one command-line reference, resolved to what it names
type getRef struct {
	arg        string
	videoID    string
	playlistID string
}

// parseGetRefs works out what each reference names. With asPlaylist, every reference
// must be a playlist; otherwise a reference is a video if it can be, and a playlist if
// not.
func parseGetRefs(args []string, asPlaylist bool) ([]getRef, error) {
	var refs []getRef
	var errs []error

	for _, arg := range args {
		ref := getRef{arg: arg}
		if !asPlaylist {
			ref.videoID, _ = ytpkg.ParseVideoRef(arg)
		}
		if ref.videoID == "" {
			ref.playlistID, _ = ytpkg.ParsePlaylistRef(arg)
		}

		if ref.videoID == "" && ref.playlistID == "" {
			what := "a YouTube video or playlist"
			if asPlaylist {
				what = "a YouTube playlist"
			}
			errs = append(errs, fmt.Errorf("%q is not %s", arg, what))
			continue
		}
		refs = append(refs, ref)
	}

	return refs, errors.Join(errs...)
}

// getResult is the JSON output of 'youtube get'
type getResult struct {
	Videos    []*ytpkg.Message `json:"videos,omitempty"`
	Playlists []*ytpkg.Series  `json:"playlists,omitempty"`
}

func youtubeGet(cmd *cobra.Command, args []string) error {
	initLogging()

	asPlaylist, _ := cmd.Flags().GetBool("playlist")
	asJSON, _ := cmd.Flags().GetBool("json")

	// a bad reference is reported, but does not stop the good ones being shown
	refs, refErr := parseGetRefs(args, asPlaylist)
	if len(refs) == 0 {
		return refErr
	}

	// reading works from any channel's sign-in, including videos on other channels.
	// WOL is the only channel set up, so it is the one used.
	channel := ytpkg.ChannelWOL
	client, err := newYouTubeClient(cmd.Context(), channel)
	if err != nil {
		return err
	}

	var videoIDs []string
	for _, ref := range refs {
		if ref.videoID != "" {
			videoIDs = append(videoIDs, ref.videoID)
		}
	}
	videos := map[string]*ytpkg.Message{}
	if len(videoIDs) > 0 {
		found, err := client.GetVideos(videoIDs...)
		if err != nil {
			return err
		}
		for _, v := range found {
			videos[v.ID] = v
		}
	}

	// the playlists a video is in come from one scan of the channel, shared by every
	// video and cached between runs. they are a courtesy, so failing to find them
	// does not lose the videos.
	var playlistErr error
	if len(videos) > 0 {
		playlistErr = loadPlaylistSnapshot(client, channel)
	}

	errs := []error{refErr}
	var result getResult
	for _, ref := range refs {
		if ref.videoID != "" {
			video, ok := videos[ref.videoID]
			if !ok {
				errs = append(errs, fmt.Errorf("video %s was not found, or is not visible to this account",
					ref.videoID))
				continue
			}

			if playlistErr == nil {
				video.Playlists, err = client.PlaylistsContaining(video.ID)
				if err != nil {
					playlistErr = err
				}
			}
			if asJSON {
				result.Videos = append(result.Videos, video)
			} else {
				printLiveVideo(os.Stdout, video, client.ChannelID, playlistErr)
			}
			continue
		}

		series, err := client.GetPlaylist(ref.playlistID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if asJSON {
			result.Playlists = append(result.Playlists, series)
		} else {
			printLivePlaylist(os.Stdout, series, client.ChannelID)
		}
	}

	if asJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	}

	return errors.Join(errs...)
}

// playlistSnapshotPath is the cache of a channel's playlists and their items
func playlistSnapshotPath(ch ytpkg.Channel) string {
	return filepath.Join(getScratchDir(), fmt.Sprintf("youtube-playlists-%s.json", ch.Key()))
}

// loadPlaylistSnapshot gives the client every playlist on the channel with its items,
// from the cache when it is fresh, otherwise by scanning the channel and saving the
// result for the next run.
//
// The snapshot is only for showing. It can be an hour old, so nothing that writes to
// YouTube may decide anything from it.
func loadPlaylistSnapshot(client *ytclient.Client, ch ytpkg.Channel) error {
	path := playlistSnapshotPath(ch)

	var snapshot ytclient.PlaylistSnapshot
	if age, ok := cache.Read(path, viper.GetBool("refresh"), &snapshot); ok {
		// a cache from another channel's sign-in is refused, and replaced below
		err := client.UseSnapshot(&snapshot)
		if err == nil {
			log.Printf("Using the cached playlists in %s, %s old (--refresh to re-read)",
				path, age.Round(time.Second))
			return nil
		}
		log.Printf("Not using the cache %s: %s", path, err)
	}

	fresh, err := client.SnapshotPlaylists()
	if err != nil {
		return err
	}
	if err := cache.Write(path, fresh); err != nil {
		// the next run scans again, which is slow but not wrong
		log.Printf("WARNING: %s", err)
	}
	return nil
}

// printLiveVideo writes one video as a closed box, followed by a blank line
func printLiveVideo(w io.Writer, video *ytpkg.Message, channelID string, playlistErr error) {
	b := box.New(w)
	b.Header("Video " + video.ID)
	video.PrintLive(b)
	if playlistErr != nil {
		b.WarningSection("WARNING")
		b.WarningLabeled("Could not look up playlists: ", playlistErr.Error())
	}
	printOtherChannel(b, video.ChannelID, channelID)
	b.Footer()
	fmt.Fprintln(w)
}

// printLivePlaylist writes one playlist as a closed box, followed by a blank line
func printLivePlaylist(w io.Writer, series *ytpkg.Series, channelID string) {
	b := box.New(w)
	b.Header("Playlist " + series.ID)
	series.Print(b)
	printOtherChannel(b, series.ChannelID, channelID)
	b.Footer()
	fmt.Fprintln(w)
}

// printOtherChannel warns when an item lives on a channel other than the signed-in one,
// which can read it but never change it
func printOtherChannel(b *box.Box, owner string, signedIn string) {
	if owner == "" || owner == signedIn {
		return
	}
	b.Section("NOTE")
	b.Line(fmt.Sprintf("This belongs to channel %s, not the signed-in channel %s. "+
		"It can be read from here but not changed.", owner, signedIn))
}
