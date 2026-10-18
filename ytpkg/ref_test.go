package ytpkg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// every form found in the sheet's Video and Resources columns
func TestParseVideoRef(t *testing.T) {
	cases := map[string]string{
		"01LiNfRzMo8":                                                       "01LiNfRzMo8",
		"  01LiNfRzMo8  ":                                                   "01LiNfRzMo8",
		"https://youtu.be/01LiNfRzMo8":                                      "01LiNfRzMo8",
		"http://youtu.be/01LiNfRzMo8":                                       "01LiNfRzMo8",
		"https://youtu.be7HcFePGg7dI":                                       "7HcFePGg7dI",
		"https://youtu.bea4fdKX1SCq8":                                       "a4fdKX1SCq8",
		"https://youtu.be/kkncKxj83yA?si=0y6TKQl69T-u5ikY":                  "kkncKxj83yA",
		"https://www.youtube.com/watch?v=PksSjMFA6N0":                       "PksSjMFA6N0",
		"https://youtube.com/watch?v=PksSjMFA6N0":                           "PksSjMFA6N0",
		"https://www.youtube.com/watch?feature=x&v=PksSjMFA6N0":             "PksSjMFA6N0",
		"https://www.youtube.com/watch?v=Yx6UgfQreYY (IKEA: Bully A Plant)": "Yx6UgfQreYY",
		"https://www.youtube.com/live/8vuxo12Nt-c?feature=share":            "8vuxo12Nt-c",
		"https://youtube.com/live/3oXvpnL75Lg":                              "3oXvpnL75Lg",
		"https://www.youtube.com/shorts/VXHmO7546ps":                        "VXHmO7546ps",
		"https://www.youtube.com/embed/VXHmO7546ps":                         "VXHmO7546ps",

		// a second link pasted on without a space
		"https://youtu.be/o8YM_2vFTwMhttps://www.usatoday.com/story": "o8YM_2vFTwM",

		// the video wins over the playlist in a watch link
		"https://www.youtube.com/watch?v=PksSjMFA6N0&list=PLabcdefghijklmnop": "PksSjMFA6N0",
	}

	for ref, expected := range cases {
		id, ok := ParseVideoRef(ref)
		assert.True(t, ok, "ref %q", ref)
		assert.Equal(t, expected, id, "ref %q", ref)
	}
}

func TestParseVideoRef_Rejects(t *testing.T) {
	for _, ref := range []string{
		"",
		"-",
		"in progress",
		"https://rumble.com/v1abcde-some-message.html",
		"https://s3.us-west-2.amazonaws.com/wordoflife.mn.video/2020-01-05.mp4",
		"https://s3-us-west-2.amazonaws.com/wordoflife.mn.audio/2014-01-05.mp3",
		"https://www.youtube.com/playlist?list=PLabcdefghijklmnop",
		"01LiNfRzMo",   // ten characters
		"01LiNfRzMo8x", // twelve
	} {
		_, ok := ParseVideoRef(ref)
		assert.False(t, ok, "ref %q", ref)
	}
}

func TestParsePlaylistRef(t *testing.T) {
	cases := map[string]string{
		"PLmN8yVd0Kk4Ec6ab2c3d4e5f6g7h8i9j0":                                            "PLmN8yVd0Kk4Ec6ab2c3d4e5f6g7h8i9j0",
		"UUZF_jDrfP6OltpviYmj-mkA":                                                      "UUZF_jDrfP6OltpviYmj-mkA",
		"https://www.youtube.com/playlist?list=PLmN8yVd0Kk4Ec6ab2c3d4e5f6g7h8i9j0":      "PLmN8yVd0Kk4Ec6ab2c3d4e5f6g7h8i9j0",
		"https://youtube.com/playlist?list=PLmN8yVd0Kk4Ec6&si=abc":                      "PLmN8yVd0Kk4Ec6",
		"https://www.youtube.com/watch?v=PksSjMFA6N0&list=PLmN8yVd0Kk4Ec6ab2c3d4e5f6g7": "PLmN8yVd0Kk4Ec6ab2c3d4e5f6g7",
	}

	for ref, expected := range cases {
		id, ok := ParsePlaylistRef(ref)
		assert.True(t, ok, "ref %q", ref)
		assert.Equal(t, expected, id, "ref %q", ref)
	}
}

func TestParsePlaylistRef_Rejects(t *testing.T) {
	for _, ref := range []string{
		"",
		"01LiNfRzMo8",
		"https://youtu.be/01LiNfRzMo8",
		"https://www.youtube.com/watch?v=PksSjMFA6N0",
		"https://rumble.com/v1abcde-some-message.html?list=PLabcdefghijklmnop",
	} {
		_, ok := ParsePlaylistRef(ref)
		assert.False(t, ok, "ref %q", ref)
	}
}
