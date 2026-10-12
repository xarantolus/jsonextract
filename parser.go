package jsonextract

import (
	"encoding/json"

	"github.com/tdewolff/parse/v2/js"
)

// frameState is the position inside of a JSON array or object
type frameState uint8

const (
	arrayOpen   frameState = iota // after '[': value or ']'
	arrayValue                    // after a value: ',' or ']'
	arrayComma                    // after ',': value
	objectOpen                    // after '{': key or '}'
	objectKey                     // after a key: ':'
	objectColon                   // after ':': value
	objectValue                   // after a value: ',' or '}'
	objectComma                   // after ',': key
)

// frame is an array or object that has been opened, but not yet closed
type frame struct {
	// start is the stream offset of the opening bracket
	start int64
	// outStart is the index of the opening bracket in parser.out
	outStart int
	state    frameState
}

// acceptValue moves the frame to the state after a value, if a value is allowed in the current state
func (f *frame) acceptValue() bool {
	switch f.state {
	case arrayOpen, arrayComma:
		f.state = arrayValue
	case objectColon:
		f.state = objectValue
	default:
		return false
	}
	return true
}

func (f *frame) acceptComma() bool {
	switch f.state {
	case arrayValue:
		f.state = arrayComma
	case objectValue:
		f.state = objectComma
	default:
		return false
	}
	return true
}

func (f *frame) acceptColon() bool {
	if f.state != objectKey {
		return false
	}
	f.state = objectColon
	return true
}

func (f *frame) canClose(tt js.TokenType) bool {
	switch f.state {
	case arrayOpen, arrayValue:
		return tt == js.CloseBracketToken
	case objectOpen, objectValue:
		return tt == js.CloseBraceToken
	}
	return false
}

// result is the outcome of parsing the JS object or array starting at a certain stream offset
type result struct {
	start int64
	// end is the stream offset after the closing bracket
	end int64
	// out is the converted JSON, nil if the object could not be converted
	out []byte
}

func (r result) ok() bool { return r.out != nil }

// parser converts the JavaScript object starting at an opening bracket to JSON.
//
// It does not just compute the result for that object: all objects and arrays nested inside of it share
// the same tokens, so the parser computes their results in the same pass. Every frame on the
// stack is a candidate object. If a token is invalid, all candidates containing it fail.
// Nested candidates that were closed before are kept in nested, as they might be needed
// if their parent fails.
//
// This allows extracting objects in linear time, instead of starting from scratch at every bracket.
type parser struct {
	tok  tokenizer
	conv converter

	frames []frame

	// out contains the converted JSON of all frames. Bytes of closed frames are never modified
	// again, which allows results to reference them
	out []byte

	// runStart is the index in out after the last structural token ({}[],:). The bytes after it
	// must form one scalar JSON value (or key), e.g. a sign, digits, '.' and more digits from separate tokens
	runStart int

	// pendingComma is true if the last structural token was a comma. It is applied to the grammar
	// with the next token, as a closing bracket directly after it removes it from the output
	pendingComma bool

	// nested contains the results of all candidates inside of the root candidate
	nested []result
}

// parse returns the result for the candidate object starting at the opening bracket at stream offset
// start. Results for all candidates nested inside of it are stored in p.nested.
//
// An error is only returned if the underlying reader returns one.
func (p *parser) parse(w *window, start int64) (root result, err error) {
	// The window must not be changed while the lexer is using it
	defer p.tok.release()

	p.tok.reset(w, start)
	p.conv = converter{}
	p.frames = p.frames[:0]
	p.pendingComma = false
	p.nested = p.nested[:0]
	// Results reference the output, so it cannot be reused
	p.out = nil

	var failed = result{start: start}

	for {
		tt, text, tokStart, tokEnd, err := p.tok.token()
		if err != nil {
			return result{}, err
		}
		if tt == js.ErrorToken {
			// End of input or invalid JavaScript
			p.failAll()
			return failed, nil
		}
		if isIgnoredToken(tt) {
			continue
		}

		var (
			before  = len(p.out)
			dropped bool
		)
		p.out, dropped, err = p.conv.convert(p.out, tt, text)
		if dropped {
			before--
		}
		if err != nil {
			p.failAll()
			return failed, nil
		}

		if len(p.frames) == 0 {
			// First token, which is the opening bracket at start
			p.push(tt, tokStart, before)
			p.runStart = len(p.out)
			continue
		}

		if dropped && p.pendingComma && (tt == js.CloseBraceToken || tt == js.CloseBracketToken) {
			// A trailing comma was removed from the output
			p.pendingComma = false
			p.runStart--
		}

		res, closed, ok := p.feed(tt, tokStart, tokEnd)
		if !ok {
			p.failAll()
			return failed, nil
		}

		if closed && len(p.frames) == 0 {
			return res, nil
		}
	}
}

