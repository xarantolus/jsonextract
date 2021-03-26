package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/xarantolus/jsonextract"
)

func main() {
	vid, err := youTubeLive("https://www.youtube.com/channel/UCSUu1lih2RifWkKtDOJdsBA/live")
	if err != nil {
		log.Fatalln(err)
	}

	if vid.IsUpcoming {
		fmt.Printf("Upcoming live stream: %q @ %s\n", vid.Title, vid.URL())
	} else {
		fmt.Printf("Current live stream: %q @ %s\n", vid.Title, vid.URL())
	}
}

var c = http.Client{
	Timeout: 10 * time.Second,
}

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:86.0) Gecko/20100101 Firefox/86.0"

// This struct only contains minimal info, there is more but I don't care about other info we can get
type liveVideo struct {
	VideoID          string `json:"videoId"`
	Title            string `json:"title"`
	IsLive           bool   `json:"isLive"`
	ShortDescription string `json:"shortDescription"`
	IsUpcoming       bool   `json:"isUpcoming"`
}

// URL returns the youtube video URL for this live stream
func (lv *liveVideo) URL() string {
	var u = &url.URL{
		Scheme: "https",
		Host:   "www.youtube.com",
		Path:   "watch",
	}

	var q = u.Query()
	q.Set("v", lv.VideoID)
	u.RawQuery = q.Encode()

	return u.String()
}

var errNoVideo = errors.New("no live video found")

// youTubeLive extracts a live stream from a channel live url. This kind of URL looks like the following:
//     https://www.youtube.com/channel/UCSUu1lih2RifWkKtDOJdsBA/live
//     https://www.youtube.com/spacex/live
func youTubeLive(channelLiveURL string) (lv liveVideo, err error) {
	req, err := http.NewRequest(http.MethodGet, channelLiveURL, nil)
	if err != nil {
		return
	}

	// Set a few headers to look like a browser
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US;q=0.7,en;q=0.3")

	resp, err := c.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// Basically extract the video info and make sure it's live
	err = jsonextract.Objects(resp.Body, []jsonextract.ObjectOption{
		{
			Keys: []string{"videoId"},
			Callback: jsonextract.Unmarshal(&lv, func() bool {
				return lv.VideoID != "" && (lv.IsLive || lv.IsUpcoming)
			}),
			Required: true,
		},
	})

	// If we get this error, then no object with videoId was found
	// or the conditions of our callback were not true
	if errors.Is(err, jsonextract.ErrCallbackNeverCalled) {
		err = errNoVideo
	}

	return
}
