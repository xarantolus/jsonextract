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
	}
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		return
	}

	var (
		arg    = flag.Arg(0)
		keys   = flag.Args()[1:]
		reader io.Reader
	)

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
