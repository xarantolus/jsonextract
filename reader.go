package jsonextract

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
)

// Opening Characters as defined by the JSON spec
const (
	openObject = '{'
	openArray  = '['
)

var matchingBracket = map[byte]byte{
	'{': '}',
	'[': ']',
}

var (
	// ErrStop can be returned from a JSONCallback function to indicate that processing should stop now
	ErrStop = errors.New("stop processing json")
)

// JSONCallback is the callback function passed to Reader and ObjectOptions.
//
// Any JSON objects will be passed to it as bytes as defined by the function.
//
// If this function returns an error, processing will stop and return that error.
// If the returned error is ErrStop, processing will stop and return the nil error.
type JSONCallback func([]byte) error

// Reader reads all JSON and JavaScript objects from the input and calls callback for each of them.
//
// Errors returned from the callback will stop the method.
// The error will be returned, except if it is ErrStop which will cause the method to return nil.
//
// Please note that the reader must return UTF-8 bytes for this to work correctly.
func Reader(reader io.Reader, callback JSONCallback) (err error) {

	// Need to buffer in order to be able to unread invalid sections
	buffered := newResettableBuffer(reader)

	var r rune

	for {
		// Read character by character
		r, _, err = buffered.ReadRune()
		if err != nil {
			break
		}

		// We're looking for opening brackets
		if r == openArray || r == openObject {

			// We go back one rune so the JavaScript decoder will also read the opening brace
			err = buffered.UnreadRune()
			if err != nil {
				break
			}

			// Mark the start of our object. We can return here in case of errors
			buffered.MarkStart()

			var (
				msg           []byte
				readByteCount int
			)

			// Now we interpret the next bytes as JS object and convert them into JSON
			// since readJSObject might return invalid JSON, we must check the output
			msg, readByteCount, err = readJSObject(buffered)

			if err != nil || !json.Valid(msg) {
				// OK, so we tried to parse, but it didn't work.
				// We now just skip this opening brace and check the following data
				err = buffered.ReturnAndSkipOne()
				if err != nil {
					break
				}

				continue
			}

			// we read a certain amount of data that we should skip in the next round,
			// but we should restore anything we read that wasn't part of the object we returned
			// It is important to note that len(msg) is only equal to readByteCount if the
			// original io.Reader already contained a valid JSON object, but not if it was an JS object
			err = buffered.ReturnAndSkip(readByteCount)
			if err != nil {
				break
			}

			// Call the callback
			err = callback(msg)
			if err != nil {
				// ErrStop just stops, returns nil
				if err == ErrStop {
					err = nil
				}
				// The returned error
				return err
			}

			buffered.MarkEnd()
		}
	}

	if err == io.EOF {
		err = nil
	}

	return
}

// resettableRuneBuffer allows reading from a buffer, then resetting certain parts
type resettableRuneBuffer struct {
	// normalBuffer is just the normal buffered reader. It is used because it allows unreading runes
	normalBuffer *bufio.Reader

	// bufBefore stores all data that was read until we return to the beginning of an object
	bufBefore *bytes.Buffer

	// returnBuffer contains the content of bufBefore after we returned to a certain part we read before
	returnBuffer *bytes.Buffer

	// enableReturn defines whether the buffer should log what is read through it.
	// if true, one can return to any position after it was enabled
	enableReturn bool
}

func newResettableBuffer(r io.Reader) *resettableRuneBuffer {
	bir, ok := r.(*bufio.Reader)
	if !ok {
		bir = bufio.NewReader(r)
	}

	return &resettableRuneBuffer{
		normalBuffer: bir,
		returnBuffer: new(bytes.Buffer),
		bufBefore:    new(bytes.Buffer),
	}
}

// Read implements io.Reader
func (s *resettableRuneBuffer) Read(p []byte) (n int, err error) {
	n, _ = s.returnBuffer.Read(p)

	if n < len(p) {
		n2, err2 := s.normalBuffer.Read(p[n:])

		n += n2
		err = err2
	}

	if s.enableReturn {
		s.bufBefore.Write(p[:n])
	}

	return n, err
}

