package builtins

// Evaluator for the bc AST plus bc's output formatting (70-character
// continuation wrapping like Toybox toys/pending/bc.c, 0BSD, commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb).

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
)

const (
	// bcMaxArrayIndex bounds an array subscript (Toybox: BC_DIM_MAX).
	bcMaxArrayIndex = 1000000
	// bcMaxCallDepth bounds function recursion so a runaway recursion is an
	// error instead of an out-of-memory crash.
	bcMaxCallDepth = 1000
	// bcMaxStringLen bounds string literals.
	bcMaxStringLen = 1000000
)

// errBcVoid marks a call to a void function so statement context can accept it.
var errBcVoid = errors.New("void value")

// bcOutput writes bc output while tracking the column, inserting a backslash
// and newline when a number reaches the line limit (Toybox's TT.nchars).
type bcOutput struct {
	w       io.Writer
	nchars  int
	lineLen int
	err     error
}

func newBcOutput(w io.Writer, lineLen int) *bcOutput {
	if lineLen < 2 {
		lineLen = 2
	}
	return &bcOutput{w: w, lineLen: lineLen}
}

func (o *bcOutput) put(s string) {
	if o.err != nil || s == "" {
		return
	}
	if _, err := io.WriteString(o.w, s); err != nil {
		o.err = err
	}
	o.nchars += len(s)
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		o.nchars = len(s) - i - 1
	}
}

func (o *bcOutput) putByte(c byte) { o.put(string(c)) }

func (o *bcOutput) newline() {
	o.put("\n")
	o.nchars = 0
}

// wrap emits the continuation sequence when the line is full.
func (o *bcOutput) wrap() {
	if o.err != nil {
		return
	}
	if o.nchars >= o.lineLen-1 {
		o.put("\\\n")
		o.nchars = 0
	}
}

// printEscaped writes a string statement or print argument, substituting the
// escape sequences bc documents (\a \b \\ \e \f \n \" \r \t).
func (o *bcOutput) printEscaped(s string) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			i++
			switch e := s[i]; e {
			case 'a':
				c = '\a'
			case 'b':
				c = '\b'
			case '\\':
				c = '\\'
			case 'e':
				c = 0x1b
			case 'f':
				c = '\f'
			case 'n':
				c = '\n'
			case '"':
				c = '"'
			case 'r':
				c = '\r'
			case 't':
				c = '\t'
			default:
				o.putByte('\\')
				c = e
			}
		}
		o.putByte(c)
		if c == '\n' {
			o.nchars = 0
		}
	}
}

// printNumber writes a value in the current obase with bc's wrapping.
func (o *bcOutput) printNumber(n bcNumber, obase int) {
	if o.err != nil {
		return
	}
	if obase == 10 {
		o.printDecimal(n)
		return
	}
	o.wrap()
	if n.isZero() {
		o.putByte('0')
		return
	}
	if n.isNeg() {
		o.putByte('-')
	}
	base := big.NewInt(int64(obase))
	intPart := new(big.Int).Abs(n.unscaled)
	if n.scale > 0 {
		intPart.Quo(intPart, bcPow10(n.scale))
	}
	var digits []int
	for intPart.Sign() != 0 {
		q, r := new(big.Int).QuoRem(intPart, base, new(big.Int))
		digits = append(digits, int(r.Int64()))
		intPart = q
	}
	width := 1
	if obase > 16 {
		width = len(fmt.Sprint(obase - 1))
		for i := len(digits) - 1; i >= 0; i-- {
			o.printGroup(' ', int64(digits[i]), width)
		}
	} else {
		for i := len(digits) - 1; i >= 0; i-- {
			o.wrap()
			o.putByte("0123456789ABCDEF"[digits[i]])
		}
	}
	if n.scale == 0 {
		return
	}
	// Fraction digits: keep multiplying the remainder by the base until the
	// decimal digit count of base^k exceeds the value's scale, which is how
	// Toybox sizes the expansion.
	den := bcPow10(n.scale)
	frac := new(big.Int).Mod(new(big.Int).Abs(n.unscaled), den)
	fracLen := big.NewInt(1)
	first := true
	for len(fracLen.String()) <= n.scale {
		frac.Mul(frac, base)
		digit, rem := new(big.Int).QuoRem(frac, den, new(big.Int))
		frac = rem
		if obase > 16 {
			sep := byte(' ')
			if first {
				sep = '.'
			}
			o.printGroup(sep, digit.Int64(), width)
		} else {
			if first {
				o.wrap()
				o.putByte('.')
			}
			o.wrap()
			o.putByte("0123456789ABCDEF"[digit.Int64()])
		}
		first = false
		fracLen.Mul(fracLen, base)
	}
}

