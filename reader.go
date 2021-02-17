package jsonextract

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
)

// Please note that the implementations of Reader and ReaderObject are almost the same,
// so if any changes are made they should be made in both functions. One could also implement
// ReaderObjects using Reader and an appropriate callback function.

// Opening Characters as defined by the JSON spec
const (
	openObject = '{'
	openArray  = '['
)

var (
	// ErrStop can be returned from a JSONCallback function to signal that processing should stop
	// at this object
	ErrStop = errors.New("stop processing json")
)

// JSONCallback is the callback function passed to Reader.
// Found JSON objects will be passed to it as bytes.
// If this function returns an error, processing will stop and return that error.
// If the returned error is ErrStop, processing will stop without an error.
type JSONCallback func([]byte) error

// Reader reads all JSON objects from the input and calls callback for each of them.
// If callback returns an error, Reader will stop processing and return the error.
// If the returned error is ErrStop, Reader will return nil instead of the error.
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

			// We go back one rune so the JSON decoder will also read the opening brace			// We go back one rune so the JSON decoder will also read that one
			err = buffered.UnreadRune()
			if err != nil {
				break
			}

			// Reset our "before" buffer, as it stores anything we read so far since the last
			// reset. This makes sure we return to the currently read rune in case it's not a valid object
			buffered.bufBefore.Reset()

			var msg json.RawMessage

			// Now we just let the default decoder parse this JSON data
			err = json.NewDecoder(&buffered).Decode(&msg)
			if err != nil {
				// OK, so we tried to parse, but it didn't work.
				// We skip the currently read rune (either '{' or ']') and continue with the next one
				err = buffered.ReturnAndSkipOne()
				if err != nil {
					break
				}

				continue
			}

			// OK, so we read a valid JSON object into msg.
			// Since the default json decoder reads more than it needs to decode, we now
			// have to reset everything it read to much, which is everything *but* the bytes
			// we read into the decoded object
			err = buffered.ReturnAndSkip(len(msg))
			if err != nil {
				break
			}

			// Call the callback
			err = callback(msg)
			if err != nil {
				break
			}
		}
	}

	if err == io.EOF || err == ErrStop {
		err = nil
	}

	return
}

// ReaderObjects takes the given io.Reader and reads all possible JSON objects it can find in it.
// Assumes the stream to consist of utf8 bytes
func ReaderObjects(reader io.Reader) (objects []json.RawMessage, err error) {

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

			var msg json.RawMessage

			// Now we just let the default decoder parse this JSON data
			err = json.NewDecoder(&buffered).Decode(&msg)
			if err != nil {
				// OK, so we tried to parse, but it didn't work.
				// We skip the currently read rune (either '{' or ']') and continue with the next one
				err = buffered.ReturnAndSkipOne()
				if err != nil {
					break
				}

				continue
			}

			// OK, so we read a valid JSON object into msg.
			// Since the default json decoder reads more than it needs to decode, we now
			// have to reset everything it read to much, which is everything *but* the bytes
			// we read into the decoded object
			err = buffered.ReturnAndSkip(len(msg))
			if err != nil {
				break
			}

			// Save this object to return later
			objects = append(objects, msg)
		}
	}

	if err == io.EOF {
		err = nil
	}

	return
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
	s.bufBefore.UnreadRune()

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

	_, err = io.CopyN(ioutil.Discard, s, int64(offset))

	s.bufBefore = bytes.Buffer{}

	return
}