// ReadRune reads exactly one rune
func (s *resettableRuneBuffer) ReadRune() (r rune, size int, err error) {
	r, size, err = s.returnBuffer.ReadRune()
	if err != nil {
		r, size, err = s.normalBuffer.ReadRune()
	}

	if s.enableReturn {
		s.bufBefore.WriteRune(r)
	}

	return
}

// UnreadRune unreads the last rune read with ReadRune
func (s *resettableRuneBuffer) UnreadRune() (err error) {
	if s.enableReturn {
		_ = s.bufBefore.UnreadRune()
	}

	err = s.returnBuffer.UnreadRune()
	if err == nil {
		return
	}

	return s.normalBuffer.UnreadRune()
}

// ReturnAndSkipOne returns the buffer to the last reset (or initial) from an outside perspective,
// except that it skips one rune from the underlying stream
func (s *resettableRuneBuffer) ReturnAndSkipOne() (err error) {
	s.returnBuffer = s.bufBefore

	// Skip one rune
	_, _, err = s.returnBuffer.ReadRune()

	s.bufBefore = new(bytes.Buffer)

	return
}

// ReturnAndSkip returns the buffer to the last reset (or initial) from an outside perspective,
// except that it skips `offset` bytes from the input
func (s *resettableRuneBuffer) ReturnAndSkip(offset int) (err error) {
	s.returnBuffer = s.bufBefore

	if offset > 0 {
		_, err = io.CopyN(ioutil.Discard, s.returnBuffer, int64(offset))
	}

	s.bufBefore = new(bytes.Buffer)

	return
}

// MarkStart marks a restart point. When calling a return method, this
// start will be used
func (s *resettableRuneBuffer) MarkStart() {
	s.enableReturn = true
	s.bufBefore.Reset()
}

// MarkEnd disables returning until MarkStart is called the next time
func (s *resettableRuneBuffer) MarkEnd() {
	s.enableReturn = false
	// bufBefore.Len() == 0 at this moment

	if s.bufBefore.Len() != 0 {
		panic("wrong use of resettableRuneBuffer")
	}
}

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

