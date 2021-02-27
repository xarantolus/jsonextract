package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/xarantolus/jsonextract"
)

var (
	limit = flag.Int("limit", -1, "Stop extracting after this many objects")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: jsonx <url/file> [keys...]\n\nFlags:")
		flag.PrintDefaults()
		fmt.Fprintln(flag.CommandLine.Output(), "\nNotes:\nIf you specify keys, only objects with all of them will be printed.")
		fmt.Fprintln(flag.CommandLine.Output(), "You can also pipe input into this program when specifying '-' as input file.")
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
			resp, err := http.Get(u.String())
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
