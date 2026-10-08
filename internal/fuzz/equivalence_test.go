package fuzz

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/xarantolus/jsonextract"
	"github.com/xarantolus/jsonextract/internal/reference"
	"github.com/xarantolus/jsonextract/internal/testinputs"
)

// The reference implementation is quadratic, so large corpus files are only compared on request:
//
//	go test ./internal/fuzz -run TestEquivalence -equiv.maxsize 1000000
var equivMaxSize = flag.Int("equiv.maxsize", 16*1024, "maximum size of corpus files that are compared against the reference implementation")

// collect returns all objects the given reader function extracts from input
func collect(read func(io.Reader, func([]byte) error) error, r io.Reader) (out []string, err error) {
	err = read(r, func(b []byte) error {
		out = append(out, string(b))
		return nil
	})
	return
}

func newReader(r io.Reader, cb func([]byte) error) error { return jsonextract.Reader(r, cb) }
func refReader(r io.Reader, cb func([]byte) error) error { return reference.Reader(r, cb) }

// readerWrappers simulate different io.Reader behaviours, e.g. ones that return very little data per call
var readerWrappers = []struct {
	name string
	wrap func([]byte) io.Reader
}{
	{"bytes", func(b []byte) io.Reader { return bytes.NewReader(b) }},
	{"onebyte", func(b []byte) io.Reader { return iotest.OneByteReader(bytes.NewReader(b)) }},
	{"half", func(b []byte) io.Reader { return iotest.HalfReader(bytes.NewReader(b)) }},
	{"dataerr", func(b []byte) io.Reader { return iotest.DataErrReader(bytes.NewReader(b)) }},
}

// checkEquivalent makes sure jsonextract.Reader returns exactly what the reference implementation returns
func checkEquivalent(t *testing.T, input []byte) {
	t.Helper()

	want, err := collect(refReader, bytes.NewReader(input))
	if err != nil {
		t.Fatalf("reference.Reader returned unexpected error: %v", err)
	}

	for _, w := range readerWrappers {
		got, err := collect(newReader, w.wrap(input))
		if err != nil {
			t.Errorf("[%s] Reader(%.100q) returned unexpected error: %v", w.name, input, err)
			continue
		}

		if len(got) != len(want) {
			t.Errorf("[%s] Reader(%.200q) returned %d objects, want %d\ngot:  %.300q\nwant: %.300q", w.name, input, len(got), len(want), got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("[%s] Reader(%.200q) object %d = %.300q, want %.300q", w.name, input, i, got[i], want[i])
			}
		}
	}
}

func TestEquivalenceSeeds(t *testing.T) {
	for i, s := range seeds() {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			checkEquivalent(t, s)
		})
	}
}

func TestEquivalenceFiles(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		input, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(filepath.Base(f), func(t *testing.T) {
			checkEquivalent(t, input)
		})
	}
}

func TestEquivalenceGenerators(t *testing.T) {
	for _, g := range testinputs.Generators {
		for _, n := range []int{0, 1, 2, 3, 10, 100} {
			input := g.Generate(n)
			t.Run(fmt.Sprintf("%s/n=%d", g.Name, n), func(t *testing.T) {
				checkEquivalent(t, input)
			})
		}
	}
}

func TestEquivalenceCorpus(t *testing.T) {
	files, err := filepath.Glob("corpus/*")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("no fuzz corpus found")
	}

	var skipped int
	for _, f := range files {
		input, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if len(input) > *equivMaxSize {
			skipped++
			continue
		}
		t.Run(filepath.Base(f), func(t *testing.T) {
			checkEquivalent(t, input)
		})
	}
	if skipped > 0 {
		t.Logf("skipped %d large corpus files, use -equiv.maxsize to include them", skipped)
	}
}

// TestEquivalenceSpanBoundaries moves tokens across the boundaries of the input the tokenizer gives
// to the lexer, which must not change the result
func TestEquivalenceSpanBoundaries(t *testing.T) {
	var snippets = []string{
		// A number directly before a long whitespace token
		"[1" + strings.Repeat(" ", 3000) + ", 2]",
		// Whitespace token after a structural token, followed by an HTML-like comment that is not at the start of a line
		"[1," + strings.Repeat(" ", 3000) + "--> 2]",
		"[1,\n-->comment\n 2]",
		// Tokens longer than the span
		"['" + strings.Repeat("a", 5000) + "', `" + strings.Repeat("b\n", 3000) + "`]",
		"{a: /" + strings.Repeat("c", 3000) + "/g, b: 0x" + strings.Repeat("f", 3000) + "}",
		"[" + strings.Repeat("9", 3000) + "]",
		// An unterminated string with brackets inside, spanning the boundary
		`{"a": "[1, 2, {b: 3}]` + strings.Repeat("x", 2000) + "\n}",
		// Identifier directly after a number is invalid
		"[5" + strings.Repeat(" ", 1000) + "6abc, [7]]",
	}

	for i, snippet := range snippets {
		// The span starts at the first bracket, so the snippet is moved inside of an array
		for pad := 1000; pad <= 1030; pad++ {
			var prefix = "[" + strings.Repeat("0,", pad/2) + strings.Repeat(" ", pad%2)
			input := []byte(prefix + snippet + "] " + snippet)
			t.Run(fmt.Sprintf("%d/pad=%d", i, pad), func(t *testing.T) {
				checkEquivalent(t, input)
			})
		}
	}
}
