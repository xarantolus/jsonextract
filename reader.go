package jsonextract

import (
	"bytes"
	"errors"
	"io"
)

// ErrStop can be returned from a JSONCallback function to indicate that processing should stop.
// When used with Reader, it will stop processing.
// When used with Objects, the callback function will never be called again (e.g. after it received the required data).
var ErrStop = errors.New("stop processing json")

// JSONCallback is the callback function passed to Reader and ObjectOptions.
//
// Any JSON objects will be passed to it as bytes as defined by the function.
//
// If this function returns an error, processing will stop and return that error.
// You can return ErrStop to make sure the function will not be called again.
type JSONCallback func([]byte) error

// minMemoPrune is the minimum number of memoized results before results that are no longer needed are removed
const minMemoPrune = 1024

// Reader reads all JSON and JavaScript objects from the input and calls callback for each of them.
//
// The input is processed as a stream: callback is called as soon as an object has been read, and
// only data that might still be part of an object is kept in memory.
//
// Errors returned from the callback will stop the method.
// The error will be returned, except if it is ErrStop which will cause the method to return nil.
//
// Please note that the reader must return UTF-8 bytes for this to work correctly.
func Reader(reader io.Reader, callback JSONCallback) (err error) {
	var (
		w = newWindow(reader)
		p parser

		// memo contains the results for opening brackets that were already parsed as part of another
		// candidate. Without it, nested brackets would be parsed again and again
		memo      = make(map[int64]result)
		pruneSize = minMemoPrune

		// pos is the stream offset where we continue looking for objects
		pos int64
	)

	for {
		// Data and results before pos are no longer needed
		w.discard(pos)
		if len(memo) >= pruneSize {
			for off := range memo {
				if off < pos {
					delete(memo, off)
				}
			}
			pruneSize = 2 * len(memo)
			if pruneSize < minMemoPrune {
				pruneSize = minMemoPrune
			}
		}

		start, found := nextBracket(w, pos)
		if !found {
			break
		}

		res, known := memo[start]
		if !known {
			res, err = p.parse(w, start)
			if err != nil {
				return err
			}

			// If this candidate failed, the ones nested in it are next
			if !res.ok() {
				for _, r := range p.nested {
					memo[r.start] = r
				}
			}
		}

		if !res.ok() {
			// We continue directly after this opening bracket
			pos = start + 1
			continue
		}

		err = callback(res.out)
		if err != nil {
			// ErrStop just stops, returns nil
			if err == ErrStop {
				err = nil
			}
			return err
		}
		pos = res.end
	}

	return w.err
}

// nextBracket returns the stream offset of the next '{' or '[' at or after from
func nextBracket(w *window, from int64) (offset int64, found bool) {
	for {
		if from < w.end() {
			if i := bytes.IndexAny(w.bytes(from), "{["); i >= 0 {
				return from + int64(i), true
			}
			from = w.end()
		}

		w.discard(from)
		if !w.fill() {
			return 0, false
		}
	}
}
