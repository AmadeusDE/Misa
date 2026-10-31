package awk

import (
	"strconv"
	"strings"
)

type tokKind uint8

const (
	tEOF tokKind = iota
	tNewline
	tLbrace
	tRbrace
	tLparen
	tRparen
	tLbracket
	tRbracket
	tSemi
	tComma
	tPlus
	tMinus
	tStar
	tSlash
	tPercent
	tCaret
	tNot
	tGT
	tLT
	tPipe
	tQuestion
	tColon
	tTilde
	tNoMatch
	tDollar
	tAssign
	tAddAssign
	tSubAssign
	tMulAssign
	tDivAssign
	tModAssign
	tPowAssign
	tEq
	tNe
	tLe
	tGe
	tAnd
	tOr
	tIncr
	tDecr
	tAppend
	tNumber
	tString
	tRegex
	tName
	tFuncName
	tBuiltin
	tKeyword
)

type token struct {
	kind tokKind
	text string
	num  float64
	line int
}

var keywords = map[string]bool{
	"BEGIN": true, "END": true, "if": true, "else": true, "while": true, "for": true,
	"do": true, "break": true, "continue": true, "next": true, "nextfile": true,
	"exit": true, "return": true, "delete": true, "in": true, "getline": true,
	"function": true, "func": true, "print": true, "printf": true,
}

// builtinFuncs are the functions that may be called; unsupportedFuncs are
// recognised so they can be rejected by name.
var builtinFuncs = map[string]bool{
	"length": true, "substr": true, "index": true, "sprintf": true, "tolower": true,
	"toupper": true, "int": true, "sqrt": true, "exp": true, "log": true, "sin": true,
	"cos": true, "atan2": true, "rand": true, "srand": true, "match": true,
	"sub": true, "gsub": true,
	"split": true, "close": true, "system": true, "fflush": true,
}

type lexer struct {
	src  string
	pos  int
	line int
	prev tokKind
	toks []token
}

func lex(src string) ([]token, error) {
	lx := &lexer{src: src, line: 1}
	for {
		t, err := lx.next()
		if err != nil {
			return nil, err
		}
		lx.toks = append(lx.toks, t)
		lx.prev = t.kind
		if t.kind == tEOF {
			return lx.toks, nil
		}
	}
}

func (lx *lexer) errorf(msg string) error { return &Error{Line: lx.line, Msg: msg} }

// afterOperand reports whether the previous token ends an operand, so that a
// following slash divides instead of starting a regular expression.
func (lx *lexer) afterOperand() bool {
	switch lx.prev {
	case tName, tNumber, tString, tRparen, tRbracket, tBuiltin, tIncr, tDecr:
		return true
	}
	return false
}

