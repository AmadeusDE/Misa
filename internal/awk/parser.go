package awk

import "fmt"

type parser struct {
	toks    []token
	pos     int
	vars    map[string]int
	noGT    bool
	groupAt int
	loops   int
}

// Parse compiles AWK source into a Program.
func Parse(src string) (prog *Program, err error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks, vars: map[string]int{}, groupAt: -1}
	for i, n := range specialNames {
		p.vars[n] = i
	}
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(*Error)
			if !ok {
				panic(r)
			}
			prog, err = nil, e
		}
	}()
	prog = p.program()
	prog.vars, prog.nvars = p.vars, len(p.vars)
	return prog, nil
}

func (p *parser) tok() token { return p.toks[p.pos] }

func (p *parser) advance() token {
	t := p.toks[p.pos]
	if t.kind != tEOF {
		p.pos++
	}
	return t
}

func (p *parser) is(k tokKind) bool { return p.tok().kind == k }

func (p *parser) isKeyword(w string) bool {
	t := p.tok()
	return t.kind == tKeyword && t.text == w
}

func (p *parser) fail(msg string) {
	panic(&Error{Line: p.tok().line, Msg: msg})
}

func (p *parser) failUnexpected() {
	t := p.tok()
	switch {
	case t.kind == tPipe:
		panic(unsupported(t.line, "pipes (| command)", "deferred to the AWK language completion task"))
	case t.kind == tAppend:
		panic(unsupported(t.line, "output redirection (>>)", "deferred to the AWK language completion task"))
	case t.kind == tGT && p.noGT:
		panic(unsupported(t.line, "output redirection (>)", "deferred to the AWK language completion task"))
	}
	p.unsupportedKeyword()
	switch t.kind {
	case tEOF:
		p.fail("syntax error at end of program")
	case tNewline:
		p.fail("syntax error at end of line")
	}
	p.fail(fmt.Sprintf("syntax error near %q", tokenText(t)))
}

func tokenText(t token) string {
	if t.text != "" {
		return t.text
	}
	return fmt.Sprint(t.kind)
}

// unsupportedKeyword rejects the language features that are not implemented yet.
func (p *parser) unsupportedKeyword() {
	t := p.tok()
	if t.kind != tKeyword {
		return
	}
	switch t.text {
	case "function", "func":
		panic(unsupported(t.line, "user-defined functions", "deferred to the AWK language completion task"))
	case "return":
		panic(unsupported(t.line, "return (user-defined functions)", "deferred to the AWK language completion task"))
	case "delete", "in":
		panic(unsupported(t.line, "arrays ("+t.text+")", "deferred to the AWK language completion task"))
	case "getline":
		panic(unsupported(t.line, "getline", "deferred to the AWK language completion task"))
	}
}

func (p *parser) expect(k tokKind) token {
	if !p.is(k) {
		p.failUnexpected()
	}
	return p.advance()
}

func (p *parser) skipNewlines() {
	for p.is(tNewline) {
		p.advance()
	}
}

func (p *parser) skipTerminators() {
	for p.is(tNewline) || p.is(tSemi) {
		p.advance()
	}
}

func (p *parser) variable(name string) *varRef {
	i, ok := p.vars[name]
	if !ok {
		i = len(p.vars)
		p.vars[name] = i
	}
	return &varRef{idx: i, name: name}
}

func (p *parser) program() *Program {
	prog := &Program{}
	p.skipTerminators()
	for !p.is(tEOF) {
		p.item(prog)
		p.skipTerminators()
	}
	return prog
}

func (p *parser) item(prog *Program) {
	switch {
	case p.isKeyword("BEGIN"):
		p.advance()
		prog.begins = append(prog.begins, p.action())
	case p.isKeyword("END"):
		p.advance()
		prog.ends = append(prog.ends, p.action())
	case p.is(tLbrace):
		prog.rules = append(prog.rules, &rule{action: p.action()})
	default:
		prog.rules = append(prog.rules, p.patternRule())
	}
}

func (p *parser) patternRule() *rule {
	p.unsupportedKeyword()
	r := &rule{pattern: p.expression()}
	if p.is(tComma) {
		p.advance()
		p.skipNewlines()
		r.ranged = true
		r.r2 = p.expression()
	}
	if p.is(tLbrace) {
		r.action = p.action()
	} else if !p.is(tNewline) && !p.is(tSemi) && !p.is(tEOF) {
		p.failUnexpected()
	}
	return r
}

