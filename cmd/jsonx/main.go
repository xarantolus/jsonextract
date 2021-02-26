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
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: jsonx <url/file> [keys...]")
		flag.PrintDefaults()
		fmt.Fprintln(flag.CommandLine.Output(), "\nYou can also pipe input into this program and only specify keys")
	}
	flag.Parse()

	var (
		keys   []string
		reader io.Reader
	)

	stat, _ := os.Stdin.Stat()
	if stat != nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		// Something is being piped into this process
		reader = os.Stdin
		keys = flag.Args()
	} else {
		// Stdin is a terminal, process input normally

		if flag.NArg() == 0 {
			flag.Usage()
			return
		}

		var arg = flag.Arg(0)

		// Check if it's an URL or file and set reader accordingly
		u, err := url.ParseRequestURI(arg)
		if err == nil && (u.Scheme == "http" || u.Scheme == "https") {
			// If yes, we download it
			resp, err := http.Get(u.String())
			if err != nil {
				log.Fatalln("Downloading:", err.Error())
			}
			defer resp.Body.Close()

			reader = resp.Body
		} else {
			// So it must be a file
			f, err := os.Open(arg)
			if err != nil {
				log.Fatalln("Opening file:", err.Error())
			}
			defer f.Close()

			reader = f
		}

		// First argument was the URL/file, everything else is keys
		keys = flag.Args()[1:]
	}

	// for callback limit
	var callbackCount int

	var callback = func(b []byte) error {
		callbackCount++

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
