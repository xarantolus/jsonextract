package jsonextract

import (
	"unicode/utf8"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
)

const (
	// lookaheadMargin is the number of bytes the lexer might look at after the end of a token
	// to decide where the token ends. Tokens that end closer than this to the end of the lexer
	// input are lexed again with more input.
	lookaheadMargin = 16

	// minSpan is the minimum number of bytes the lexer gets as input. Every lexer error copies and scans
	// the entire input (to compute a line number we don't need), so it should not be too large
	minSpan = 1024
)

// tokenizer lexes JavaScript tokens from a window, with the input data arriving incrementally.
//
// The js.Lexer needs all of its input up front, so it only gets a span of the data. Whenever a token
// might continue after the end of the span, the tokenizer restarts the lexer at that token with a new span,
// reading more data if necessary. The restarted lexer is put into the same state as the previous one,
// so this is invisible to the caller.
type tokenizer struct {
	w *window

	// lex reads from in. Index 0 of in is at stream offset inBase, and in ends at inEnd.
	// lex is nil if it must be (re-)created before the next token
	lex           *js.Lexer
	in            *parse.Input
	inBase, inEnd int64

	// span is the number of bytes the lexer gets as input. It grows for long tokens
	span int

	// next is the stream offset of the next token, prev that of the last token
	next, prev int64

	// These mirror the internal state of js.Lexer after the token at prev, so that we can
	// recreate an equivalent lexer at next
	prevLineTerminator bool
	prevNumeric        bool

	// scratch is reused to restart the lexer with a prefix
	scratch []byte
}

// reset makes the tokenizer start at the given stream offset, just like a new js.Lexer would
func (t *tokenizer) reset(w *window, at int64) {
	t.release()
	t.w = w
	t.span = minSpan
	t.next, t.prev = at, at
	t.prevLineTerminator, t.prevNumeric = true, false
}

// release stops using the current lexer. It must be called before the window is modified
func (t *tokenizer) release() {
	if t.in != nil {
		// parse.NewInputBytes might have overwritten the byte after the input with a NULL byte
		t.in.Restore()
		t.in, t.lex = nil, nil
	}
}

// start creates a new lexer for data, which starts at stream offset base.
// The first skip tokens are skipped, they were already returned before.
func (t *tokenizer) start(data []byte, base int64, skip int) {
	if len(data) > t.span {
		data = data[:t.span]
	}
	t.in = parse.NewInputBytes(data)
	t.lex = js.NewLexer(t.in)
	t.inBase = base
	t.inEnd = base + int64(len(data))
	for i := 0; i < skip; i++ {
		t.lex.Next()
	}
}

// restart creates a lexer for the data at next that is in the same state as a lexer
// that has read all tokens before next.
func (t *tokenizer) restart() {
	switch {
	case t.prevNumeric:
		// The lexer needs to know that the last token was a number (an identifier directly
		// after a number is an error). Lexing a number does not depend on any state, so we
		// just start one token earlier
		t.start(t.w.bytes(t.prev), t.prev, 1)
	case !t.prevLineTerminator:
		// The lexer must know we're not at the start of a line, otherwise "-->" would be lexed
		// as a comment. A semicolon puts the lexer into exactly that state
		var data = t.w.bytes(t.next)
		if len(data) >= t.span {
			data = data[:t.span-1]
		}
		// The extra byte of capacity avoids a copy in parse.NewInputBytes, which appends a NULL byte
		t.scratch = append(append(t.scratch[:0], ';'), data...)
		t.scratch = append(t.scratch, 0)[:len(t.scratch)]
		t.start(t.scratch, t.next-1, 1)
	default:
		// Same state as a new lexer
		t.start(t.w.bytes(t.next), t.next, 0)
	}
}

// token returns the next token and its stream offsets. Regular expressions are returned as
// js.RegExpToken, never as js.DivToken. Returned text is only valid until the next call.
//
// At the end of the input or on lexer errors, js.ErrorToken is returned.
// A non-nil error is only returned if the underlying reader returned one.
func (t *tokenizer) token() (tt js.TokenType, text []byte, start, end int64, err error) {
	for {
		if t.lex == nil {
			t.restart()
		}

		tt, text = t.lex.Next()
		if tt == js.DivToken || tt == js.DivEqToken {
			// We don't know the context, so we just assume a '/' starts a regex
			tt, text = t.lex.RegExp()
		}
		end = t.inBase + int64(t.in.Offset())

		if t.complete(tt, end) {
			break
		}

		// The token might continue after the input of the lexer. Lex it again with more input
		t.release()
		if t.inEnd == t.w.end() {
			// The lexer had all data, so we need more
			if !t.w.fill() && t.w.err != nil {
				return js.ErrorToken, nil, 0, 0, t.w.err
			}
		} else {
			// A token is longer than the span. Doubling it avoids lexing long tokens too often
			t.span *= 2
		}
	}

	if tt != js.WhitespaceToken {
		t.prevLineTerminator = tt == js.LineTerminatorToken || tt == js.CommentLineTerminatorToken
	}
	t.prevNumeric = js.IsNumeric(tt)

	start = t.next
	t.prev, t.next = t.next, end

	return tt, text, start, end, nil
}

// complete returns whether the token ending at the given stream offset is complete,
// which means that the lexer would return the same token if it had more input
func (t *tokenizer) complete(tt js.TokenType, end int64) bool {
	switch {
	case !needsLookahead(tt) || end+lookaheadMargin <= t.inEnd:
		return true
	case t.w.eof && t.inEnd == t.w.end():
		// The lexer had all input there is
		return true
	case tt != js.ErrorToken && end < t.inEnd:
		// The lexer never looks past a delimiter that follows a token.
		// This allows us to process objects at the end of the data that was read so far,
		// instead of waiting for more data
		var next = t.w.bytes(end)[0]
		switch tt {
		case js.WhitespaceToken:
			// Whitespace is only continued by more whitespace, which might be a multi-byte character
			return next < utf8.RuneSelf && next != ' ' && next != '\t' && next != '\v' && next != '\f'
		case js.LineTerminatorToken:
			return next < utf8.RuneSelf && next != '\n' && next != '\r'
		}
		return isDelimiter(next)
	}
	return false
}

// isDelimiter returns whether c ends any token it follows
func isDelimiter(c byte) bool {
	switch c {
	case '{', '}', '[', ']', '(', ')', ',', ';', ':', '\'', '"', '`', ' ', '\t', '\n', '\r':
		return true
	}
	return false
}

// needsLookahead returns whether the lexer might look at bytes after the end
// of the token to decide that it ends there
func needsLookahead(tt js.TokenType) bool {
	switch tt {
	case js.OpenBraceToken, js.CloseBraceToken, js.OpenBracketToken, js.CloseBracketToken,
		js.OpenParenToken, js.CloseParenToken, js.CommaToken, js.ColonToken, js.SemicolonToken,
		js.StringToken, js.TemplateToken:
		return false
	}
	return true
}
