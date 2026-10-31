package builtins

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// xargsToken records the nonempty logical input line on which an argument starts.
// Lines ending in a blank (or an escaped newline) continue on the next line.
type xargsToken struct {
	text string
	line int
}

// xargsTokenizer returns one argument at a time, retaining only the current
// argument and a fixed-size read buffer. A single argument may itself be large.
// Context cancellation is checked between reads; an arbitrary blocked Reader
// cannot itself be interrupted by a context.
type xargsTokenizer struct {
	ctx         context.Context
	reader      *bufio.Reader
	delim       *byte
	eof         string
	replace     bool
	line, start int
	lineHasWord bool
	previous    byte
	done        bool
}

func newXargsTokenizer(ctx context.Context, r io.Reader, delim *byte, eof string, replace bool) *xargsTokenizer {
	return &xargsTokenizer{ctx: ctx, reader: bufio.NewReader(r), delim: delim, eof: eof, replace: replace, line: 1}
}

// Next returns the next input argument, or io.EOF after the input or logical
// EOF marker. Errors are terminal. In delimiter mode, records are literal and
// every delimiter ends a record (including an empty one).
func (t *xargsTokenizer) Next() (xargsToken, error) {
	if t.done {
		return xargsToken{}, io.EOF
	}
	if t.replace && t.delim == nil {
		return t.nextReplacement()
	}
	var word strings.Builder
	active := false
	var quote byte
	escaped := false
	finish := func() (xargsToken, error) {
		text := word.String()
		if t.delim == nil && t.eof != "" && text == t.eof {
			t.done = true
			return xargsToken{}, io.EOF
		}
		return xargsToken{text: text, line: t.start}, nil
	}
	for {
		if err := t.ctx.Err(); err != nil {
			t.done = true
			return xargsToken{}, err
		}
		c, err := t.reader.ReadByte()
		if err != nil {
			t.done = true
			if err != io.EOF {
				return xargsToken{}, err
			}
			if quote != 0 {
				return xargsToken{}, fmt.Errorf("unmatched %c quote", quote)
			}
			if active {
				return finish()
			}
			return xargsToken{}, io.EOF
		}
		if t.delim != nil {
			if !active {
				t.start = t.line
				active = true
			}
			if c == *t.delim {
				t.line++
				return finish()
			}
			word.WriteByte(c)
			continue
		}
		wasEscaped := escaped
		finished := false
		if escaped {
			word.WriteByte(c)
			escaped = false
		} else if quote != 0 {
			if c == '\n' {
				t.done = true
				return xargsToken{}, fmt.Errorf("unmatched %c quote", quote)
			}
			if c == quote {
				quote = 0
			} else {
				word.WriteByte(c)
			}
		} else if c == ' ' || c == '\t' || c == '\n' {
			finished = active
		} else {
			if !active {
				t.start = t.line
				active = true
				t.lineHasWord = true
			}
			switch c {
			case '\'', '"':
				quote = c
			case '\\':
				escaped = true
			default:
				word.WriteByte(c)
			}
		}
		if c == '\n' && !wasEscaped && t.previous != ' ' && t.previous != '\t' && t.lineHasWord {
			t.line++
			t.lineHasWord = false
		}
		t.previous = c
		if finished {
			return finish()
		}
	}
}

// nextReplacement parses one logical replacement record. Unlike ordinary
// tokenization, interior and trailing blanks are preserved and newline ends a
// record even when preceded by blanks.
func (t *xargsTokenizer) nextReplacement() (xargsToken, error) {
	var word strings.Builder
	active := false
	var quote byte
	escaped := false
	finish := func() (xargsToken, error) {
		text := word.String()
		if t.eof != "" && text == t.eof {
			t.done = true
			return xargsToken{}, io.EOF
		}
		return xargsToken{text: text, line: t.start}, nil
	}
	for {
		if err := t.ctx.Err(); err != nil {
			t.done = true
			return xargsToken{}, err
		}
		c, err := t.reader.ReadByte()
		if err != nil {
			t.done = true
			if err != io.EOF {
				return xargsToken{}, err
			}
			if quote != 0 {
				return xargsToken{}, fmt.Errorf("unmatched %c quote", quote)
			}
			if active {
				return finish()
			}
			return xargsToken{}, io.EOF
		}
		if escaped {
			word.WriteByte(c)
			escaped = false
		} else if quote != 0 {
			if c == '\n' {
				t.done = true
				return xargsToken{}, fmt.Errorf("unmatched %c quote", quote)
			}
			if c == quote {
				quote = 0
			} else {
				word.WriteByte(c)
			}
		} else if c == '\n' {
			if active {
				t.line++
				return finish()
			}
			continue
		} else if !active && (c == ' ' || c == '\t') {
			continue
		} else {
			if !active {
				t.start = t.line
				active = true
			}
			switch c {
			case '\'', '"':
				quote = c
			case '\\':
				escaped = true
			default:
				word.WriteByte(c)
			}
		}
	}
}

// tokenizeXargs is a collecting adapter for callers that need all tokens.
// Command execution should use Next directly to avoid buffering all input.
func tokenizeXargs(ctx context.Context, r io.Reader, delim *byte, eof string) ([]xargsToken, error) {
	t := newXargsTokenizer(ctx, r, delim, eof, false)
	var tokens []xargsToken
	for {
		tok, err := t.Next()
		if err == io.EOF {
			return tokens, nil
		}
		if err != nil {
			return tokens, err
		}
		tokens = append(tokens, tok)
	}
}
