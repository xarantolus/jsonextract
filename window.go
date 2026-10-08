package jsonextract

import (
	"io"
)

const (
	// initialSize is the initial size of the window buffer, which grows when necessary
	initialSize = 4096
	// minReadSize is the minimum free space for a read
	minReadSize = 512
)

// window holds the part of a stream that might still be needed. Unlike bufio.Reader, it can grow
// to any size and data is addressed by stream offset, so it can be read again (e.g. after a candidate failed).
//
// The window contains the bytes from stream offset base (inclusive) to end() (exclusive).
// fill might move the data, so slices returned by bytes must not be used after calling it.
type window struct {
	r io.Reader

	// buf contains the bytes of the stream starting at offset base.
	// cap(buf) > len(buf) is always true after the first fill: we keep spare capacity so that
	// parse.NewInputBytes can add its NULL terminator without copying the buffer
	buf  []byte
	base int64

	// eof is true if the reader returned io.EOF, which means that no more data will be added
	eof bool
	// err is the first error the reader returned, except io.EOF
	err error
}

func newWindow(r io.Reader) *window {
	return &window{r: r}
}

// end returns the stream offset after the last byte in the window
func (w *window) end() int64 {
	return w.base + int64(len(w.buf))
}

// bytes returns all available bytes from the given stream offset, which must be in the window
func (w *window) bytes(from int64) []byte {
	return w.buf[from-w.base:]
}

// discard allows the window to forget all data before the given stream offset, which must not be after end()
func (w *window) discard(before int64) {
	if before > w.base {
		w.buf = w.buf[before-w.base:]
		w.base = before
	}
}

// fill reads more data into the window. It only blocks until the reader returns some data.
// It returns false if no more data is available because the reader returned an error or io.EOF.
func (w *window) fill() bool {
	if w.eof || w.err != nil {
		return false
	}

	if cap(w.buf)-len(w.buf) <= minReadSize {
		// Move the data to a new buffer, which also frees discarded data
		var newBuf = make([]byte, len(w.buf), 2*len(w.buf)+initialSize)
		copy(newBuf, w.buf)
		w.buf = newBuf
	}

	n, err := io.ReadAtLeast(w.r, w.buf[len(w.buf):cap(w.buf)-1], 1)
	w.buf = w.buf[:len(w.buf)+n]

	if err == io.EOF {
		w.eof = true
	} else if err != nil {
		w.err = err
	}

	return n > 0
}
