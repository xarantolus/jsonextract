package jsonextract

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	openObject = '{'
	openArray  = '['
)

// StringObjects takes the given String and extracts all valid JSON objects / Arrays it can find
func StringObjects(data string) (extracted []json.RawMessage) {
	// Reader only returns errors if reading failed, which is not possible here
	extracted, _ = ReaderObjects(strings.NewReader(data))
	return
}

func extract(text string, start int) json.RawMessage {
	msg := json.RawMessage{}

	err := json.Unmarshal([]byte(text[start:]), &msg)

	if err == nil {
		return msg
	}

	if syerr, ok := err.(*json.SyntaxError); ok {
		fmt.Println(syerr.Offset)
	}

	return nil
}

// ReaderObjects takes the given io.Reader and reads all possible JSON objects it can find in it
// Assumes the stream to consist of utf8 bytes
func ReaderObjects(reader io.Reader) (objects []json.RawMessage, err error) {
	// Need to buffer in order to be able to unread
	buffered := prependableBuffer{
		normalBuffer: bufio.NewReader(reader),
	}

	// decoder to decode our data
	// If singleStepper wasn't there, the decoder would load too much
	// data in its internal buffer, which would destroy the logic in the
	// loop, as in it would read further than the JSON object
	stepper := &singleStepper{r: buffered}
	dec := json.NewDecoder(stepper)

	for {
		// stepper.Throwaway()

		// Read every rune in our buffered stream
		r, _, err := buffered.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// We are at the end of the reader
				return objects, nil
			}
			return nil, err
		}

		// If we find any opening, we check if it continues with valid JSON data
		if r == openObject || r == openArray {
			// Before decoding, we step back one step to
			// make sure the decoder will get the opening brace
			err = buffered.UnreadRune() // TODO: Does this unread correctly?
			if err != nil {
				return nil, err
			}

			// we decode to json.RawMessage
			var msg = json.RawMessage{}

			err = dec.Decode(&msg)
			if err != nil {
				var prepend = stepper.Data(1)

				buffered.Prepend(prepend)
				continue
			}

			objects = append(objects, json.RawMessage(msg))
			stepper.Throwaway()
		}
	}
}

type prependableBuffer struct {
	bufBefore bytes.Buffer

	normalBuffer *bufio.Reader
}

func (s *prependableBuffer) Read(p []byte) (n int, err error) {
	if s.bufBefore.Len() != 0 {
		n, err = s.bufBefore.Read(p)
		if err == io.EOF {
			err = nil
		}
	}

	n2, err2 := s.normalBuffer.Read(p[n:])

	return n + n2, err2
}

func (b *prependableBuffer) ReadRune() (r rune, size int, err error) {
	r, size, err = b.bufBefore.ReadRune()
	if err == nil {
		return
	}
	return b.normalBuffer.ReadRune()
}

func (b *prependableBuffer) UnreadRune() (err error) {
	err = b.bufBefore.UnreadRune()
	if err == nil {
		return
	}

	return b.normalBuffer.UnreadRune()
}

func (s *prependableBuffer) Prepend(p []byte) {
	if s.bufBefore.Len() == 0 {
		// cannot fail
		_, _ = s.bufBefore.Write(p)
		return
	}

	data := append(p, s.bufBefore.Bytes()...)
	s.bufBefore.Reset()

	s.bufBefore.Write(data)
	return
}

type singleStepper struct {
	r prependableBuffer

	buf bytes.Buffer
}

func (s *singleStepper) Read(p []byte) (n int, err error) {
	next, size, err := s.r.normalBuffer.ReadRune()
	if err != nil {
		return
	}

	if len(p) < size {
		return 0, nil
	}

	written := utf8.EncodeRune(p, next)

	// keep the rune
	s.buf.Write(p[:written])

	return written, nil
}

func (s *singleStepper) Data(offset int) (bytes []byte) {
	len := s.buf.Len()
	if len < offset {
		return
	}

	bytes = make([]byte, len-offset)
	copy(bytes, s.buf.Bytes()[offset:])

	s.Throwaway()

	return
}

func (s *singleStepper) Throwaway() {
	s.buf.Reset()
}
