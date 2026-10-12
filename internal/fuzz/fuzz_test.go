//go:build go1.18
// +build go1.18

package fuzz

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"testing/iotest"

	"github.com/xarantolus/jsonextract"
)

// FuzzReader checks properties of jsonextract.Reader that must hold for any input:
//
//	go test ./internal/fuzz -run '^$' -fuzz FuzzReader -fuzztime 10m
func FuzzReader(f *testing.F) {
	for _, s := range seeds() {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		got, err := collect(newReader, bytes.NewReader(input))
		if err != nil {
			t.Fatalf("Reader(%q) returned error: %v", input, err)
		}

		for _, obj := range got {
			if !json.Valid([]byte(obj)) {
				t.Errorf("Reader(%q) returned invalid JSON %q", input, obj)
			}
		}

		// The result must not depend on how the data arrives
		gotOneByte, err := collect(newReader, iotest.OneByteReader(bytes.NewReader(input)))
		if err != nil {
			t.Fatalf("Reader(%q) with one byte reader returned error: %v", input, err)
		}
		if !reflect.DeepEqual(got, gotOneByte) {
			t.Errorf("Reader(%q) = %q, but %q with one byte reader", input, got, gotOneByte)
		}

		err = jsonextract.Objects(bytes.NewReader(input), []jsonextract.ObjectOption{
			{
				Callback: func(b []byte) error {
					if !json.Valid(b) {
						t.Errorf("Objects(%q) passed invalid JSON %q", input, b)
					}
					return nil
				},
			},
		})
		if err != nil {
			t.Errorf("Objects(%q) returned error: %v", input, err)
		}
	})
}

// FuzzEquivalence compares jsonextract.Reader against the reference implementation:
//
//	go test ./internal/fuzz -run '^$' -fuzz FuzzEquivalence -fuzztime 10m
func FuzzEquivalence(f *testing.F) {
	for _, s := range seeds() {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		checkEquivalent(t, input)
	})
}
