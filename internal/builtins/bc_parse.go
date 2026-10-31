package builtins

// Recursive-descent parser for the bc language, producing an AST that
// bc_exec.go evaluates. Grammar and accepted extensions follow POSIX bc and
// Toybox toys/pending/bc.c (0BSD, commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb).

import (
	"errors"
	"fmt"
)

var (
	errBcIncomplete = errors.New("incomplete input")
	errBcQuit       = errors.New("quit")
)

// ---- Statements ----

type bcStmt interface{}

type bcExprStmt struct {
	x bcExpr
	// paren records that the expression started with "(", so a leading
	// assignment is still printed (Toybox keys this on the first token, so
	// `(x) = 2` prints 2).
	paren bool
}

type bcStringStmt struct{ s string }

type bcPrintStmt struct{ items []bcPrintItem }

type bcPrintItem struct {
	str   string
	isStr bool
	x     bcExpr
}

type bcIfStmt struct {
	cond bcExpr
	then []bcStmt
	els  []bcStmt
}

type bcWhileStmt struct {
	cond bcExpr
	body []bcStmt
}

type bcForStmt struct {
	init   bcExpr
	cond   bcExpr
	update bcExpr
	body   []bcStmt
}

type bcBreakStmt struct{}

type bcContinueStmt struct{}

type bcHaltStmt struct{}

type bcLimitsStmt struct{}

type bcReturnStmt struct {
	x    bcExpr
	hasX bool
}

type bcDefineStmt struct {
	name   string
	params []string
	autos  []string
	body   []bcStmt
	void   bool
}

// ---- Expressions ----

type bcExpr interface{}

type bcNumLit struct{ text string }

type bcVarRef struct{ name string }

type bcArrayElem struct {
	name  string
	index bcExpr
}

// bcSpecialRef is one of ibase, obase, scale.
type bcSpecialRef struct{ which string }

// bcLastRef is `last` or its `.` shortcut.
type bcLastRef struct{}

type bcCallExpr struct {
	name string
	args []bcExpr
}

// bcBuiltinExpr is length(x), scale(x), sqrt(x) or abs(x).
type bcBuiltinExpr struct {
	name string
	x    bcExpr
}

type bcUnaryExpr struct {
	op string // "-", "!", "++", "--"
	x  bcExpr
}

type bcPostfixExpr struct {
	op string // "++", "--"
	x  bcExpr
}

type bcBinaryExpr struct {
	op   string
	l, r bcExpr
}

type bcAssignExpr struct {
	op   string // "=", "+=", ...
	l, r bcExpr
}

// bcParenExpr is a parenthesized expression. It is a pure pass-through for
// evaluation, but ++ and -- reject it: Toybox's parser refuses an increment
// whose operand ends with ")".
type bcParenExpr struct{ x bcExpr }

// ---- Parser ----

type bcParser struct {
	toks      []bcToken
	pos       int
	loopDepth int
	funcDepth int
	voidFunc  bool
}

func bcParse(toks []bcToken) ([]bcStmt, error) {
	p := &bcParser{toks: toks}
	stmts, err := p.parseStmtList(false)
	if err != nil {
		return nil, err
	}
	if p.cur().kind != bcTokEOF {
		return nil, p.errf(p.cur(), "bad statement")
	}
	return stmts, nil
}

func (p *bcParser) cur() bcToken {
	if p.pos >= len(p.toks) {
		return bcToken{kind: bcTokEOF, line: p.lastLine()}
	}
	return p.toks[p.pos]
}

func (p *bcParser) lastLine() int {
	if len(p.toks) == 0 {
		return 1
	}
	return p.toks[len(p.toks)-1].line
}

func (p *bcParser) peek(n int) bcToken {
	if p.pos+n >= len(p.toks) {
		return bcToken{kind: bcTokEOF, line: p.lastLine()}
	}
	return p.toks[p.pos+n]
}

func (p *bcParser) advance() bcToken {
	t := p.cur()
	if p.pos < len(p.toks) {
		p.pos++
	}
	return t
}

func (p *bcParser) errf(t bcToken, format string, args ...any) error {
	return &bcParseError{line: t.line, msg: fmt.Sprintf(format, args...)}
}

type bcParseError struct {
	line int
	msg  string
}

func (e *bcParseError) Error() string { return e.msg }

func (p *bcParser) isOp(text string) bool {
	t := p.cur()
	return t.kind == bcTokOp && t.text == text
}

