package jsonextract

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
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
	buffered := resettableRuneBuffer{
		normalBuffer: bufio.NewReader(reader),
	}

	// decoder to decode our data
	// If singleStepper wasn't there, the decoder would load too much
	// data in its internal buffer, which would destroy the logic in the
	// loop, as in it would read further than the JSON object
	// stepper := &singleStepper{r: buffered}

	for {
		r, _, err := buffered.ReadRune()
		if err != nil {
			break
		}

		if r == openArray || r == openObject {

			err = buffered.UnreadRune()
			if err != nil {
				break
			}

			buffered.bufBefore.Reset()

			var msg json.RawMessage

			err = json.NewDecoder(&buffered).Decode(&msg)
			if err != nil {
				err = buffered.ReturnAndSkipOne()
				if err != nil {
					break
				}

				continue
			}

			err = buffered.ReturnAndSkip(len(msg))
			if err != nil {
				break
			}

			objects = append(objects, msg)
		}
	}

	if err == io.EOF {
		err = nil
	}

	return
}

type resettableRuneBuffer struct {
	normalBuffer *bufio.Reader

	bufBefore bytes.Buffer

	returnBuffer bytes.Buffer
}

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

func (s *resettableRuneBuffer) ReadRune() (r rune, size int, err error) {
	r, size, err = s.returnBuffer.ReadRune()
	if err != nil {
		r, size, err = s.normalBuffer.ReadRune()
	}

	s.bufBefore.WriteRune(r)

	return
}

func (s *resettableRuneBuffer) UnreadRune() (err error) {
	s.bufBefore.UnreadRune()

	err = s.returnBuffer.UnreadRune()
	if err == nil {
		return
	}

	return s.normalBuffer.UnreadRune()
}

func (s *resettableRuneBuffer) ReturnAndSkipOne() (err error) {
	s.returnBuffer = s.bufBefore

	_, _, err = s.returnBuffer.ReadRune()

	s.bufBefore = bytes.Buffer{}

	return
}

func (s *resettableRuneBuffer) ReturnAndSkip(offset int) (err error) {
	s.returnBuffer = s.bufBefore

	_, err = io.CopyN(ioutil.Discard, s, int64(offset))

	s.bufBefore = bytes.Buffer{}

	return
}
