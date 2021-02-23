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

	type quotedexample struct {
		Quoted      *int   `json:"quoted"`
		OtherQuotes bool   `json:"other quotes"`
		Unquoted    string `json:"unquoted"`
	}

	var obj = quotedexample{}

	err = jsonextract.Objects(f, []jsonextract.ObjectOption{
		{
			Keys: []string{"quoted", "other quotes"},
			Callback: jsonextract.Unmarshal(&obj, func() bool {
				return obj.Quoted != nil
			}),
		},
	})
	if err != nil {
		panic("Object extraction: " + err.Error())
	}

	fmt.Printf("Extracted other quotes value: %v\n", obj.OtherQuotes)
}
