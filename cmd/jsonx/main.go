package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"runtime/debug"
	"time"

	"github.com/xarantolus/jsonextract"
)

var (
	limit = flag.Int("limit", -1, "Stop extracting after this many objects")

	possibleUserAgents = []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:86.0) Gecko/20100101 Firefox/86.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/42.0.2311.135 Safari/537.36 Edge/12.246",
		"Mozilla/5.0 (X11; CrOS x86_64 8172.45.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.64 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_2) AppleWebKit/601.3.9 (KHTML, like Gecko) Version/9.0.2 Safari/601.3.9",
	}

	client = http.Client{
		Timeout: time.Minute,
	}
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: jsonx <url/file> [keys...]\n\nFlags:")
		flag.PrintDefaults()
		fmt.Fprintln(flag.CommandLine.Output(), "\nNotes:\nIf you specify keys, only objects with all of them will be printed.")
		fmt.Fprintln(flag.CommandLine.Output(), "You can also pipe input into this program when specifying '-' as input file.")

		info, ok := debug.ReadBuildInfo()
		if ok {
			fmt.Fprintln(flag.CommandLine.Output(), "Compiled with version "+info.Main.Version)
		}
	}
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		return
	}

	var sourceArg = flag.Arg(0)

	var (
		keys   []string
		reader io.Reader
	)

	// Determine where to read data from
	if sourceArg == "-" {
		reader = os.Stdin
	} else {
		// Check if it's an URL or file and set reader accordingly
		u, err := url.ParseRequestURI(sourceArg)
		if err == nil && (u.Scheme == "http" || u.Scheme == "https") {
			// If yes, we download it
			req, err := http.NewRequest(http.MethodGet, u.String(), nil)
			if err != nil {
				log.Fatalln("Creating request:", err.Error())
			}

			rand.Seed(time.Now().UnixNano())

			// Set a few headers to look like a browser
			req.Header.Set("User-Agent", possibleUserAgents[rand.Intn(len(possibleUserAgents))])
			req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
			req.Header.Set("Accept-Language", "en-US;q=0.7,en;q=0.3")

			resp, err := client.Do(req)
			if err != nil {
				log.Fatalln("Downloading:", err.Error())
			}
			defer resp.Body.Close()

			reader = resp.Body
		} else {
			// Seems like we got a file name
			f, err := os.Open(sourceArg)
			if err != nil {
				log.Fatalln("Opening file:", err.Error())
			}
			defer f.Close()

			reader = f
		}
	}

	// First argument was the URL/file, everything else is keys
	keys = flag.Args()[1:]

	// for callback limit
	var callbackCount int

	var callback = func(b []byte) error {
		callbackCount++

		// Copy bytes to Stdout
		_, err := io.Copy(os.Stdout, bytes.NewReader(append(b, '\n')))
		if err != nil {
			panic(err)
		}

		if callbackCount == *limit {
			return jsonextract.ErrStop
		}

		return nil
	}

	var err error

	// If no keys are given, we extract all objects and print them
	if len(keys) == 0 {
		// This also prints arrays, while Objects wouldn't do that
		err = jsonextract.Reader(reader, callback)
	} else {
		// If keys are given, we only print objects with those keys
		err = jsonextract.Objects(reader, []jsonextract.ObjectOption{
			{
				Keys:     keys,
				Callback: callback,
			},
		})
	}
	if err != nil {
		log.Fatalln("Error while extracting:", err.Error())
	}
}