// printGroup writes one base>16 digit: a separator, then the digit padded to
// width decimal places.
func (o *bcOutput) printGroup(sep byte, digit int64, width int) {
	if sep != 0 {
		o.wrap()
		o.putByte(sep)
	}
	text := fmt.Sprintf("%0*d", width, digit)
	for i := 0; i < len(text); i++ {
		o.wrap()
		o.putByte(text[i])
	}
}

func (o *bcOutput) printDecimal(n bcNumber) {
	o.wrap()
	if n.isZero() {
		o.putByte('0')
		return
	}
	text := n.decimalString()
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c != '-' {
			o.wrap()
		}
		o.putByte(c)
	}
}

// ---- Interpreter ----

type bcSignal int

const (
	bcSigNone bcSignal = iota
	bcSigBreak
	bcSigContinue
	bcSigReturn
	bcSigHalt
)

type bcFrame struct {
	locals map[string]bcNumber
}

type bcInterp struct {
	out     *bcOutput
	ctx     *Context
	scale   int
	ibase   int
	obase   int
	last    bcNumber
	globals map[string]bcNumber
	arrays  map[string][]bcNumber
	funcs   map[string]*bcDefineStmt
	frames  []*bcFrame
	steps   int
	cancel  <-chan struct{}
	retVal  bcNumber
}

func newBcInterp(ctx *Context, out *bcOutput) *bcInterp {
	in := &bcInterp{
		out:     out,
		ctx:     ctx,
		scale:   0,
		ibase:   10,
		obase:   10,
		last:    bcNumZero(),
		globals: map[string]bcNumber{},
		arrays:  map[string][]bcNumber{},
		funcs:   map[string]*bcDefineStmt{},
	}
	if ctx != nil {
		in.cancel = ctx.Cancel
	}
	return in
}

// tick bounds work and honours cancellation in loops and long evaluations.
func (in *bcInterp) tick() error {
	in.steps++
	if in.steps&0x3ff != 0 {
		return nil
	}
	if in.out != nil && in.out.err != nil {
		return in.out.err
	}
	if in.cancel != nil {
		select {
		case <-in.cancel:
			return errBcInterrupted
		default:
		}
	}
	return nil
}

// runStmts executes statements until a control-flow signal or an error.
func (in *bcInterp) runStmts(stmts []bcStmt) (bcSignal, error) {
	for _, st := range stmts {
		sig, err := in.runStmt(st)
		if err != nil || sig != bcSigNone {
			return sig, err
		}
	}
	return bcSigNone, nil
}