func (p *bcParser) isKeyword(text string) bool {
	t := p.cur()
	return t.kind == bcTokKeyword && t.text == text
}

func (p *bcParser) acceptOp(text string) bool {
	if p.isOp(text) {
		p.advance()
		return true
	}
	return false
}

func (p *bcParser) expectOp(text string) error {
	if p.cur().kind == bcTokEOF {
		return errBcIncomplete
	}
	if !p.isOp(text) {
		return p.errf(p.cur(), "bad expression: expected %q", text)
	}
	p.advance()
	return nil
}

func (p *bcParser) isSeparator() bool {
	t := p.cur()
	return t.kind == bcTokNewline || (t.kind == bcTokOp && t.text == ";")
}

// skipNewlines consumes newline tokens only (separators include ';').
func (p *bcParser) skipNewlines() {
	for p.cur().kind == bcTokNewline {
		p.advance()
	}
}

func (p *bcParser) skipSeparators() {
	for p.isSeparator() {
		p.advance()
	}
}

// parseStmtList parses statements until EOF or, when inBlock, a closing brace.
func (p *bcParser) parseStmtList(inBlock bool) ([]bcStmt, error) {
	var stmts []bcStmt
	for {
		p.skipSeparators()
		t := p.cur()
		if t.kind == bcTokEOF {
			if inBlock {
				return nil, errBcIncomplete
			}
			return stmts, nil
		}
		if t.kind == bcTokOp && t.text == "}" {
			if !inBlock {
				return nil, p.errf(t, "block end could not be found")
			}
			p.advance()
			return stmts, nil
		}
		st, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		if st != nil {
			stmts = append(stmts, st)
		}
		// A statement must end at a separator, a brace, or the end of input.
		if !p.isSeparator() && p.cur().kind != bcTokEOF {
			if !(p.cur().kind == bcTokOp && p.cur().text == "}") {
				return nil, p.errf(p.cur(), "bad statement: unexpected %s", p.cur())
			}
		}
	}
}

// parseBody parses the body of if/while/for: either one statement or a block.
// Newlines before the body are skipped, as Toybox does, so `if (x)` on one
// line and the statement on the next still pair up.
func (p *bcParser) parseBody() ([]bcStmt, error) {
	for p.cur().kind == bcTokNewline {
		p.advance()
	}
	switch {
	case p.isOp("{"):
		p.advance()
		return p.parseStmtList(true)
	case p.isOp(";"):
		// An empty statement is a valid if/while/for body.
		p.advance()
		return nil, nil
	case p.isOp("}"):
		// The enclosing block ends here: the body is empty.
		return nil, nil
	case p.cur().kind == bcTokEOF:
		return nil, errBcIncomplete
	}
	st, err := p.parseStmt()
	if err != nil {
		return nil, err
	}
	if st == nil {
		return nil, nil
	}
	return []bcStmt{st}, nil
}

func (p *bcParser) parseStmt() (bcStmt, error) {
	t := p.cur()
	if t.kind == bcTokEOF {
		return nil, errBcIncomplete
	}
	if t.kind == bcTokString {
		p.advance()
		return &bcStringStmt{s: t.text}, nil
	}
	if t.kind == bcTokKeyword {
		switch t.text {
		case "if":
			return p.parseIf()
		case "while":
			return p.parseWhile()
		case "for":
			return p.parseFor()
		case "define":
			return p.parseDefine()
		case "break":
			p.advance()
			if p.loopDepth == 0 {
				return nil, p.errf(t, "break outside a loop")
			}
			return &bcBreakStmt{}, nil
		case "continue":
			p.advance()
			if p.loopDepth == 0 {
				return nil, p.errf(t, "continue outside a loop")
			}
			return &bcContinueStmt{}, nil
		case "quit":
			// Toybox treats quit as a compile-time command: parsing stops and
			// nothing parsed in this unit runs.
			return nil, errBcQuit
		case "halt":
			p.advance()
			return &bcHaltStmt{}, nil
		case "limits":
			p.advance()
			return &bcLimitsStmt{}, nil
		case "return":
			return p.parseReturn()
		case "print":
			return p.parsePrint()
		case "auto":
			return nil, p.errf(t, "no auto variable found")
		}
	}
	startsParen := p.isOp("(")
	x, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &bcExprStmt{x: x, paren: startsParen}, nil
}

