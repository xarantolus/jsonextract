package jsonextract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/tdewolff/parse/v2/js"
)

var jsIdentifiers = map[string][]byte{
	"true":  []byte("true"),
	"false": []byte("false"),
	"null":  []byte("null"),
	// Special cases
	// treat undefined as null
	"undefined": []byte("null"),
	// treat NaN as null
	"NaN": []byte("null"),
}

// singleQuoteReplacer replaces a single quoted string to be double-quoted
var singleQuoteReplacer = strings.NewReplacer(
	// Replace single quotes with double, ' => "
	"'", "\"",
	// Escape quotes from before, " => \"
	"\"", "\\\"",
	// unescape single quotes from before, \' => '
	"\\'", "'",
)

// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Template_literals
var templateQuoteReplacer = strings.NewReplacer(
	// Escaped quotes become normal characters
	"\\`", "`",
)

// converter converts JavaScript tokens to JSON, one token at a time.
// Its output for a sequence of tokens might not be valid JSON and must be checked.
type converter struct {
	// lastByte is the last byte of the last token that was not ignored.
	// It is used for detecting and correcting trailing commas and signs
	lastByte  byte
	lastToken js.TokenType
}

// convert appends the JSON representation of the token to out.
// Some tokens make the token before them obsolete, e.g. a trailing comma before a closing bracket
// or a '+' before a number. In that case, the last byte of out is removed and dropped is true.
//
// Regular expressions must already be lexed as js.RegExpToken.
// Ignored tokens (see isIgnoredToken) must not be passed to this method.
func (c *converter) convert(out []byte, tt js.TokenType, text []byte) (_ []byte, dropped bool, err error) {
	// drop removes the last byte that was written
	drop := func() {
		out = out[:len(out)-1]
		dropped = true
	}

	// The following code assumes len(text) > 0
	switch {
	case tt == js.RegExpToken:
		// Regex patterns are just escaped and treated as strings,
		// no need to skip the entire object
		text = marshalString(string(text))
		out = append(out, text...)
	case js.IsIdentifier(tt):
		// Certain keywords are reserved in JSON. As a special case,
		// we replace "undefined" with "null"
		if val, ok := jsIdentifiers[string(text)]; ok {
			// Another special case: this handles stuff like -NaN, which would
			// result in "-null", which is invalid JSON
			if c.lastByte == '+' || c.lastByte == '-' {
				drop()
			}
			out = append(out, val...)
		} else {
			// This is reached if we have an unquoted key in an object, e.g.
			//     { key: "value" }
			// We want to quote this identifier, as in marshal it into a string
			text = marshalString(string(text))
			out = append(out, text...)
		}
	case js.IsPunctuator(tt):
		if len(text) > 1 {
			return nil, false, fmt.Errorf("unexpected token %q in JS value", string(text))
		}

		switch text[0] {
		case '{', '[':
			if c.lastByte == '{' && text[0] == '{' {
				return nil, false, fmt.Errorf("opening brace { cannot come after another opening brace")
			}
		case ']', '}':
			// An array/object with trailing comma was found.
			// Example: [1, 2, 3, ]
			if c.lastByte == ',' {
				// We remove the comma to also support those objects.
				drop()
			}
		case '+':
			if '0' <= c.lastByte && c.lastByte <= '9' {
				return nil, false, fmt.Errorf("cannot use '+' to add numbers/strings")
			}
		}
		// This could e.g. be a "-" in front of a number
		out = append(out, text...)
	case tt == js.StringToken:
		switch text[0] {
		case '\'':
			// Special quotes must be handled
			out = append(out, singleQuoteReplacer.Replace(string(text))...)
		case '"':
			// A normal string
			out = append(out, text...)
		default:
			return nil, false, fmt.Errorf("unsupported string type (text: %s)", string(text))
		}
	case tt == js.TemplateToken:
		if len(text) <= 2 {
			return nil, false, fmt.Errorf("expected string to have at least quotes, but that didn't happen")
		}

		var toEscape = templateQuoteReplacer.Replace(string(text[1 : len(text)-1]))

		text = marshalString(toEscape)
		out = append(out, text...)
	case js.IsNumeric(tt):
		if js.IsNumeric(c.lastToken) {
			return nil, false, fmt.Errorf("invalid: writing two numbers directly after each other")
		}
		// Not all JS numbers are valid JSON numbers, e.g. the following are valid in JS, but not JSON:
		// +5, 0x3, 0o4, 0b1001, -0x3, 8n

		// If the number starts with a '+', we already wrote it. Remove it again, as plus signs are not valid json numbers
		if c.lastByte == '+' {
			drop()
		}

		if tt == js.IntegerToken {
			// BigIntegers can be written e.g. as "50n", "0x5n" etc.
			text = bytes.TrimSuffix(text, []byte("n"))
		}
		text = transformNumber(text)
		out = append(out, text...)
	default:
		// There shouldn't be much left. But in case it's valid JSON, we keep it
		out = append(out, text...)
	}

	c.lastByte = text[len(text)-1]
	c.lastToken = tt

	return out, dropped, nil
}

// marshalString returns s as JSON string
func marshalString(s string) []byte {
	// Marshalling a string cannot fail, invalid UTF-8 is replaced
	b, _ := json.Marshal(s)
	return b
}

func isIgnoredToken(tt js.TokenType) bool {
	return tt == js.WhitespaceToken || tt == js.LineTerminatorToken || tt == js.CommentToken || tt == js.CommentLineTerminatorToken
}

// transformNumber transforms the given number to a decimal number, if possible. Might return
// invalid JSON data
func transformNumber(number []byte) []byte {
	if isDecimalInteger(number) {
		// Most numbers are already valid JSON
		return number
	}

	var out = make([]byte, 0, len(number))

	// "+"-Prefix is not valid JSON, just strip it
	switch number[0] {
	case '+':
		number = number[1:]
	case '-':
		// Keep "-" sign
		number = number[1:]
		out = append(out, '-')
	}

	// Just parse the number. This also deals with leading zeros, all kinds of
	// number literals (e.g. 1_00 == 100) etc.
	ui, err := strconv.ParseUint(string(number), 0, 64)
	if err != nil {
		// If we have exactly one dot, and it is at the end
		// e.g. the number "15."" should be interpreted as "15.0"
		if number[len(number)-1] == '.' && bytes.IndexByte(number, '.') == len(number)-1 {
			return append(out, append(number, '0')...)
		}

		// this can happen if the number is a float. We just leave it as that, it should be accepted by JSON parsers
		return append(out, number...)
	}

	// Now convert to decimal
	return strconv.AppendUint(out, ui, 10)
}

// isDecimalInteger returns whether number is a decimal integer without sign and leading zeros
func isDecimalInteger(number []byte) bool {
	if len(number) == 0 || number[0] == '0' && len(number) > 1 {
		return false
	}
	for _, c := range number {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