func (p *parser) action() []stmt {
	p.expect(tLbrace)
	var list []stmt
	p.skipTerminators()
	for !p.is(tRbrace) {
		if p.is(tEOF) {
			p.fail("missing } at end of program")
		}
		list = append(list, p.statement())
		p.skipTerminators()
	}
	p.advance()
	if list == nil {
		list = []stmt{}
	}
	return list
}

func (p *parser) statement() stmt {
	t := p.tok()
	switch {
	case t.kind == tLbrace:
		return &blockStmt{list: p.action()}
	case t.kind == tSemi:
		p.advance()
		return &blockStmt{}
	case t.kind == tKeyword:
		switch t.text {
		case "if":
			return p.ifStatement()
		case "while":
			return p.whileStatement()
		case "do":
			return p.doStatement()
		case "for":
			return p.forStatement()
		}
	}
	s := p.simple()
	p.endSimple()
	return s
}

func (p *parser) endSimple() {
	switch {
	case p.is(tSemi), p.is(tNewline):
		p.advance()
	case p.is(tRbrace), p.is(tEOF):
	default:
		p.failUnexpected()
	}
}

func (p *parser) body() stmt {
	p.skipNewlines()
	return p.statement()
}

func (p *parser) condition() expr {
	p.expect(tLparen)
	c := p.expression()
	p.expect(tRparen)
	return c
}

func (p *parser) ifStatement() stmt {
	p.advance()
	s := &ifStmt{cond: p.condition()}
	s.then = p.body()
	save := p.pos
	p.skipTerminators()
	if p.isKeyword("else") {
		p.advance()
		s.els = p.body()
	} else {
		p.pos = save
	}
	return s
}

func (p *parser) whileStatement() stmt {
	p.advance()
	s := &whileStmt{cond: p.condition()}
	p.loops++
	s.body = p.body()
	p.loops--
	return s
}

func (p *parser) doStatement() stmt {
	p.advance()
	p.loops++
	body := p.body()
	p.loops--
	p.skipTerminators()
	if !p.isKeyword("while") {
		p.failUnexpected()
	}
	p.advance()
	s := &doStmt{body: body, cond: p.condition()}
	p.endSimple()
	return s
}

func (p *parser) forStatement() stmt {
	p.advance()
	p.expect(tLparen)
	if p.is(tName) && p.toks[p.pos+1].kind == tKeyword && p.toks[p.pos+1].text == "in" {
		panic(unsupported(p.tok().line, "arrays (for (x in array))", "deferred to the AWK language completion task"))
	}
	s := &forStmt{}
	if !p.is(tSemi) {
		s.init = p.simple()
	}
	p.expect(tSemi)
	p.skipNewlines()
	if !p.is(tSemi) {
		s.cond = p.expression()
	}
	p.expect(tSemi)
	p.skipNewlines()
	if !p.is(tRparen) {
		s.post = p.simple()
	}
	p.expect(tRparen)
	p.loops++
	s.body = p.body()
	p.loops--
	return s
}

func (p *parser) simple() stmt {
	t := p.tok()
	if t.kind == tKeyword {
		switch t.text {
		case "print", "printf":
			return p.printStatement()
		case "next":
			p.advance()
			return &nextStmt{}
		case "nextfile":
			p.advance()
			return &nextFileStmt{}
		case "break", "continue":
			p.advance()
			if p.loops == 0 {
				p.fail(t.text + " outside a loop")
			}
			if t.text == "break" {
				return &breakStmt{}
			}
			return &continueStmt{}
		case "exit":
			p.advance()
			s := &exitStmt{}
			if !p.is(tSemi) && !p.is(tNewline) && !p.is(tRbrace) && !p.is(tEOF) {
				s.e = p.expression()
			}
			return s
		}
		p.unsupportedKeyword()
	}
	return &exprStmt{e: p.expression()}
}

func (p *parser) printStatement() stmt {
	s := &printStmt{printf: p.advance().text == "printf"}
	p.noGT = true
	defer func() { p.noGT = false }()
	if !p.is(tSemi) && !p.is(tNewline) && !p.is(tRbrace) && !p.is(tEOF) && !p.is(tGT) && !p.is(tAppend) && !p.is(tPipe) {
		p.groupAt = p.pos
		s.args = append(s.args, p.expression())
		for p.is(tComma) {
			p.advance()
			p.skipNewlines()
			s.args = append(s.args, p.expression())
		}
	}
	if g, ok := firstGroup(s.args); ok {
		s.args = g.items
	}
	for _, a := range s.args {
		if _, ok := a.(*groupExpr); ok {
			p.fail("syntax error: unexpected expression list")
		}
	}
	if !p.endsPrintArgs() || p.is(tGT) || p.is(tAppend) || p.is(tPipe) {
		p.failUnexpected()
	}
	if s.printf && len(s.args) == 0 {
		p.fail("printf: no format")
	}
	return s
}

