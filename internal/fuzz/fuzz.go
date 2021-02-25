// +build gofuzz

package fuzz

import (
	"bytes"

	"github.com/xarantolus/jsonextract"
)

func Fuzz(data []byte) (ret int) {
	// Returns 1 for something that looked good, everything else is neutral (0)
	err := jsonextract.Reader(bytes.NewReader(data), func(b []byte) error {
		ret = 1

		return nil
	})
	if err != nil {
		panic(err)
	}

	return
}