// failAll marks all open frames as failed
func (p *parser) failAll() {
	for _, f := range p.frames {
		p.nested = append(p.nested, result{start: f.start})
	}
	p.frames = p.frames[:0]
}

func (p *parser) push(tt js.TokenType, start int64, outStart int) {
	var state = arrayOpen
	if tt == js.OpenBraceToken {
		state = objectOpen
	}
	p.frames = append(p.frames, frame{start: start, outStart: outStart, state: state})
}

// feed applies a token, which was already written to p.out, to the grammar of the innermost frame.
// ok is false if the token is not valid at this position.
// If the token closes a frame, closed is true and res is the result for that frame.
func (p *parser) feed(tt js.TokenType, start, end int64) (res result, closed, ok bool) {
	var top = &p.frames[len(p.frames)-1]

	if p.pendingComma {
		// The comma was not removed, so it is part of the output
		p.pendingComma = false
		if !top.acceptComma() {
			return
		}
	}

	switch tt {
	case js.OpenBraceToken, js.OpenBracketToken, js.CloseBraceToken, js.CloseBracketToken, js.CommaToken, js.ColonToken:
	default:
		// Not a structural token, so it is part of a scalar value. It is checked once the value is complete
		return result{}, false, true
	}

	// A scalar value before this token is complete
	var tokenOut = len(p.out) - 1
	if tokenOut > p.runStart && !p.acceptScalar(top, p.out[p.runStart:tokenOut]) {
		return
	}
	p.runStart = len(p.out)

	switch tt {
	case js.OpenBraceToken, js.OpenBracketToken:
		// If the new frame is not valid here, it must not be pushed: an object starting at this bracket
		// might still be valid, so it has to be parsed on its own later
		if ok = top.acceptValue(); ok {
			p.push(tt, start, tokenOut)
		}
	case js.CloseBraceToken, js.CloseBracketToken:
		if !top.canClose(tt) {
			return
		}
		p.frames = p.frames[:len(p.frames)-1]
		res = result{start: top.start, end: end, out: p.out[top.outStart:len(p.out):len(p.out)]}
		if len(p.frames) > 0 {
			p.nested = append(p.nested, res)
		}
		return res, true, true
	case js.CommaToken:
		p.pendingComma = true
		ok = true
	case js.ColonToken:
		ok = top.acceptColon()
	}
	return
}

// acceptScalar checks whether value is a valid scalar JSON value at the current position
func (p *parser) acceptScalar(top *frame, value []byte) bool {
	if !isSimpleScalar(value) && !json.Valid(value) {
		return false
	}
	if top.state == objectOpen || top.state == objectComma {
		// Object keys must be strings
		top.state = objectKey
		return value[0] == '"'
	}
	return top.acceptValue()
}

// isSimpleScalar returns whether value is a common JSON value that is obviously valid.
// It is a lot faster than json.Valid for these values
func isSimpleScalar(value []byte) bool {
	switch value[0] {
	case '"':
		if len(value) < 2 || value[len(value)-1] != '"' {
			return false
		}
		for _, c := range value[1 : len(value)-1] {
			if c == '"' || c == '\\' || c < 0x20 {
				return false
			}
		}
		return true
	case 't', 'f', 'n':
		var s = string(value)
		return s == "true" || s == "false" || s == "null"
	}
	return isDecimalInteger(value)
}
