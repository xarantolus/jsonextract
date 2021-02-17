package main

import (
	"fmt"
	"strings"

	"github.com/xarantolus/jsonextract"
)

var test = `This text contains the following JSON object from https://httpbin.org/json: {
  "slideshow": {
    "author": "Yours Truly", 
    "date": "date of publication", 
    "slides": [
      {
        "title": "Wake up to WonderWidgets!", 
        "type": "all"
      }, 
      {
        "items": [
          "Why <em>WonderWidgets</em> are great", 
          "Who <em>buys</em> WonderWidgets"
        ], 
        "title": "Overview", 
        "type": "all"
      }
    ], 
    "title": "Sample Slide Show"
  }
}
That's it.
The parser could be confused by [ opening { brackets, but it should notice that they shouldn't be included.
`

func main() {
	err := jsonextract.Reader(strings.NewReader(test), func(b []byte) error {
		fmt.Println(string(b))
		return nil
	})
	if err != nil {
		panic("reader: " + err.Error())
	}
}