func (p *bcParser) parseIf() (bcStmt, error) {
	p.advance() // if
	if err := p.expectOp("("); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if err := p.expectOp(")"); err != nil {
		return nil, err
	}
	then, err := p.parseBody()
	if err != nil {
		return nil, err
	}
	st := &bcIfStmt{cond: cond, then: then}
	save := p.pos
	p.skipSeparators()
	if p.isKeyword("else") {
		p.advance()
		p.skipSeparators()
		els, err := p.parseBody()
		if err != nil {
			return nil, err
		}
		st.els = els
	} else {
		p.pos = save
	}
	return st, nil
}

func (p *bcParser) parseWhile() (bcStmt, error) {
	p.advance()
	if err := p.expectOp("("); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if err := p.expectOp(")"); err != nil {
		return nil, err
	}
	p.loopDepth++
	body, err := p.parseBody()
	p.loopDepth--
	if err != nil {
		return nil, err
	}
	return &bcWhileStmt{cond: cond, body: body}, nil
}

func (p *bcParser) parseFor() (bcStmt, error) {
	p.advance()
	if err := p.expectOp("("); err != nil {
		return nil, err
	}
	st := &bcForStmt{}
	var err error
	// The three header parts may each start on a new line (Toybox skips the
	// newlines inside for(...)).
	p.skipNewlines()
	if !p.isOp(";") {
		if st.init, err = p.parseExpr(); err != nil {
			return nil, err
		}
	}
	if err := p.expectOp(";"); err != nil {
		return nil, err
	}
	p.skipNewlines()
	if !p.isOp(";") {
		if st.cond, err = p.parseExpr(); err != nil {
			return nil, err
		}
	}
	if err := p.expectOp(";"); err != nil {
		return nil, err
	}
	p.skipNewlines()
	if !p.isOp(")") {
		if st.update, err = p.parseExpr(); err != nil {
			return nil, err
		}
	}
	if err := p.expectOp(")"); err != nil {
		return nil, err
	}
	p.loopDepth++
	st.body, err = p.parseBody()
	p.loopDepth--
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (p *bcParser) parseReturn() (bcStmt, error) {
	t := p.advance()
	if p.funcDepth == 0 {
		return nil, p.errf(t, "bad return: not in a function")
	}
	st := &bcReturnStmt{}
	if p.isSeparator() || p.cur().kind == bcTokEOF || p.isOp("}") {
		return st, nil
	}
	x, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.voidFunc {
		return nil, p.errf(t, "cannot return a value from a void function")
	}
	st.x = x
	st.hasX = true
	return st, nil
}

func (p *bcParser) parsePrint() (bcStmt, error) {
	p.advance()
	st := &bcPrintStmt{}
	for {
		t := p.cur()
		if t.kind == bcTokEOF {
			return nil, errBcIncomplete
		}
		if p.isSeparator() || (t.kind == bcTokOp && t.text == "}") {
			break
		}
		if t.kind == bcTokString {
			p.advance()
			st.items = append(st.items, bcPrintItem{str: t.text, isStr: true})
		} else {
			x, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			st.items = append(st.items, bcPrintItem{x: x})
		}
		if !p.acceptOp(",") {
			break
		}
		// A newline after the comma continues the list, so a long print can
		// be split across lines.
		p.skipNewlines()
		if p.cur().kind == bcTokEOF {
			return nil, errBcIncomplete
		}
		if p.isSeparator() || (p.cur().kind == bcTokOp && p.cur().text == "}") {
			return nil, p.errf(p.cur(), "bad print statement")
		}
	}
	if len(st.items) == 0 {
		return nil, p.errf(p.cur(), "bad print statement")
	}
	return st, nil
}

func (p *bcParser) parseDefine() (bcStmt, error) {
	t := p.advance() // define
	if p.funcDepth > 0 {
		return nil, p.errf(t, "bad function definition: define inside a function")
	}
	st := &bcDefineStmt{}
	if p.isKeyword("void") {
		p.advance()
		st.void = true
	}
	nameTok := p.cur()
	if nameTok.kind != bcTokName {
		return nil, p.errf(t, "bad function definition: expected a name")
	}
	p.advance()
	st.name = nameTok.text
	if err := p.expectOp("("); err != nil {
		return nil, err
	}
	if !p.isOp(")") {
		for {
			pt := p.cur()
			if pt.kind == bcTokEOF {
				return nil, errBcIncomplete
			}
			if pt.kind != bcTokName {
				return nil, p.errf(pt, "bad function definition: expected a parameter name")
			}
			p.advance()
			if p.isOp("[") {
				return nil, p.errf(pt, "array parameters are not supported")
			}
			for _, existing := range st.params {
				if existing == pt.text {
					return nil, p.errf(pt, "function parameter or auto %q already exists", pt.text)
				}
			}
			st.params = append(st.params, pt.text)
			if !p.acceptOp(",") {
				break
			}
		}
	}
	if err := p.expectOp(")"); err != nil {
		return nil, err
	}
	p.skipSeparators()
	if !p.isOp("{") {
		return nil, p.errf(p.cur(), "bad function definition: expected {")
	}
	p.advance()
	p.funcDepth++
	savedVoid := p.voidFunc
	p.voidFunc = st.void
	defer func() { p.funcDepth--; p.voidFunc = savedVoid }()

	p.skipSeparators()
	// Only one auto statement is allowed (a second one is an error, as in
	// Toybox: it falls through to the `auto` statement case).
	if p.isKeyword("auto") {
		autoTok := p.advance()
		for {
			at := p.cur()
			if at.kind == bcTokEOF {
				return nil, errBcIncomplete
			}
			if at.kind != bcTokName {
				return nil, p.errf(autoTok, "no auto variable found")
			}
			p.advance()
			if p.isOp("[") {
				return nil, p.errf(at, "array autos are not supported")
			}
			for _, existing := range st.params {
				if existing == at.text {
					return nil, p.errf(at, "function parameter or auto %q already exists", at.text)
				}
			}
			for _, existing := range st.autos {
				if existing == at.text {
					return nil, p.errf(at, "function parameter or auto %q already exists", at.text)
				}
			}
			st.autos = append(st.autos, at.text)
			if !p.acceptOp(",") {
				break
			}
		}
		if !p.isSeparator() && !p.isOp("}") && p.cur().kind != bcTokEOF {
			return nil, p.errf(p.cur(), "bad auto statement")
		}
		p.skipSeparators()
	}
	body, err := p.parseStmtList(true)
	if err != nil {
		return nil, err
	}
	st.body = body
	return st, nil
}

// ---- Expressions ----

func (p *bcParser) parseExpr() (bcExpr, error) {
	p.skipNewlines()
	return p.parseAssign()
}

func bcIsLvalue(e bcExpr) bool {
	switch e := e.(type) {
	case *bcVarRef, *bcArrayElem, *bcSpecialRef, *bcLastRef:
		return true
	case *bcParenExpr:
		return bcIsLvalue(e.x)
	}
	return false
}

func (p *bcParser) parseAssign() (bcExpr, error) {
	left, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	t := p.cur()
	if t.kind == bcTokOp {
		switch t.text {
		case "=", "+=", "-=", "*=", "/=", "%=", "^=":
			if !bcIsLvalue(left) {
				return nil, p.errf(t, "bad assignment: left side must be scale, ibase, obase, last, var, or array element")
			}
			p.advance()
			right, err := p.parseAssign()
			if err != nil {
				return nil, err
			}
			return &bcAssignExpr{op: t.text, l: left, r: right}, nil
		}
	}
	return left, nil
}

func (p *bcParser) parseOr() (bcExpr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.isOp("||") {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &bcBinaryExpr{op: "||", l: left, r: right}
	}
	return left, nil
}

func (p *bcParser) parseAnd() (bcExpr, error) {
	left, err := p.parseRel()
	if err != nil {
		return nil, err
	}
	for p.isOp("&&") {
		p.advance()
		right, err := p.parseRel()
		if err != nil {
			return nil, err
		}
		left = &bcBinaryExpr{op: "&&", l: left, r: right}
	}
	return left, nil
}

func (p *bcParser) parseRel() (bcExpr, error) {
	left, err := p.parseAdd()
	if err != nil {
		return nil, err
	}
	for {
		t := p.cur()
		if t.kind != bcTokOp {
			return left, nil
		}
		switch t.text {
		case "==", "!=", "<", "<=", ">", ">=":
			p.advance()
			right, err := p.parseAdd()
			if err != nil {
				return nil, err
			}
			left = &bcBinaryExpr{op: t.text, l: left, r: right}
		default:
			return left, nil
		}
	}
}

func (p *bcParser) parseAdd() (bcExpr, error) {
	left, err := p.parseMul()
	if err != nil {
		return nil, err
	}
	for {
		t := p.cur()
		if t.kind != bcTokOp || (t.text != "+" && t.text != "-") {
			return left, nil
		}
		p.advance()
		right, err := p.parseMul()
		if err != nil {
			return nil, err
		}
		left = &bcBinaryExpr{op: t.text, l: left, r: right}
	}
}

func (p *bcParser) parseMul() (bcExpr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		t := p.cur()
		if t.kind != bcTokOp || (t.text != "*" && t.text != "/" && t.text != "%") {
			return left, nil
		}
		p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &bcBinaryExpr{op: t.text, l: left, r: right}
	}
}