// readJSObject converts the input data from `r` to JSON if possible.
// Input data should either already be JSON or a JavaScript object declaration.
// Please note that output might not be valid JSON and should be checked using json.Valid()
func readJSObject(r io.Reader) (output []byte, readInputBytes int, err error) {
	// Note: the current implementation of NewInput reads all bytes in the reader,
	// which is problematic for large files
	lex := js.NewLexer(parse.NewInput(r))

	// buf stores the bytes that should be returned in output
	var buf = new(bytes.Buffer)

	var (
		// since it's a dyck language, we just count the level of braces.
		// If we reach zero, we can stop parsing as we know this is the end of this object
		first byte
		level int
	)

	// lastByte stores the last byte we wrote to buf
	// It is used for detecting and correcting trailing commas
	var lastByte byte

	var merr error
loop:
	for {
		tt, text := lex.Next()
		if tt == js.ErrorToken {
			err = lex.Err()
			break loop
		}

		// The following code assumes len(text) > 0

		// First will always be either '{' or '['
		if readInputBytes == 0 {
			first = text[0]
		}

		readInputBytes += len(text)

		switch {
		case isIgnoredToken(tt):
			// Ignore tokens that are not needed for JSON.
			// We must continue so they are not seen as last written byte
			continue
		case js.IsIdentifier(tt):
			// Certain keywords are reserved in JSON. As a special case,
			// we replace "undefined" with "null"
			if val, ok := jsIdentifiers[string(text)]; ok {
				buf.Write(val)
			} else {
				// This is reached if we have an unquoted key in an object, e.g.
				//     { key: "value" }
				// We want to quote this identifier, as in marshal it into a string
				text, merr = json.Marshal(string(text))
				if merr != nil {
					err = merr
					break loop
				}
				buf.Write(text)
			}
		case tt == js.DivToken || tt == js.DivEqToken:
			// It is important that this comes before the IsPunctuator check
			// Basically if we find a '/', we suspect it's a regex
			tt, text = lex.RegExp()
			if tt != js.RegExpToken {
				err = fmt.Errorf("expected regex token when starting with '/', but was %s (lex err: %w)", tt.String(), lex.Err())
				break loop
			}

			// Regex patterns are just escaped and treated as strings,
			// no need to skip the entire object
			text, merr = json.Marshal(string(text))
			if merr != nil {
				err = merr
				break loop
			}
			buf.Write(text)
		case js.IsPunctuator(tt):
			if len(text) > 1 {
				err = fmt.Errorf("unexpected token %q in JS value", string(text))
				break loop
			}

			switch text[0] {
			case '{', '[':
				if text[0] == first {
					level++
				}

				if lastByte == '{' && text[0] == '{' {
					err = fmt.Errorf("Opening brace { cannot come after another opening brace")
					break loop
				}

				buf.Write(text)
			case ']', '}':
				if text[0] == matchingBracket[first] {
					level--
				}

				// An array/object with trailing comma was found.
				// Example: [1, 2, 3, ]
				if lastByte == ',' {
					// We remove the comma to also support those objects.
					buf.Truncate(buf.Len() - 1)
				}

				buf.Write(text)

				// We finished the JS object that was started with `first`. Time to stop
				if level == 0 {
					break loop
				}
			case '+':
				if '0' <= lastByte && lastByte <= '9' {
					err = fmt.Errorf("cannot use '+' to add numbers/strings")
					break loop
				}
				// continue with buf.Write
				fallthrough
			default:
				// This could e.g. be a "-" in front of a number
				buf.Write(text)
			}
		case tt == js.StringToken:
			// Special quotes must be handled
			if text[0] == '\'' {
				buf.WriteString(singleQuoteReplacer.Replace(string(text)))
				// Break out of switch to continue with the lastByte assignment below
				break
			}

			if text[0] == '"' {
				// A normal string
				buf.Write(text)
				// Continue with lastByte assignment
				break
			}

			err = fmt.Errorf("unsupported string type (text: %s)", string(text))
			break loop
		case tt == js.TemplateToken:
			if len(text) <= 2 {
				err = fmt.Errorf("Expected string to have at least quotes, but that didn't happen")
				break loop
			}

			var toEscape = templateQuoteReplacer.Replace(string(text[1 : len(text)-1]))

			text, merr = json.Marshal(string(toEscape))
			if merr != nil {
				err = merr
				break loop
			}

			buf.Write(text)
		case js.IsNumeric(tt):
			// Not all JS numbers are valid JSON numbers, e.g. the following are valid in JS, but not JSON:
			// +5, 0x3, 0o4, 0b1001, -0x3, 8n

			// If the number starts with a '+', we already wrote it. Remove it again, as plus signs are not valid json numbers
			if lastByte == '+' {
				buf.Truncate(buf.Len() - 1)
			}

			switch tt {
			case js.BigIntToken:
				// BigIntegers can be written e.g. as "50n", "0x5n" etc.
				text = bytes.TrimSuffix(text, []byte("n"))
				fallthrough
			default:
				text = transformNumber(text)
			}

			buf.Write(text)
		default:
			// There shouldn't be much left. But in case it's valid JSON, we keep it
			buf.Write(text)
		}

		lastByte = text[len(text)-1]
	}

	if err == nil || err == io.EOF {
		return buf.Bytes(), readInputBytes, nil
	}
	return nil, 0, err
}

func isIgnoredToken(tt js.TokenType) bool {
	return tt == js.WhitespaceToken || tt == js.LineTerminatorToken || tt == js.CommentToken || tt == js.CommentLineTerminatorToken
}

// transformNumber transforms the given number to a decimal number, if possible. Might return
// invalid JSON data
func transformNumber(number []byte) []byte {
	var out = make([]byte, 0, len(number))

	// "+"-Prefix is not valid JSON, just strip it
	if number[0] == '+' {
		number = number[1:]
	} else if number[0] == '-' {
		// Keep "-" sign
		number = number[1:]
		out = append(out, '-')
	}

	// Just parse the number. This also deals with leading zeros, all kinds of
	// number literals (e.g. 1_00 == 100) etc.
	ui, err := strconv.ParseUint(string(number), 0, 64)
	if err != nil {
		// this can happen if the number is a float. We just leave it as that, it should be accepted by JSON parsers
		return append(out, number...)
	}

	// Now convert to decimal
	return strconv.AppendUint(out, ui, 10)
}
