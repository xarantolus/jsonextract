// Package testinputs generates inputs for benchmarks and equivalence tests.
package testinputs

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"
)

// Generator produces an input of roughly n "units", e.g. objects or brackets
type Generator struct {
	Name     string
	Generate func(n int) []byte
}

// Generators cover interesting edge cases for the extraction algorithm, not just typical inputs
var Generators = []Generator{
	{
		// Many opening brackets that are never closed, except the last one.
		// Every opening bracket is a failed candidate
		"DeepOpenArrays",
		func(n int) []byte { return []byte(strings.Repeat("[", n) + "]") },
	},
	{
		"DeepOpenObjects",
		func(n int) []byte { return []byte(strings.Repeat("{a:", n) + "1}") },
	},
	{
		// Balanced nesting, the best case: one candidate that succeeds
		"DeepValidNested",
		func(n int) []byte { return []byte(strings.Repeat("[", n) + strings.Repeat("]", n)) },
	},
	{
		// Unbalanced nesting with a valid object deep inside
		"DeepOpenWithValidInner",
		func(n int) []byte {
			return []byte(strings.Repeat("{x: [", n) + `{"a": 1, b: 'c'}`)
		},
	},
	{
		"ManySmallObjectsGibberish",
		func(n int) []byte {
			var (
				rng = rand.New(rand.NewSource(1))
				buf bytes.Buffer
			)
			const gibberish = "abcdefghijklmnopqrstuvwxyz      .,;:!?<>/=+-*()\n'\"{[]}"
			for i := 0; i < n; i++ {
				fmt.Fprintf(&buf, "{a: %d, b: 'x', c: [1, 2, {d: null}]}", i)
				for j := rng.Intn(40); j > 0; j-- {
					buf.WriteByte(gibberish[rng.Intn(len(gibberish))])
				}
			}
			return buf.Bytes()
		},
	},
	{
		"ManyEmpty",
		func(n int) []byte { return []byte(strings.Repeat("{}", n)) },
	},
	{
		"UnbalancedClosers",
		func(n int) []byte { return []byte(strings.Repeat("}]}[}{]", n)) },
	},
	{
		// Brackets inside of strings only become candidates once the surrounding candidate failed
		"BracketsInStrings",
		func(n int) []byte {
			return []byte(strings.Repeat(`x = ["[1,2]", '{a:1}', "{"]; y("[", `+"`{b: 2}`"+");\n", n))
		},
	},
	{
		"LargeSingleArray",
		func(n int) []byte {
			var buf bytes.Buffer
			buf.WriteString("var graphData = [")
			for i := 0; i < n; i++ {
				if i > 0 {
					buf.WriteByte(',')
				}
				fmt.Fprintf(&buf, "%d", 20000+i*7)
			}
			buf.WriteString("];")
			return buf.Bytes()
		},
	},
}