func (p *parser) endsPrintArgs() bool {
	switch p.tok().kind {
	case tSemi, tNewline, tRbrace, tEOF, tGT, tAppend, tPipe:
		return true
	}
	return false
}

func firstGroup(args []expr) (*groupExpr, bool) {
	if len(args) != 1 {
		return nil, false
	}
	g, ok := args[0].(*groupExpr)
	return g, ok
}

func isLvalue(e expr) bool {
	switch e.(type) {
	case *varRef, *fieldRef:
		return true
	}
	return false
}

func (p *parser) expression() expr { return p.assignment() }

var assignOps = map[tokKind]bool{
	tAssign: true, tAddAssign: true, tSubAssign: true, tMulAssign: true,
	tDivAssign: true, tModAssign: true, tPowAssign: true,
}

func (p *parser) assignment() expr {
	left := p.ternary()
	if assignOps[p.tok().kind] && isLvalue(left) {
		op := p.advance().kind
		p.skipNewlines()
		return &assignExpr{op: op, lv: left, rhs: p.assignment()}
	}
	return left
}

func (p *parser) ternary() expr {
	c := p.or()
	if !p.is(tQuestion) {
		return c
	}
	p.advance()
	p.skipNewlines()
	a := p.assignment()
	p.skipNewlines()
	p.expect(tColon)
	p.skipNewlines()
	return &condExpr{c: c, a: a, b: p.assignment()}
}

func (p *parser) or() expr {
	l := p.and()
	for p.is(tOr) {
		p.advance()
		p.skipNewlines()
		l = &orExpr{l: l, r: p.and()}
	}
	return l
}

func (p *parser) and() expr {
	l := p.match()
	for p.is(tAnd) {
		p.advance()
		p.skipNewlines()
		l = &andExpr{l: l, r: p.match()}
	}
	return l
}

func (p *parser) match() expr {
	l := p.comparison()
	for p.is(tTilde) || p.is(tNoMatch) {
		neg := p.advance().kind == tNoMatch
		l = &matchExpr{negate: neg, l: l, r: p.comparison()}
	}
	if p.isKeyword("in") {
		p.unsupportedKeyword()
	}
	return l
}

func (p *parser) comparison() expr {
	l := p.concat()
	switch p.tok().kind {
	case tLT, tLe, tEq, tNe, tGe:
	case tGT:
		if p.noGT {
			return l
		}
	default:
		return l
	}
	op := p.advance().kind
	return &cmpExpr{op: op, l: l, r: p.concat()}
}

func (p *parser) startsOperand() bool {
	switch p.tok().kind {
	case tNumber, tString, tName, tFuncName, tBuiltin, tDollar, tLparen, tIncr, tDecr, tNot, tRegex:
		return true
	}
	return false
}

func (p *parser) concat() expr {
	l := p.additive()
	for p.startsOperand() && !p.is(tNot) {
		l = &concatExpr{l: l, r: p.additive()}
	}
	return l
}

func (p *parser) additive() expr {
	l := p.multiplicative()
	for p.is(tPlus) || p.is(tMinus) {
		op := p.advance().kind
		l = &binaryExpr{op: op, l: l, r: p.multiplicative()}
	}
	return l
}

func (p *parser) multiplicative() expr {
	l := p.unary()
	for p.is(tStar) || p.is(tSlash) || p.is(tPercent) {
		op := p.advance().kind
		l = &binaryExpr{op: op, l: l, r: p.unary()}
	}
	return l
}

func (p *parser) unary() expr {
	switch p.tok().kind {
	case tNot:
		p.advance()
		return &notExpr{e: p.unary()}
	case tMinus, tPlus:
		op := p.advance().kind
		return &unaryExpr{op: op, e: p.unary()}
	}
	return p.power()
}

func (p *parser) power() expr {
	base := p.postfix()
	if p.is(tCaret) {
		p.advance()
		return &binaryExpr{op: tCaret, l: base, r: p.unary()}
	}
	return base
}