func (in *bcInterp) runStmt(st bcStmt) (bcSignal, error) {
	if err := in.tick(); err != nil {
		return bcSigNone, err
	}
	switch st := st.(type) {
	case *bcExprStmt:
		v, err := in.eval(st.x)
		if err != nil {
			if errors.Is(err, errBcVoid) {
				return bcSigNone, nil
			}
			return bcSigNone, err
		}
		if _, ok := st.x.(*bcAssignExpr); ok && !st.paren {
			// A bare assignment is not printed and does not set `last`.
			return bcSigNone, nil
		}
		in.last = v
		in.out.printNumber(v, in.obase)
		in.out.newline()
		return bcSigNone, in.out.err
	case *bcStringStmt:
		// A string statement prints the string verbatim, with no escape
		// processing and no trailing newline (Toybox BC_INST_PRINT_STR).
		in.out.put(st.s)
		return bcSigNone, in.out.err
	case *bcPrintStmt:
		for _, item := range st.items {
			if item.isStr {
				in.out.printEscaped(item.str)
				continue
			}
			v, err := in.evalNum(item.x)
			if err != nil {
				return bcSigNone, err
			}
			in.last = v
			in.out.printNumber(v, in.obase)
			if in.out.err != nil {
				return bcSigNone, in.out.err
			}
		}
		return bcSigNone, nil
	case *bcIfStmt:
		cond, err := in.evalNum(st.cond)
		if err != nil {
			return bcSigNone, err
		}
		if !cond.isZero() {
			return in.runStmts(st.then)
		}
		return in.runStmts(st.els)
	case *bcWhileStmt:
		for {
			if err := in.tick(); err != nil {
				return bcSigNone, err
			}
			cond, err := in.evalNum(st.cond)
			if err != nil {
				return bcSigNone, err
			}
			if cond.isZero() {
				return bcSigNone, nil
			}
			sig, err := in.runStmts(st.body)
			if err != nil {
				return bcSigNone, err
			}
			switch sig {
			case bcSigBreak:
				return bcSigNone, nil
			case bcSigHalt:
				return bcSigHalt, nil
			case bcSigReturn:
				return sig, nil
			}
		}
	case *bcForStmt:
		if st.init != nil {
			if _, err := in.evalNum(st.init); err != nil {
				return bcSigNone, err
			}
		}
		for {
			if err := in.tick(); err != nil {
				return bcSigNone, err
			}
			if st.cond != nil {
				cond, err := in.evalNum(st.cond)
				if err != nil {
					return bcSigNone, err
				}
				if cond.isZero() {
					return bcSigNone, nil
				}
			}
			sig, err := in.runStmts(st.body)
			if err != nil {
				return bcSigNone, err
			}
			if sig == bcSigHalt || sig == bcSigReturn {
				return sig, nil
			}
			if sig == bcSigBreak {
				return bcSigNone, nil
			}
			if st.update != nil {
				if _, err := in.evalNum(st.update); err != nil {
					return bcSigNone, err
				}
			}
		}
	case *bcBreakStmt:
		return bcSigBreak, nil
	case *bcContinueStmt:
		return bcSigContinue, nil
	case *bcHaltStmt:
		return bcSigHalt, nil
	case *bcLimitsStmt:
		in.printLimits()
		return bcSigNone, in.out.err
	case *bcReturnStmt:
		in.retVal = bcNumZero()
		if st.hasX {
			v, err := in.evalNum(st.x)
			if err != nil {
				return bcSigNone, err
			}
			in.retVal = v
		}
		return bcSigReturn, nil
	case *bcDefineStmt:
		in.funcs[st.name] = st
		return bcSigNone, nil
	}
	return bcSigNone, fmt.Errorf("unsupported statement")
}

func (in *bcInterp) printLimits() {
	o := in.out
	o.put(fmt.Sprintf("BC_BASE_MAX     = %d\n", bcMaxObase))
	o.put(fmt.Sprintf("BC_DIM_MAX      = %d\n", bcMaxArrayIndex))
	o.put(fmt.Sprintf("BC_SCALE_MAX    = %d\n", bcMaxScale))
	o.put(fmt.Sprintf("BC_STRING_MAX   = %d\n", bcMaxStringLen))
	o.put(fmt.Sprintf("BC_NUM_MAX      = %d\n", bcMaxDigits))
	o.put(fmt.Sprintf("BC_NAME_MAX     = %d\n", bcMaxNameLen))
	o.put(fmt.Sprintf("MAX Exponent    = %d\n", bcMaxExponent))
	o.put(fmt.Sprintf("Number of vars  = %d\n", bcMaxVars))
	o.nchars = 0
}