func isLetter(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func (lx *lexer) skipSpace() {
	for lx.pos < len(lx.src) {
		c := lx.src[lx.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			lx.pos++
		case c == '\\' && strings.HasPrefix(lx.src[lx.pos:], "\\\n"):
			lx.pos += 2
			lx.line++
		case c == '\\' && strings.HasPrefix(lx.src[lx.pos:], "\\\r\n"):
			lx.pos += 3
			lx.line++
		case c == '#':
			for lx.pos < len(lx.src) && lx.src[lx.pos] != '\n' {
				lx.pos++
			}
		default:
			return
		}
	}
}

func (lx *lexer) next() (token, error) {
	lx.skipSpace()
	tok := token{line: lx.line}
	if lx.pos >= len(lx.src) {
		tok.kind = tEOF
		return tok, nil
	}
	c := lx.src[lx.pos]
	switch {
	case c == '\n':
		lx.pos++
		lx.line++
		tok.kind = tNewline
		return tok, nil
	case isLetter(c):
		return lx.word(tok), nil
	case isDigit(c) || (c == '.' && lx.pos+1 < len(lx.src) && isDigit(lx.src[lx.pos+1])):
		return lx.number(tok), nil
	case c == '"':
		return lx.stringLit(tok)
	case c == '/' && !lx.afterOperand():
		return lx.regexLit(tok)
	}
	return lx.operator(tok)
}

func (lx *lexer) word(tok token) token {
	start := lx.pos
	for lx.pos < len(lx.src) && (isLetter(lx.src[lx.pos]) || isDigit(lx.src[lx.pos])) {
		lx.pos++
	}
	tok.text = lx.src[start:lx.pos]
	switch {
	case keywords[tok.text]:
		tok.kind = tKeyword
	case builtinFuncs[tok.text]:
		tok.kind = tBuiltin
	case lx.pos < len(lx.src) && lx.src[lx.pos] == '(':
		tok.kind = tFuncName
	default:
		tok.kind = tName
	}
	return tok
}

func (lx *lexer) number(tok token) token {
	start := lx.pos
	end := scanNumber(lx.src[start:])
	lx.pos += end
	tok.kind = tNumber
	tok.text = lx.src[start:lx.pos]
	tok.num = parseDecimal(tok.text)
	return tok
}

var simpleEscapes = map[byte]byte{
	'"': '"', '\\': '\\', '/': '/', 'n': '\n', 't': '\t', 'r': '\r',
	'a': 7, 'b': 8, 'f': 12, 'v': 11,
}

func (lx *lexer) stringLit(tok token) (token, error) {
	lx.pos++
	var b strings.Builder
	for {
		if lx.pos >= len(lx.src) || lx.src[lx.pos] == '\n' {
			return tok, lx.errorf("non-terminated string")
		}
		c := lx.src[lx.pos]
		lx.pos++
		switch {
		case c == '"':
			tok.kind = tString
			tok.text = b.String()
			return tok, nil
		case c != '\\':
			b.WriteByte(c)
		case lx.pos >= len(lx.src):
			b.WriteByte('\\')
		default:
			lx.escape(&b)
		}
	}
}

// decodeEscape writes the escape starting at s[i], the character after a
// backslash, and returns the index after it. Unknown escapes keep the
// backslash so "\." still reaches a dynamic regex as \.
func decodeEscape(s string, i int, b *strings.Builder) int {
	c := s[i]
	if r, ok := simpleEscapes[c]; ok {
		b.WriteByte(r)
		return i + 1
	}
	if c >= '0' && c <= '7' {
		v, n := 0, 0
		for n < 3 && i < len(s) && s[i] >= '0' && s[i] <= '7' {
			v = v*8 + int(s[i]-'0')
			i++
			n++
		}
		b.WriteByte(byte(v))
		return i
	}
	b.WriteByte('\\')
	b.WriteByte(c)
	return i + 1
}

func (lx *lexer) escape(b *strings.Builder) {
	if lx.src[lx.pos] == '\n' {
		lx.pos++
		lx.line++
		return
	}
	lx.pos = decodeEscape(lx.src, lx.pos, b)
}

func (lx *lexer) regexLit(tok token) (token, error) {
	lx.pos++
	var b strings.Builder
	inBracket := false
	for {
		if lx.pos >= len(lx.src) || lx.src[lx.pos] == '\n' {
			return tok, lx.errorf("non-terminated regular expression")
		}
		c := lx.src[lx.pos]
		lx.pos++
		switch {
		case c == '/' && !inBracket:
			tok.kind = tRegex
			tok.text = b.String()
			return tok, nil
		case c == '\\' && lx.pos < len(lx.src) && lx.src[lx.pos] == '/':
			b.WriteByte('/')
			lx.pos++
		case c == '\\' && lx.pos < len(lx.src) && lx.src[lx.pos] != '\n':
			b.WriteByte('\\')
			b.WriteByte(lx.src[lx.pos])
			lx.pos++
		case c == '[' && !inBracket:
			inBracket = true
			b.WriteByte(c)
			lx.copyBracketStart(&b)
		case c == ']' && inBracket:
			inBracket = false
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
}

// copyBracketStart keeps a leading ^ and ] literally so they do not end
// the bracket expression early.
func (lx *lexer) copyBracketStart(b *strings.Builder) {
	if lx.pos < len(lx.src) && lx.src[lx.pos] == '^' {
		b.WriteByte('^')
		lx.pos++
	}
	if lx.pos < len(lx.src) && lx.src[lx.pos] == ']' {
		b.WriteByte(']')
		lx.pos++
	}
}

var operators = []struct {
	text string
	kind tokKind
}{
	{"**=", tPowAssign}, {"&&", tAnd}, {"||", tOr}, {"==", tEq}, {"!=", tNe}, {"<=", tLe},
	{">=", tGe}, {"++", tIncr}, {"--", tDecr}, {"+=", tAddAssign}, {"-=", tSubAssign},
	{"*=", tMulAssign}, {"/=", tDivAssign}, {"%=", tModAssign}, {"^=", tPowAssign},
	{"!~", tNoMatch}, {">>", tAppend}, {"**", tCaret},
	{"{", tLbrace}, {"}", tRbrace}, {"(", tLparen}, {")", tRparen}, {"[", tLbracket},
	{"]", tRbracket}, {";", tSemi}, {",", tComma}, {"+", tPlus}, {"-", tMinus},
	{"*", tStar}, {"/", tSlash}, {"%", tPercent}, {"^", tCaret}, {"!", tNot},
	{">", tGT}, {"<", tLT}, {"|", tPipe}, {"?", tQuestion}, {":", tColon},
	{"~", tTilde}, {"$", tDollar}, {"=", tAssign},
}

func (lx *lexer) operator(tok token) (token, error) {
	rest := lx.src[lx.pos:]
	for _, op := range operators {
		if strings.HasPrefix(rest, op.text) {
			lx.pos += len(op.text)
			tok.kind = op.kind
			tok.text = op.text
			return tok, nil
		}
	}
	return tok, lx.errorf("unexpected character " + strconv.QuoteRune(rune(rest[0])))
}
