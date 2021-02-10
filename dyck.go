package jsonextract

import "encoding/json"

const (
	openObject = '{'
	openArray  = '['

	closeObject = '}'
	closeArray  = ']'
)

var closing = map[rune]rune{
	openObject: closeObject,
	openArray:  closeArray,
}

// String taks the given String and extracts all valid JSON objects / Arrays it can find
func String(data string) (extracted []string) {
	for i, r := range []rune(data) {
		if r == openObject || r == openArray {
			closingPosition := nextBracket(data, r, closing[r], i)
			if closingPosition == -1 {
				continue
			}

			var roi = data[i:closingPosition]

			if json.Valid([]byte(roi)) {
				extracted = append(extracted, roi)
			}
		}
	}

	return
}

func nextBracket(text string, open, close rune, start int) int {
	if len(text) < start {
		return -1
	}

	var level int

	for i := start; i < len(text); i++ {
		if rune(text[i]) == open {
			level++
		} else if rune(text[i]) == close {
			level--

			if level == 0 {
				return i + 1
			}
		}
	}

	return -1
}