// ---- Variables ----

// lookupVar finds a variable in the innermost frame that declares it, so
// autos and parameters are visible to functions called from that frame
// (dynamic scoping, like Toybox and GNU bc).
func (in *bcInterp) lookupVar(name string) bcNumber {
	for i := len(in.frames) - 1; i >= 0; i-- {
		if v, ok := in.frames[i].locals[name]; ok {
			return v
		}
	}
	if v, ok := in.globals[name]; ok {
		return v
	}
	return bcNumZero()
}

func (in *bcInterp) storeVar(name string, v bcNumber) {
	for i := len(in.frames) - 1; i >= 0; i-- {
		if _, ok := in.frames[i].locals[name]; ok {
			in.frames[i].locals[name] = v
			return
		}
	}
	in.globals[name] = v
}

func (in *bcInterp) arrayIndex(x bcExpr) (int, error) {
	v, err := in.evalNum(x)
	if err != nil {
		return 0, err
	}
	idx, err := v.intValue()
	if err != nil {
		return 0, err
	}
	if idx < 0 {
		return 0, bcErrNegative
	}
	if idx > bcMaxArrayIndex {
		return 0, bcErrOverflow
	}
	return int(idx), nil
}

func (in *bcInterp) getLvalue(e bcExpr) (bcNumber, error) {
	switch e := e.(type) {
	case *bcParenExpr:
		return in.getLvalue(e.x)
	case *bcVarRef:
		return in.lookupVar(e.name), nil
	case *bcArrayElem:
		idx, err := in.arrayIndex(e.index)
		if err != nil {
			return bcNumber{}, err
		}
		arr := in.arrays[e.name]
		if idx >= len(arr) {
			return bcNumZero(), nil
		}
		return arr[idx], nil
	case *bcSpecialRef:
		return in.specialValue(e.which), nil
	case *bcLastRef:
		return in.last, nil
	}
	return bcNumber{}, fmt.Errorf("bad assignment target")
}

// bcRef is a resolved assignment target: the array index or special name is
// evaluated once, so `a[i++] += 1` and `x++` touch the index only once.
type bcRef struct {
	cur bcNumber
	set func(bcNumber) (bcNumber, error)
}

func (in *bcInterp) resolve(e bcExpr) (*bcRef, error) {
	switch e := e.(type) {
	case *bcParenExpr:
		return in.resolve(e.x)
	case *bcVarRef:
		name := e.name
		return &bcRef{cur: in.lookupVar(name), set: func(v bcNumber) (bcNumber, error) {
			in.storeVar(name, v)
			return v, nil
		}}, nil
	case *bcArrayElem:
		idx, err := in.arrayIndex(e.index)
		if err != nil {
			return nil, err
		}
		name := e.name
		cur := bcNumZero()
		if arr := in.arrays[name]; idx < len(arr) {
			cur = arr[idx]
		}
		return &bcRef{cur: cur, set: func(v bcNumber) (bcNumber, error) {
			arr := in.arrays[name]
			if idx >= len(arr) {
				grown := make([]bcNumber, idx+1)
				for i := range grown {
					grown[i] = bcNumZero()
				}
				copy(grown, arr)
				arr = grown
			}
			arr[idx] = v
			in.arrays[name] = arr
			return v, nil
		}}, nil
	case *bcSpecialRef:
		which := e.which
		return &bcRef{cur: in.specialValue(which), set: func(v bcNumber) (bcNumber, error) {
			return in.setSpecial(which, v)
		}}, nil
	case *bcLastRef:
		return &bcRef{cur: in.last, set: func(v bcNumber) (bcNumber, error) {
			in.last = v
			return v, nil
		}}, nil
	}
	return nil, fmt.Errorf("bad assignment target")
}