func (p *bcParser) parseUnary() (bcExpr, error) {
	t := p.cur()
	if t.kind == bcTokOp {
		switch t.text {
		case "-":
			p.advance()
			x, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			return &bcUnaryExpr{op: "-", x: x}, nil
		case "!":
			p.advance()
			x, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			return &bcUnaryExpr{op: "!", x: x}, nil
		case "++", "--":
			p.advance()
			x, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			if !bcIsLvalue(x) {
				return nil, p.errf(t, "bad %s: operand must be a variable", t.text)
			}
			if _, ok := x.(*bcParenExpr); ok {
				return nil, p.errf(t, "bad %s: operand must be a variable", t.text)
			}
			return &bcUnaryExpr{op: t.text, x: x}, nil
		}
	}
	return p.parsePower()
}

func (p *bcParser) parsePower() (bcExpr, error) {
	base, err := p.parsePostfix()
	if err != nil {
		return nil, err
	}
	if p.isOp("^") {
		p.advance()
		// Right associative, and the exponent may be signed: 2^-3 is valid.
		exp, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &bcBinaryExpr{op: "^", l: base, r: exp}, nil
	}
	return base, nil
}

func (p *bcParser) parsePostfix() (bcExpr, error) {
	x, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		t := p.cur()
		if t.kind != bcTokOp || (t.text != "++" && t.text != "--") {
			return x, nil
		}
		if !bcIsLvalue(x) {
			return nil, p.errf(t, "bad %s: operand must be a variable", t.text)
		}
		if _, ok := x.(*bcParenExpr); ok {
			return nil, p.errf(t, "bad %s: operand must be a variable", t.text)
		}
		p.advance()
		x = &bcPostfixExpr{op: t.text, x: x}
	}
}

