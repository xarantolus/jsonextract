package main

import (
	"fmt"
	"os"

	"github.com/xarantolus/jsonextract"
)

func main() {
	f, err := os.Open("README.md")
	if err != nil {
		panic("cannot open file: " + err.Error())
	}
	defer f.Close()

	err = jsonextract.Reader(f, func(b []byte) error {
		fmt.Println(string(b))
		return nil
	})
	if err != nil {
		panic("reader: " + err.Error())
	}
}
