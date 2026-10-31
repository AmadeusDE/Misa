package builtins

// Lexer for the bc language. Token set and accepted extensions follow Toybox
// toys/pending/bc.c (0BSD, commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb):
// `#` line comments, multi-character lowercase names, `abs()`, and strings
// that end at the next '"' without backslash escapes. `void` functions are a
// Misa extension (see docs/plans/bc.md).

import "fmt"

type bcTokenKind int

const (
	bcTokEOF bcTokenKind = iota
	bcTokNewline
	bcTokNumber
	bcTokName
	bcTokString
	bcTokKeyword
	bcTokOp
)

// bcKeywords are the reserved words. POSIX defines all but abs, limits, read,
// void and halt; see docs/plans/bc.md.
var bcKeywords = map[string]bool{
	"abs": true, "auto": true, "break": true, "continue": true, "define": true,
	"else": true, "for": true, "halt": true, "ibase": true, "if": true,
	"last": true, "length": true, "limits": true, "obase": true, "print": true,
	"quit": true, "read": true, "return": true, "scale": true, "sqrt": true,
	"while": true, "void": true,
}

type bcToken struct {
	kind bcTokenKind
	text string
	line int
}

func (t bcToken) String() string {
	switch t.kind {
	case bcTokEOF:
		return "end of input"
	case bcTokNewline:
		return "newline"
	case bcTokString:
		return fmt.Sprintf("%q", t.text)
	}
	return fmt.Sprintf("%q", t.text)
}

type bcLexer struct {
	src  string
	pos  int
	line int
}

func bcLex(src string) ([]bcToken, error) {
	l := &bcLexer{src: src, line: 1}
	var toks []bcToken
	for {
		tok, err := l.next()
		if err != nil {
			return nil, err
		}
		toks = append(toks, tok)
		if tok.kind == bcTokEOF {
			return toks, nil
		}
	}
}

func (l *bcLexer) errf(format string, args ...any) error {
	return &bcParseError{line: l.line, msg: fmt.Sprintf(format, args...)}
}

func (l *bcLexer) peekByte() byte {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *bcLexer) peekByteAt(off int) byte {
	if l.pos+off >= len(l.src) {
		return 0
	}
	return l.src[l.pos+off]
}

func bcIsDigit(c byte) bool { return c >= '0' && c <= '9' }
func bcIsUpper(c byte) bool { return c >= 'A' && c <= 'Z' }
func bcIsLower(c byte) bool { return c >= 'a' && c <= 'z' }

func (l *bcLexer) next() (bcToken, error) {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\v' || c == '\f':
			l.pos++
			continue
		case c == '\\' && l.peekByteAt(1) == '\n':
			// Backslash-newline is whitespace (POSIX line continuation).
			l.pos += 2
			l.line++
			continue
		case c == '/' && l.peekByteAt(1) == '*':
			l.pos += 2
			closed := false
			for l.pos < len(l.src) {
				if l.src[l.pos] == '*' && l.peekByteAt(1) == '/' {
					l.pos += 2
					closed = true
					break
				}
				if l.src[l.pos] == '\n' {
					l.line++
				}
				l.pos++
			}
			if !closed {
				return bcToken{}, l.errf("comment end could not be found")
			}
			continue
		case c == '#':
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
			continue
		}
		break
	}

	tok := bcToken{line: l.line}
	if l.pos >= len(l.src) {
		tok.kind = bcTokEOF
		return tok, nil
	}
	c := l.src[l.pos]

	switch {
	case c == '\n':
		l.pos++
		tok.kind = bcTokNewline
		tok.text = "\n"
		l.line++
		return tok, nil
	case c == '"':
		l.pos++
		start := l.pos
		for l.pos < len(l.src) && l.src[l.pos] != '"' {
			if l.src[l.pos] == '\n' {
				l.line++
			}
			l.pos++
		}
		if l.pos >= len(l.src) {
			// Toybox requires the closing quote on the same input unit.
			return bcToken{}, l.errf("string end could not be found")
		}
		if l.pos-start > bcMaxStringLen {
			return bcToken{}, l.errf("string too long")
		}
		tok.kind = bcTokString
		tok.text = l.src[start:l.pos]
		l.pos++
		return tok, nil
	case bcIsDigit(c) || bcIsUpper(c) || (c == '.' && (bcIsDigit(l.peekByteAt(1)) || bcIsUpper(l.peekByteAt(1)))):
		start := l.pos
		dot := false
		for l.pos < len(l.src) {
			ch := l.src[l.pos]
			if ch == '.' && !dot {
				dot = true
				l.pos++
				continue
			}
			// A number is digits, A-Z digits and one decimal point; a
			// lowercase letter starts a name instead.
			if bcIsDigit(ch) || bcIsUpper(ch) {
				l.pos++
				continue
			}
			break
		}
		tok.kind = bcTokNumber
		tok.text = l.src[start:l.pos]
		return tok, nil
	case c == '.':
		l.pos++
		tok.kind = bcTokOp
		tok.text = "."
		return tok, nil
	case bcIsLower(c):
		start := l.pos
		for l.pos < len(l.src) {
			ch := l.src[l.pos]
			if bcIsLower(ch) || bcIsDigit(ch) || ch == '_' {
				l.pos++
				continue
			}
			break
		}
		tok.text = l.src[start:l.pos]
		if bcKeywords[tok.text] {
			tok.kind = bcTokKeyword
		} else {
			tok.kind = bcTokName
		}
		return tok, nil
	}

	// Operators, longest match first.
	two := ""
	if l.pos+1 < len(l.src) {
		two = l.src[l.pos : l.pos+2]
	}
	switch two {
	case "++", "--", "+=", "-=", "*=", "/=", "%=", "^=", "==", "!=", "<=", ">=", "&&", "||":
		l.pos += 2
		tok.kind = bcTokOp
		tok.text = two
		return tok, nil
	}
	switch c {
	case '+', '-', '*', '/', '%', '^', '=', '<', '>', '!', '(', ')', '[', ']', '{', '}', ';', ',':
		l.pos++
		tok.kind = bcTokOp
		tok.text = string(c)
		return tok, nil
	}
	return bcToken{}, l.errf("bad character (%c)", c)
}