func (in *bcInterp) specialValue(which string) bcNumber {
	switch which {
	case "ibase":
		return bcNumInt(int64(in.ibase))
	case "obase":
		return bcNumInt(int64(in.obase))
	case "scale":
		return bcNumInt(int64(in.scale))
	}
	return bcNumZero()
}

// setSpecial assigns ibase, obase or scale after range checking the truncated
// integer value, and returns the right-hand side as written (`(scale = 2.7)`
// is 2.7 while scale becomes 2, as in Toybox).
func (in *bcInterp) setSpecial(which string, v bcNumber) (bcNumber, error) {
	iv, err := v.intValue()
	if err != nil {
		return bcNumber{}, err
	}
	switch which {
	case "ibase":
		if iv < bcMinBase || iv > bcMaxIbase {
			return bcNumber{}, fmt.Errorf("bad ibase; must be [%d, %d]", bcMinBase, bcMaxIbase)
		}
		in.ibase = int(iv)
	case "obase":
		if iv < bcMinBase || iv > bcMaxObase {
			return bcNumber{}, fmt.Errorf("bad obase; must be [%d, %d]", bcMinBase, bcMaxObase)
		}
		in.obase = int(iv)
	case "scale":
		if iv > bcMaxScale {
			return bcNumber{}, fmt.Errorf("bad scale; must be [%d, %d]", 0, bcMaxScale)
		}
		in.scale = int(iv)
	}
	return v, nil
}

// ---- Expressions ----

func (in *bcInterp) eval(e bcExpr) (bcNumber, error) {
	if err := in.tick(); err != nil {
		return bcNumber{}, err
	}
	switch e := e.(type) {
	case *bcParenExpr:
		return in.eval(e.x)
	case *bcNumLit:
		if len(e.text) > bcMaxDigits {
			return bcNumber{}, bcErrTooLarge
		}
		return bcParseNumber(e.text, in.ibase)
	case *bcVarRef, *bcArrayElem, *bcSpecialRef, *bcLastRef:
		return in.getLvalue(e)
	case *bcBuiltinExpr:
		v, err := in.evalNum(e.x)
		if err != nil {
			return bcNumber{}, err
		}
		switch e.name {
		case "length":
			return v.bcLength(), nil
		case "scale":
			return v.bcScaleOf(), nil
		case "sqrt":
			return bcSqrt(v, in.scale)
		case "abs":
			return v.abs(), nil
		}
		return bcNumber{}, fmt.Errorf("unsupported function %s()", e.name)
	case *bcUnaryExpr:
		switch e.op {
		case "-":
			v, err := in.evalNum(e.x)
			if err != nil {
				return bcNumber{}, err
			}
			return v.negated(), nil
		case "!":
			v, err := in.evalNum(e.x)
			if err != nil {
				return bcNumber{}, err
			}
			if v.isZero() {
				return bcNumInt(1), nil
			}
			return bcNumInt(0), nil
		case "++", "--":
			ref, err := in.resolve(e.x)
			if err != nil {
				return bcNumber{}, err
			}
			nv, err := in.addOne(ref.cur, e.op == "++")
			if err != nil {
				return bcNumber{}, err
			}
			return ref.set(nv)
		}
		return bcNumber{}, fmt.Errorf("unsupported operator %s", e.op)
	case *bcPostfixExpr:
		ref, err := in.resolve(e.x)
		if err != nil {
			return bcNumber{}, err
		}
		nv, err := in.addOne(ref.cur, e.op == "++")
		if err != nil {
			return bcNumber{}, err
		}
		if _, err := ref.set(nv); err != nil {
			return bcNumber{}, err
		}
		return ref.cur, nil
	case *bcAssignExpr:
		ref, err := in.resolve(e.l)
		if err != nil {
			return bcNumber{}, err
		}
		right, err := in.evalNum(e.r)
		if err != nil {
			return bcNumber{}, err
		}
		if e.op != "=" {
			right, err = in.binary(strings.TrimSuffix(e.op, "="), ref.cur, right)
			if err != nil {
				return bcNumber{}, err
			}
		}
		return ref.set(right)
	case *bcBinaryExpr:
		left, err := in.evalNum(e.l)
		if err != nil {
			return bcNumber{}, err
		}
		right, err := in.evalNum(e.r)
		if err != nil {
			return bcNumber{}, err
		}
		return in.binary(e.op, left, right)
	case *bcCallExpr:
		return in.call(e)
	}
	return bcNumber{}, fmt.Errorf("unsupported expression")
}

