package jsonextract

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/tdewolff/parse/js"
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
	// ErrStop can be returned from a JSONCallback function to indicate that processing should stop at this object
	ErrStop = errors.New("stop processing json")
)

// JSONCallback is the callback function passed to Reader.
// Found JSON objects will be passed to it as bytes.
// If this function returns an error, processing will stop and return that error.
// If the returned error is ErrStop, processing will stop but not return an error.
type JSONCallback func([]byte) error

// Reader reads all JSON and JavaScript objects from the input and calls callback for each of them.
// If callback returns an error, Reader will stop processing and return the error.
// If the returned error is ErrStop, Reader will return nil instead of the error.
// Please note that reader must return UTF-8 bytes, if you're not sure use the charset.NewReader
// method to convert to the correct charset (https://pkg.go.dev/golang.org/x/net/html/charset#NewReader)
func Reader(reader io.Reader, callback JSONCallback) (err error) {

	// Need to buffer in order to be able to unread invalid sections
	buffered := resettableRuneBuffer{
		normalBuffer: bufio.NewReader(reader),
	}

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
			msg, readByteCount, err = readJSObject(&buffered)
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

// ReaderObjects takes the given io.Reader and reads all possible JSON and JavaScript objects it can find
func ReaderObjects(reader io.Reader) (objects []json.RawMessage, err error) {
	return objects, Reader(reader, func(b []byte) error {
		objects = append(objects, b)
		return nil
	})
}

// resettableRuneBuffer allows reading from a buffer, then resetting certain parts
type resettableRuneBuffer struct {
	// normalBuffer is just the normal buffered reader. It is used because it allows unreading runes
	normalBuffer *bufio.Reader

	// bufBefore stores all data that was read until we return to the beginning of an object
	bufBefore bytes.Buffer

	// returnBuffer contains the content of bufBefore after we returned to a certain part we read before
	returnBuffer bytes.Buffer

	// enableReturn defines whether the buffer should log what is read through it.
	// if true, one can return to any position after it was enabled
	enableReturn bool
}

// Read implements io.Reader
func (s *resettableRuneBuffer) Read(p []byte) (n int, err error) {
	if s.returnBuffer.Len() != 0 {
		n, err = s.returnBuffer.Read(p)
		if err == io.EOF {
			err = nil
		}

		if s.enableReturn {
			s.bufBefore.Write(p[:n])
		}
	}

	n2, err2 := s.normalBuffer.Read(p[n:])

	if s.enableReturn {
		s.bufBefore.Write(p[n : n+n2])
	}
	n += n2

	return n, err2
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

	s.bufBefore.Reset()

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
}

// ReturnAndSkip returns the buffer to the last reset (or initial) from an outside perspective,
// except that it skips `offset` bytes from the input
func (s *resettableRuneBuffer) ReturnAndSkip(offset int) (err error) {
	s.returnBuffer = s.bufBefore

	if offset > 0 {
		_, err = io.CopyN(ioutil.Discard, s, int64(offset))
	}

	s.bufBefore = bytes.Buffer{}

	return
}

var jsIdentifiers = map[string]bool{
	"true":  true,
	"false": true,
	"null":  true,
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
	lex := js.NewLexer(r)

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

loop:
	for {
		tt, text := lex.Next()
		if tt == js.ErrorToken {
			err = lex.Err()
			break loop
		}

		// The following code assumes len(text) > 0

		// First will always be either '{' or '{'
		if readInputBytes == 0 {
			first = text[0]
		}

		readInputBytes += len(text)

		switch tt {
		case js.SingleLineCommentToken, js.MultiLineCommentToken, js.WhitespaceToken, js.LineTerminatorToken:
			// Ignore tokens that are not needed for JSON.
			// We must continue so they are not seen as last written byte
			continue
		case js.IdentifierToken:
			// Quote keys/values, except if they are special
			if jsIdentifiers[string(text)] {
				buf.Write(text)
			} else {
				// Quote this identifier, as in interpret it as string
				data, merr := json.Marshal(string(text))
				if merr != nil {
					err = merr
					break loop
				}
				buf.Write(data)
			}
		case js.PunctuatorToken:
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
			case ':', ',':
				buf.Write(text)
			default:
				err = fmt.Errorf("unexpected token %q in JS value", string(text))
				break loop
			}
		case js.StringToken:
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
		case js.TemplateToken:
			var toEscape = templateQuoteReplacer.Replace(string(text[1 : len(text)-1]))

			data, merr := json.Marshal(string(toEscape))
			if merr != nil {
				err = merr
				break loop
			}

			buf.Write(data)
		case js.RegexpToken:
			// Regex patterns are just escaped and treated as strings,
			// no need to skip the entire object
			data, merr := json.Marshal(string(text))
			if merr != nil {
				err = merr
				break loop
			}
			buf.Write(data)
		default:
			// Basically only numbers are left, i guess?
			buf.Write(text)
		}

		lastByte = text[len(text)-1]
	}

	if err == nil || err == io.EOF {
		return buf.Bytes(), readInputBytes, nil
	}
	return nil, 0, err
}
