package jsonextract

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/xarantolus/jsonextract/internal/testinputs"
)

// benchSizes are the input scales used by the generated benchmarks. Comparing ns/op
// between sizes shows how an algorithm scales (10x per step means linear time).
var benchSizes = []int{100, 1000, 10000}

func BenchmarkReader(b *testing.B) {
	for _, g := range testinputs.Generators {
		for _, n := range benchSizes {
			input := g.Generate(n)
			b.Run(fmt.Sprintf("%s/n=%d", g.Name, n), func(b *testing.B) {
				benchmarkReader(b, input)
			})
		}
	}
}

func BenchmarkReaderFiles(b *testing.B) {
	for _, file := range []string{"testdata/playlist.html", "testdata/repo.json", "testdata/test.html"} {
		input, err := os.ReadFile(file)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(file, func(b *testing.B) {
			benchmarkReader(b, input)
		})
	}
}

func BenchmarkObjectsPlaylist(b *testing.B) {
	input, err := os.ReadFile("testdata/playlist.html")
	if err != nil {
		b.Fatal(err)
	}

	options := []ObjectOption{
		{
			Keys:     []string{"videoId", "thumbnail", "title", "index", "lengthSeconds"},
			Callback: func([]byte) error { return nil },
		},
		{
			Keys:     []string{"urlCanonical", "title"},
			Callback: func([]byte) error { return nil },
		},
	}

	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	// Reusing the reader makes sure allocations of the library are measured, not those of the benchmark
	var r = bytes.NewReader(input)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.Reset(input)
		if err := Objects(r, options); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkReader(b *testing.B, input []byte) {
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	// Reusing the reader makes sure allocations of the library are measured, not those of the benchmark
	var r = bytes.NewReader(input)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.Reset(input)
		err := Reader(r, func([]byte) error { return nil })
		if err != nil {
			b.Fatal(err)
		}
	}
}