// evalNum evaluates an expression that must produce a number, so a void
// function call in operand position is a clear error rather than a silent
// no-op.
func (in *bcInterp) evalNum(e bcExpr) (bcNumber, error) {
	v, err := in.eval(e)
	if errors.Is(err, errBcVoid) {
		return bcNumber{}, errors.New("cannot use a void value in an expression")
	}
	return v, err
}

func (in *bcInterp) addOne(v bcNumber, up bool) (bcNumber, error) {
	if up {
		return bcAdd(v, bcNumInt(1))
	}
	return bcSub(v, bcNumInt(1))
}

func (in *bcInterp) binary(op string, l, r bcNumber) (bcNumber, error) {
	switch op {
	case "+":
		return bcAdd(l, r)
	case "-":
		return bcSub(l, r)
	case "*":
		return bcMul(l, r, in.scale)
	case "/":
		return bcDiv(l, r, in.scale)
	case "%":
		return bcMod(l, r, in.scale)
	case "^":
		return bcPow(l, r, in.scale)
	case "==", "!=", "<", "<=", ">", ">=":
		cmp := bcCompare(l, r)
		var truth bool
		switch op {
		case "==":
			truth = cmp == 0
		case "!=":
			truth = cmp != 0
		case "<":
			truth = cmp < 0
		case "<=":
			truth = cmp <= 0
		case ">":
			truth = cmp > 0
		case ">=":
			truth = cmp >= 0
		}
		if truth {
			return bcNumInt(1), nil
		}
		return bcNumInt(0), nil
	case "&&":
		if !l.isZero() && !r.isZero() {
			return bcNumInt(1), nil
		}
		return bcNumInt(0), nil
	case "||":
		if !l.isZero() || !r.isZero() {
			return bcNumInt(1), nil
		}
		return bcNumInt(0), nil
	}
	return bcNumber{}, fmt.Errorf("unsupported operator %s", op)
}

func (in *bcInterp) call(e *bcCallExpr) (bcNumber, error) {
	def, ok := in.funcs[e.name]
	if !ok {
		return bcNumber{}, fmt.Errorf("undefined function: %s()", e.name)
	}
	if len(e.args) != len(def.params) {
		return bcNumber{}, fmt.Errorf("mismatched parameters; need %d, have %d", len(def.params), len(e.args))
	}
	if len(in.frames) >= bcMaxCallDepth {
		return bcNumber{}, fmt.Errorf("call stack depth exceeded")
	}
	locals := make(map[string]bcNumber, len(def.params)+len(def.autos))
	for i, p := range def.params {
		v, err := in.evalNum(e.args[i])
		if err != nil {
			return bcNumber{}, err
		}
		locals[p] = v
	}
	for _, a := range def.autos {
		locals[a] = bcNumZero()
	}
	in.frames = append(in.frames, &bcFrame{locals: locals})
	sig, err := in.runStmts(def.body)
	in.frames = in.frames[:len(in.frames)-1]
	if err != nil {
		return bcNumber{}, err
	}
	if sig == bcSigHalt {
		return bcNumber{}, errBcHalt
	}
	if def.void {
		return bcNumber{}, errBcVoid
	}
	if sig == bcSigReturn {
		return in.retVal, nil
	}
	return bcNumZero(), nil
}

// errBcHalt unwinds the interpreter after halt.
var errBcHalt = errors.New("halt")
