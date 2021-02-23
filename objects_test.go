package jsonextract

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestObjects(t *testing.T) {
	tests := []struct {
		json     string
		expected map[string]int
	}{
		{
			`{ key1: "asdf", key2: "ghijk"}`,
			map[string]int{
				`{"key1":"asdf","key2":"ghijk"}`: 0,
			},
		},
		{
			`{ "unrelated": { key1: "asdf", key2: "ghijk"}}`,
			map[string]int{
				`{"key1":"asdf","key2":"ghijk"}`: 0,
			},
		},
		{
			`{ "unrelated": [{ key1: "asdf", key2: "ghijk"}, { key1: "asdf", key3: "ghijk"}]}`,
			map[string]int{
				`{"key1":"asdf","key2":"ghijk"}`: 0,
				`{"key1":"asdf","key3":"ghijk"}`: 1,
			},
		},
		{
			`{
			 "slideshow": {
			   "author": "Yours Truly",
			   "date": "date of publication",
			   "slides": [
			     {
			       "key1": "Wake up to WonderWidgets!",
			       "key2": "all"
			     },
			     {
			       "key3": [
			         "Why <em>WonderWidgets</em> are great",
			         "Who <em>buys</em> WonderWidgets"
			       ],
			       "key1": "Overview",
			       "key2": "all"
			     }
			   ],
			    "title": "Sample Slide Show"
			  }
			}`,
			map[string]int{
				`{"key1":"Wake up to WonderWidgets!","key2":"all"}`:                                                                  0,
				`{"key3":["Why <em>WonderWidgets</em> are great","Who <em>buys</em> WonderWidgets"],"key1":"Overview","key2":"all"}`: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(t.Name(), func(t *testing.T) {
			var calls int

			var cbFunc = func(i int) func(b []byte) error {
				return func(b []byte) error {
					ec, ok := tt.expected[string(b)]
					if !ok {
						t.Errorf("Unexpected callback value %q", string(b))
					}

					if ec != i {
						t.Errorf("Called wrong callback %d for %q, wanted callback %d", i, string(b), ec)
					}

					calls++

					return nil
				}
			}

			var options = []ObjectOption{
				{
					Keys:     []string{"key1", "key2"},
					Callback: cbFunc(0),
				},
				{
					Keys:     []string{"key3"},
					Callback: cbFunc(1),
				},
			}

			if err := Objects(strings.NewReader(tt.json), options); err != nil {
				t.Errorf("Unexpected Objects() error: %v", err)
			}

			if calls != len(tt.expected) {
				t.Errorf("Called callbacks %d times, but wanted %d", calls, len(tt.expected))
			}
		})
	}
}

func TestObjectsJSONFile(t *testing.T) {
	f, err := os.Open("testdata/repo.json")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	type License struct {
		Key    string `json:"key"`
		Name   string `json:"name"`
		SpdxID string `json:"spdx_id"`
		URL    string `json:"url"`
		NodeID string `json:"node_id"`
	}

	type Owner struct {
		Login      string `json:"login"`
		ID         int    `json:"id"`
		NodeID     string `json:"node_id"`
		AvatarURL  string `json:"avatar_url"`
		GravatarID string `json:"gravatar_id"`
		HTMLURL    string `json:"html_url"`
		Type       string `json:"type"`
		SiteAdmin  bool   `json:"site_admin"`
	}

	var (
		license License
		owner   Owner
	)

	var (
		calledLicense, calledOwner bool
	)

	err = Objects(f, []ObjectOption{
		{
			// License object
			Keys: []string{"key", "name", "spdx_id"},
			Callback: Unmarshal(&license, func() bool {
				calledLicense = true

				// Return true if the struct has all required fields
				return license.Key != "" && license.Name != "" && license.SpdxID != ""
			}),
		},
		{
			Keys: []string{"login", "id", "html_url"},
			Callback: Unmarshal(&owner, func() bool {
				calledOwner = true

				return owner.Login != "" && owner.HTMLURL != ""
			}),
		},
	})
	if err != nil {
		panic(err)
	}

	if !calledLicense {
		t.Errorf("Expected License callback to be called, but wasn't")
	}

	if !calledOwner {
		t.Errorf("Expected Owner callback to be called, but wasn't")
	}
}

func TestObjectsHTMLPlaylist(t *testing.T) {

	// Define all structs we need for extraction
	type ytVideo struct {
		VideoID   string `json:"videoId"`
		Thumbnail struct {
			Thumbnails []struct {
				URL    string `json:"url"`
				Width  int    `json:"width"`
				Height int    `json:"height"`
			} `json:"thumbnails"`
		} `json:"thumbnail"`
		Title struct {
			Runs []struct {
				Text string `json:"text"`
			} `json:"runs"`
		} `json:"title"`
		Index struct {
			SimpleText string `json:"simpleText"`
		} `json:"index"`
		LengthSeconds  string `json:"lengthSeconds"`
		TrackingParams string `json:"trackingParams"`
		IsPlayable     bool   `json:"isPlayable"`
	}

	type ytPlaylist struct {
		URLCanonical string `json:"urlCanonical"`
		Title        string `json:"title"`
		Description  string `json:"description"`
		Thumbnail    struct {
			Thumbnails []struct {
				URL    string `json:"url"`
				Width  int    `json:"width"`
				Height int    `json:"height"`
			} `json:"thumbnails"`
		} `json:"thumbnail"`
		SiteName string `json:"siteName"`
	}

	// This is where our data should end up
	var (
		videoList []ytVideo
		playlist  ytPlaylist
	)

	f, err := os.Open("testdata/playlist.html")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	err = Objects(f, []ObjectOption{
		{
			// For extracting video info
			Keys: []string{"videoId", "title"},
			Callback: func(b []byte) error {
				var vid ytVideo

				err := json.Unmarshal(b, &vid)
				if err != nil {
					return nil
				}

				// Check if anything required is missing
				if len(vid.Title.Runs) == 0 || vid.VideoID == "" {
					return nil
				}

				// Seems like we got the info we wanted
				videoList = append(videoList, vid)

				return nil
			},
		},
		{
			Keys: []string{"title", "urlCanonical"},
			Callback: Unmarshal(&playlist, func() bool {
				return playlist.Title != "" && playlist.URLCanonical != ""
			}),
		},
	})
	if err != nil {
		panic(err)
	}

	// Playlist has 10 entries
	if len(videoList) != 10 {
		t.Errorf("Expected %d videos, but got %d", 10, len(videoList))
	}

	if playlist.Title == "" || playlist.URLCanonical == "" {
		t.Errorf("Expected extraction of playlist data, but no data was extracted")
	}
}
