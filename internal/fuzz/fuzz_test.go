//go:build go1.18
// +build go1.18

package fuzz

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzEquivalence compares jsonextract.Reader against the reference implementation:
//
//	go test ./internal/fuzz -run ^$ -fuzz FuzzEquivalence -fuzztime 5m
func FuzzEquivalence(f *testing.F) {
	files, _ := filepath.Glob("corpus/*")
	for _, file := range files {
		input, err := os.ReadFile(file)
		if err == nil && len(input) <= 4096 {
			f.Add(input)
		}
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		checkEquivalent(t, input)
	})
}