func (p *parser) postfix() expr {
	e := p.primary()
	if isLvalue(e) && (p.is(tIncr) || p.is(tDecr)) {
		delta := 1.0
		if p.advance().kind == tDecr {
			delta = -1
		}
		return &incDecExpr{delta: delta, lv: e}
	}
	return e
}

func (p *parser) primary() expr {
	t := p.tok()
	switch t.kind {
	case tNumber:
		p.advance()
		return &numLit{v: num(t.num)}
	case tString:
		p.advance()
		return &strLit{v: str(t.text)}
	case tRegex:
		p.advance()
		return &regexLit{src: t.text}
	case tDollar:
		p.advance()
		return &fieldRef{index: p.fieldIndex()}
	case tIncr, tDecr:
		p.advance()
		lv := p.primary()
		if !isLvalue(lv) {
			p.fail("++/-- needs a variable or field")
		}
		delta := 1.0
		if t.kind == tDecr {
			delta = -1
		}
		return &incDecExpr{pre: true, delta: delta, lv: lv}
	case tLparen:
		return p.parenthesized()
	case tName:
		p.advance()
		if p.is(tLbracket) {
			panic(unsupported(t.line, "arrays ("+t.text+"[...])", "deferred to the AWK language completion task"))
		}
		return p.variable(t.text)
	case tFuncName:
		panic(&Error{Line: t.line, Msg: fmt.Sprintf("call to undefined function %s (user-defined functions are not supported yet)", t.text)})
	case tBuiltin:
		return p.builtinCall()
	}
	p.failUnexpected()
	return nil
}

func (p *parser) fieldIndex() expr {
	switch p.tok().kind {
	case tIncr, tDecr:
		return p.primary()
	case tMinus, tPlus:
		op := p.advance().kind
		return &unaryExpr{op: op, e: p.primary()}
	}
	return p.primary()
}

func (p *parser) parenthesized() expr {
	open := p.pos
	p.advance()
	saved := p.noGT
	p.noGT = false
	e := p.expression()
	if p.is(tComma) {
		items := []expr{e}
		for p.is(tComma) {
			p.advance()
			p.skipNewlines()
			items = append(items, p.expression())
		}
		if open != p.groupAt {
			if p.is(tRparen) && p.toks[p.pos+1].kind == tKeyword && p.toks[p.pos+1].text == "in" {
				panic(unsupported(p.tok().line, "arrays ((i, j) in array)", "deferred to the AWK language completion task"))
			}
			p.fail("syntax error: unexpected expression list")
		}
		e = &groupExpr{items: items}
	}
	p.noGT = saved
	p.expect(tRparen)
	if _, ok := e.(*groupExpr); ok && !p.endsPrintArgs() {
		p.unsupportedKeyword()
		p.fail("syntax error: unexpected expression list")
	}
	return e
}

type arity struct{ min, max int }

var builtinArity = map[string]arity{
	"length": {0, 1}, "substr": {2, 3}, "index": {2, 2}, "sprintf": {1, -1},
	"tolower": {1, 1}, "toupper": {1, 1}, "int": {1, 1}, "sqrt": {1, 1}, "exp": {1, 1},
	"log": {1, 1}, "sin": {1, 1}, "cos": {1, 1}, "atan2": {2, 2}, "rand": {0, 0},
	"srand": {0, 1}, "match": {2, 2}, "sub": {2, 3}, "gsub": {2, 3},
}

func (p *parser) builtinCall() expr {
	t := p.advance()
	ar, ok := builtinArity[t.text]
	if !ok {
		panic(unsupported(t.line, t.text+"()", "deferred to the AWK language completion task"))
	}
	call := &callExpr{name: t.text}
	if !p.is(tLparen) {
		if t.text != "length" {
			p.failUnexpected()
		}
		return call
	}
	p.advance()
	saved := p.noGT
	p.noGT = false
	p.skipNewlines()
	for !p.is(tRparen) {
		call.args = append(call.args, p.expression())
		p.skipNewlines()
		if !p.is(tComma) {
			break
		}
		p.advance()
		p.skipNewlines()
	}
	p.noGT = saved
	p.expect(tRparen)
	if len(call.args) < ar.min || (ar.max >= 0 && len(call.args) > ar.max) {
		panic(&Error{Line: t.line, Msg: fmt.Sprintf("wrong number of arguments to %s", t.text)})
	}
	if t.text == "sub" || t.text == "gsub" {
		if len(call.args) == 3 && !isLvalue(call.args[2]) {
			panic(&Error{Line: t.line, Msg: t.text + ": third argument must be a variable or field"})
		}
	}
	return call
}
