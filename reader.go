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
	// ErrStop can be returned from a JSONCallback function to signal that processing should stop
	// at this object
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
func Reader(reader io.Reader, callback JSONCallback) (err error) {

	// Need to buffer in order to be able to unread invalid sections
	buffered := resettableRuneBuffer{
		normalBuffer: bufio.NewReader(reader),
	}

	for {
		// Read character by character
		r, _, err := buffered.ReadRune()
		if err != nil {
			break
		}

		// We're looking for opening brackets
		if r == openArray || r == openObject {

			// We go back one rune so the JSON decoder will also read the opening brace
			err = buffered.UnreadRune()
			if err != nil {
				break
			}

			// Reset our "before" buffer, as it stores anything we read so far since the last
			// reset. This makes sure we return to the currently read rune in case it's not a valid object
			buffered.bufBefore.Reset()

			var (
				msg           []byte
				readByteCount int
			)

			// Now we just let the default decoder parse this JSON data
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

			// we read a certain amount of data that we should skip in the next round
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
	normalBuffer *bufio.Reader

	bufBefore bytes.Buffer

	returnBuffer bytes.Buffer
}

// Read implements io.Reader
func (s *resettableRuneBuffer) Read(p []byte) (n int, err error) {
	if s.returnBuffer.Len() != 0 {
		n, err = s.returnBuffer.Read(p)
		if err == io.EOF {
			err = nil
		}

		s.bufBefore.Write(p[:n])
	}

	n2, err2 := s.normalBuffer.Read(p[n:])

	s.bufBefore.Write(p[n : n+n2])
	n += n2

	return n, err2
}

// ReadRune reads exactly one rune
func (s *resettableRuneBuffer) ReadRune() (r rune, size int, err error) {
	r, size, err = s.returnBuffer.ReadRune()
	if err != nil {
		r, size, err = s.normalBuffer.ReadRune()
	}

	s.bufBefore.WriteRune(r)

	return
}

// UnreadRune unreads the last rune read with ReadRune
func (s *resettableRuneBuffer) UnreadRune() (err error) {
	_ = s.bufBefore.UnreadRune()

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

	s.bufBefore = bytes.Buffer{}

	return
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

// readJSObject converts the input data from `r` to JSON if possible.
// Input data should either already be JSON or a JavaScript object declaration.
// Please note that output might not be valid JSON and should be checked using json.Valid()
func readJSObject(r io.Reader) (output []byte, readInputBytes int, err error) {
	lex := js.NewLexer(r)

	var (
		buf = bytes.Buffer{}
	)

	var (
		first byte
		level int
	)

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
			// Ignore tokens that are not needed for JSON
		case js.IdentifierToken:
			// Quote keys in maps
			buf.Write([]byte(strconv.Quote(string(text))))
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
				buf.Write(text)
			case ']', '}':
				if text[0] == matchingBracket[first] {
					level--
				}
				buf.Write(text)

				// We finished the JS object that was started. Time to stop
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
			if text[0] == '"' {
				buf.Write(text)
				continue
			}

			// Most likely a single quoted string
			var s string
			err = json.Unmarshal(text, &s)
			if err != nil {
				break loop
			}

			quoted, merr := json.Marshal(s)
			if err != nil {
				err = merr
				break loop
			}

			buf.Write(quoted)
		default:
			buf.Write(text)
		}
	}

	if err == nil || err == io.EOF {
		return buf.Bytes(), readInputBytes, nil
	}
	return nil, 0, err
}