func (p *bcParser) parsePrimary() (bcExpr, error) {
	t := p.cur()
	switch t.kind {
	case bcTokEOF:
		return nil, errBcIncomplete
	case bcTokNumber:
		p.advance()
		return &bcNumLit{text: t.text}, nil
	case bcTokString:
		return nil, p.errf(t, "bad expression: string in expression")
	case bcTokOp:
		if t.text == "(" {
			p.advance()
			x, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if err := p.expectOp(")"); err != nil {
				return nil, err
			}
			return &bcParenExpr{x: x}, nil
		}
		if t.text == "." {
			p.advance()
			return &bcLastRef{}, nil
		}
		return nil, p.errf(t, "bad expression: unexpected %s", t)
	case bcTokName:
		p.advance()
		if p.isOp("(") {
			p.advance()
			call := &bcCallExpr{name: t.text}
			if !p.isOp(")") {
				for {
					arg, err := p.parseExpr()
					if err != nil {
						return nil, err
					}
					call.args = append(call.args, arg)
					if !p.acceptOp(",") {
						break
					}
				}
			}
			if err := p.expectOp(")"); err != nil {
				return nil, err
			}
			return call, nil
		}
		if p.isOp("[") {
			p.advance()
			idx, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if err := p.expectOp("]"); err != nil {
				return nil, err
			}
			return &bcArrayElem{name: t.text, index: idx}, nil
		}
		return &bcVarRef{name: t.text}, nil
	case bcTokKeyword:
		switch t.text {
		case "ibase", "obase", "scale":
			p.advance()
			if p.isOp("(") && t.text == "scale" {
				p.advance()
				x, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				if err := p.expectOp(")"); err != nil {
					return nil, err
				}
				return &bcBuiltinExpr{name: "scale", x: x}, nil
			}
			return &bcSpecialRef{which: t.text}, nil
		case "last":
			p.advance()
			return &bcLastRef{}, nil
		case "length", "sqrt", "abs":
			p.advance()
			if err := p.expectOp("("); err != nil {
				return nil, err
			}
			x, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if err := p.expectOp(")"); err != nil {
				return nil, err
			}
			return &bcBuiltinExpr{name: t.text, x: x}, nil
		case "read":
			return nil, p.errf(t, "read() is not supported")
		}
	}
	return nil, p.errf(t, "bad expression: unexpected %s", t)
}
