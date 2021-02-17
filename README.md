# jsonextract
`jsonextract` is a Go library for extracting JSON objects from any source. It can be used for data extraction tasks like web scraping.


### Examples
Here is an example program that extracts all JSON objects from a file and prints them to the console:

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/xarantolus/jsonextract"
)

func main() {
	file, err := os.Open("file.html")
	if err != nil {
		log.Fatalln(err.Error())
	}
	defer file.Close()

	// Print all JSON objects and arrays found in file.html
	err = jsonextract.Reader(file, func(b []byte) error {
		fmt.Println(string(b))

		return nil
	})
	if err != nil {
		log.Fatalln(err.Error())
	}
}
```

### Extractor program
There's a small extractor program that uses this library to get data from URLs and files.

If you want to give it a try, you can just go-get it:

    go get -u github.com/xarantolus/jsonextract/cmd/jsonx

You can use it on files or URLs, e.g. like this:

    jsonx reader_test.go

or on URLs like this:

    jsonx "https://stackoverflow.com/users/5728357/xarantolus?tab=topactivity"

### Other examples
There are also examples in the [`examples`](examples/) subdirectory.

The [string example](examples/string/main.go) shows how to use the package to quickly get all JSON objects/arrays in a string, it uses an [`strings.Reader`](https://pkg.go.dev/strings#NewReader) for that.

The [`stackoverflow-chart` example](examples/stackoverflow-chart/main.go) shows how to extract the reputation chart data of a StackOverflow user. Extracted data is then used to draw the same chart using Go:

![Comparing chart from StackOverflow and the scraped and drawn result](.github/img/comparison-stackoverflow.png)


### [License](LICENSE)
This is free as in freedom software. Do whatever you like with it.
