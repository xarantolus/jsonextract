package jsonextract

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// This example shows how to extract nested objects.
func ExampleObjects_nestedObjects() {
	// Test input
	var input = strings.NewReader(`
	<script>
	var x = {
		"id": 339750489,
		// This comment makes the input invalid JSON
		"node_id": "MDEwOlJlcG9zaXRvcnkzMzk3NTA0ODk=",
		"name": "jsonextract",
		"full_name": "xarantolus/jsonextract",
		"private": false,
		"owner": {
			"login": "xarantolus",
			"id": 32465636,
			"node_id": "MDQ6VXNlcjMyNDY1NjM2",
			"avatar_url": "https://avatars.githubusercontent.com/u/32465636?v=4",
			"gravatar_id": "",
			"html_url": "https://github.com/xarantolus",
			"type": "User",
			"site_admin": false
		},
		"html_url": "https://github.com/xarantolus/jsonextract",
		"description": "Go package for finding and extracting any valid JavaScript object (not just JSON) from an io.Reader",
		"open_issues_count": 0,
		"license": {
			"key": "mit",
			"name": "MIT License",
			"spdx_id": "MIT",
			"url": "https://api.github.com/licenses/mit",
			"node_id": "MDc6TGljZW5zZTEz"
		},
	}
	</script>`)

	// The "license" object has this structure
	type License struct {
		Key    string `json:"key"`
		Name   string `json:"name"`
		SpdxID string `json:"spdx_id"`
		URL    string `json:"url"`
		NodeID string `json:"node_id"`
	}

	// ... and the "owner" object has this one
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

	// We want to extract these two different objects that are nested within
	// the whole JSON-like structure
	var (
		license License
		owner   Owner
	)

	// Use Objects to extract all objects and match them to their keys
	err := Objects(input, []ObjectOption{
		{
			// A valid license object has these keys
			Keys: []string{"key", "name", "spdx_id", "node_id"},
			// Unmarshal decodes the object to license until the function verifies that correct data was found
			// If there were multiple objects matching the keys, one could select the one that is wanted
			Callback: Unmarshal(&license, func() bool {
				// Return true if all fields we want have valid values
				return license.Key != "" && license.Name != ""
			}),
			// If this value is not present in the JSON data, the Objects call will return an error
			Required: true,
		},
		{
			// The owner object mostly has different keys, the overlap with "node_id"
			// doesn't matter because all listed keys must be present anyways
			Keys: []string{"login", "id", "html_url", "node_id"},
			Callback: Unmarshal(&owner, func() bool {
				return owner.Login != "" && owner.HTMLURL != ""
			}),
			Required: true,
		},
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s has published their package under the %s\n", owner.Login, license.Name)
	// Output: xarantolus has published their package under the MIT License
}

// This example shows how to extract both a single object and a list of other objects.
func ExampleObjects_multipleList() {
	// Define all structs we need for extraction
	type ytVideo struct {
		VideoID string `json:"videoId"`
		Title   struct {
			Runs []struct {
				Text string `json:"text"`
			} `json:"runs"`
		} `json:"title"`
	}

	type ytPlaylist struct {
		URLCanonical string `json:"urlCanonical"`
		Title        string `json:"title"`
	}

	// This is where our data should end up
	var (
		videoList []ytVideo
		playlist  ytPlaylist
	)

	// This file contains the HTML response of a YouTube playlist.
	// One could also extract directly from a response body
	f, err := os.Open("testdata/playlist.html")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	err = Objects(f, []ObjectOption{
		{
			// All videos have an "videoId" and "title" key
			Keys: []string{"videoId", "title"},
			// We use a more specialized callback to append to videoList
			Callback: func(b []byte) error {
				var vid ytVideo

				// Decode the given object. It has at least the Keys defined above
				err := json.Unmarshal(b, &vid)
				if err != nil {
					// if that didn't work, we skip the object
					return nil
				}

				// Check if anything required is missing
				if len(vid.Title.Runs) == 0 || vid.VideoID == "" {
					return nil
				}

				// Seems like we got the info we wanted, we can now store it
				videoList = append(videoList, vid)

				// ... and continue with the next object
				return nil
			},
		},
		{
			// Here we want to extract a playlist info object
			Keys: []string{"title", "urlCanonical"},
			Callback: Unmarshal(&playlist, func() bool {
				return playlist.Title != "" && playlist.URLCanonical != ""
			}),
		},
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("The %q playlist has %d videos\n", playlist.Title, len(videoList))
	// Output: The "Starship" playlist has 10 videos
}
