package main

import (
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
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: jsonx <url/file>")
		flag.PrintDefaults()
	}
	flag.Parse()

	// Need exactly one argument, either an URL or a file path
	if flag.NArg() != 1 {
		flag.Usage()
		return
	}

	var (
		arg    = flag.Arg(0)
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

	err = jsonextract.Reader(reader, func(b []byte) error {
		callbackCount++

		fmt.Println(string(b))

		if callbackCount == *limit {

			if callbackCount == 1 {
				log.Println("Stopped extracting after one value")
			} else {
				log.Printf("Stopped extracting after %d values\n", *limit)
			}

			return jsonextract.ErrStop
		}

		return nil
	})
	if err != nil {
		log.Fatalln("Error while extracting:", err.Error())
	}
}
