package awk

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"strings"

	"github.com/AmadeusDE/Misa/internal/posixre"
)

// Config describes one awk invocation.
type Config struct {
	Program  string
	Operands []string // files and var=value assignments
	Assign   []string // -v assignments
	FS       *string  // -F
	Bytes    bool     // count bytes instead of characters
	Dir      string   // relative operand files resolve here
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
	// Done, when closed, stops the program at its next record or loop turn.
	Done <-chan struct{}
}

type control uint8

const (
	ctlNone control = iota
	ctlBreak
	ctlContinue
	ctlNext
	ctlNextFile
	ctlExit
)

type interp struct {
	cfg  Config
	prog *Program
	vars []Value
	r    record
	in   inputs
	out  *bufio.Writer

	fs, ofs, ors, rs, subsep, convfmt, ofmt string

	reCache  map[string]*posixre.Regexp
	rng      *rand.Rand
	seed     float64
	status   int
	exitCode int
	steps    int
}

type canceled struct{}

// Run executes the program and returns the exit status: the value of exit,
// 2 for syntax and runtime errors.
func Run(cfg Config) (status int) {
	prog, err := Parse(cfg.Program)
	if err != nil {
		fmt.Fprintf(cfg.Stderr, "awk: %v\n", err)
		return 2
	}
	in := newInterp(cfg, prog)
	defer func() {
		if r := recover(); r != nil {
			in.out.Flush()
			switch e := r.(type) {
			case *Error:
				fmt.Fprintf(cfg.Stderr, "awk: %v\n", e)
				status = 2
			case canceled:
				fmt.Fprintln(cfg.Stderr, "awk: canceled")
				status = 1
			default:
				panic(r)
			}
		}
	}()
	if msg := in.setup(); msg != "" {
		fmt.Fprintf(cfg.Stderr, "awk: %s\n", msg)
		return 2
	}
	in.run()
	in.out.Flush()
	if in.exitCode != 0 {
		return in.exitCode
	}
	return in.status
}

func newInterp(cfg Config, prog *Program) *interp {
	in := &interp{cfg: cfg, prog: prog, vars: make([]Value, prog.nvars), seed: 1}
	in.out = bufio.NewWriterSize(cfg.Stdout, 32<<10)
	in.reCache = map[string]*posixre.Regexp{}
	in.rng = rand.New(rand.NewSource(1))
	in.vars[vFS] = str(" ")
	in.vars[vOFS] = str(" ")
	in.vars[vORS] = str("\n")
	in.vars[vRS] = str("\n")
	in.vars[vSUBSEP] = str("\x1c")
	in.vars[vCONVFMT] = str("%.6g")
	in.vars[vOFMT] = str("%.6g")
	in.vars[vRSTART] = num(0)
	in.vars[vRLENGTH] = num(-1)
	in.vars[vNR] = num(0)
	in.vars[vFNR] = num(0)
	in.vars[vARGC] = num(float64(len(cfg.Operands) + 1))
	in.refreshCaches()
	in.in.operands = cfg.Operands
	in.setRecord("")
	return in
}

func (in *interp) refreshCaches() {
	in.fs = in.vars[vFS].s
	in.ofs = in.vars[vOFS].s
	in.ors = in.vars[vORS].s
	in.rs = in.vars[vRS].s
	in.subsep = in.vars[vSUBSEP].s
	in.convfmt = in.vars[vCONVFMT].s
	in.ofmt = in.vars[vOFMT].s
}

// setup applies -F and -v. It returns an error message, or "".
func (in *interp) setup() string {
	if in.cfg.FS != nil {
		in.setVar(vFS, str(unescape(*in.cfg.FS)))
	}
	for _, a := range in.cfg.Assign {
		name, value, ok := isAssignment(a)
		if !ok {
			return fmt.Sprintf("invalid -v argument %q: want var=value", a)
		}
		in.assignOperand(name, value)
	}
	return ""
}

func (in *interp) assignOperand(name, value string) {
	if i, ok := in.prog.varIndex(name); ok {
		in.setVar(i, input(unescape(value)))
	}
}

func (in *interp) warnf(format string, args ...any) {
	in.out.Flush()
	fmt.Fprintf(in.cfg.Stderr, "awk: "+format+"\n", args...)
}

func (in *interp) tick() {
	in.steps++
	if in.steps&0xfff != 0 || in.cfg.Done == nil {
		return
	}
	select {
	case <-in.cfg.Done:
		panic(canceled{})
	default:
	}
}

func (in *interp) run() {
	exited := in.runBegin()
	if !exited && (len(in.prog.rules) > 0 || len(in.prog.ends) > 0) {
		in.runRecords()
	}
	in.skipFile()
	in.runEnd()
}

func (in *interp) runBegin() bool {
	for _, body := range in.prog.begins {
		if in.execList(body) == ctlExit {
			return true
		}
	}
	return false
}

func (in *interp) runRecords() {
	for in.nextRecord() {
		in.tick()
		for _, r := range in.prog.rules {
			c := in.runRule(r)
			if c == ctlExit {
				return
			}
			if c == ctlNextFile {
				in.skipFile()
			}
			if c == ctlNext || c == ctlNextFile {
				break
			}
		}
	}
}

func (in *interp) runEnd() {
	for _, body := range in.prog.ends {
		if in.execList(body) == ctlExit {
			return
		}
	}
}

func (in *interp) runRule(r *rule) control {
	if !in.ruleMatches(r) {
		return ctlNone
	}
	if r.action == nil {
		in.printRecord()
		return ctlNone
	}
	return in.execList(r.action)
}

func (in *interp) ruleMatches(r *rule) bool {
	switch {
	case r.pattern == nil:
		return true
	case !r.ranged:
		return in.eval(r.pattern).truth()
	case r.inRange:
		if in.eval(r.r2).truth() {
			r.inRange = false
		}
		return true
	case in.eval(r.pattern).truth():
		r.inRange = !in.eval(r.r2).truth()
		return true
	}
	return false
}

func (in *interp) printRecord() {
	in.ensureRecord()
	in.out.WriteString(in.r.rec)
	in.out.WriteString(in.ors)
}

func (in *interp) execList(list []stmt) control {
	for _, s := range list {
		if c := in.exec(s); c != ctlNone {
			return c
		}
	}
	return ctlNone
}

func (in *interp) exec(s stmt) control {
	switch s := s.(type) {
	case *exprStmt:
		in.eval(s.e)
	case *printStmt:
		in.execPrint(s)
	case *blockStmt:
		return in.execList(s.list)
	case *ifStmt:
		if in.eval(s.cond).truth() {
			return in.exec(s.then)
		} else if s.els != nil {
			return in.exec(s.els)
		}
	case *whileStmt:
		for in.eval(s.cond).truth() {
			in.tick()
			if c := in.exec(s.body); c == ctlBreak {
				break
			} else if c != ctlNone && c != ctlContinue {
				return c
			}
		}
	case *doStmt:
		for {
			in.tick()
			if c := in.exec(s.body); c == ctlBreak {
				break
			} else if c != ctlNone && c != ctlContinue {
				return c
			}
			if !in.eval(s.cond).truth() {
				break
			}
		}
	case *forStmt:
		return in.execFor(s)
	case *nextStmt:
		return ctlNext
	case *nextFileStmt:
		return ctlNextFile
	case *breakStmt:
		return ctlBreak
	case *continueStmt:
		return ctlContinue
	case *exitStmt:
		if s.e != nil {
			in.exitCode = int(in.eval(s.e).float())
		}
		return ctlExit
	}
	return ctlNone
}

func (in *interp) execFor(s *forStmt) control {
	if s.init != nil {
		in.exec(s.init)
	}
	for s.cond == nil || in.eval(s.cond).truth() {
		in.tick()
		if c := in.exec(s.body); c == ctlBreak {
			break
		} else if c != ctlNone && c != ctlContinue {
			return c
		}
		if s.post != nil {
			in.exec(s.post)
		}
	}
	return ctlNone
}

func (in *interp) outputText(v Value) string {
	if v.k == kNum {
		return numToString(v.n, in.ofmt)
	}
	return v.s
}

func (in *interp) execPrint(s *printStmt) {
	if s.printf {
		format := in.eval(s.args[0]).text(in.convfmt)
		in.out.WriteString(sprintf(format, in.evalAll(s.args[1:]), in.cfg.Bytes, in.convfmt))
		return
	}
	if len(s.args) == 0 {
		in.printRecord()
		return
	}
	for i, a := range s.args {
		if i > 0 {
			in.out.WriteString(in.ofs)
		}
		in.out.WriteString(in.outputText(in.eval(a)))
	}
	in.out.WriteString(in.ors)
}

func (in *interp) evalAll(args []expr) []Value {
	vals := make([]Value, len(args))
	for i, a := range args {
		vals[i] = in.eval(a)
	}
	return vals
}

// setVar stores a variable, keeping the cached special values current.
// FS and OFS changes first settle the record under the old value.
func (in *interp) setVar(idx int, v Value) {
	switch idx {
	case vNF:
		in.setNF(int(v.float()))
		return
	case vFS:
		in.ensureFields()
	case vOFS:
		in.ensureRecord()
	}
	in.vars[idx] = v
	if idx < numSpecial {
		in.vars[idx] = in.specialValue(idx, v)
		in.refreshCaches()
	}
}

// specialValue keeps string-valued specials as strings, as the caches read s.
func (in *interp) specialValue(idx int, v Value) Value {
	switch idx {
	case vFS, vOFS, vORS, vRS, vSUBSEP, vCONVFMT, vOFMT:
		return Value{k: v.k, n: v.n, s: v.text(in.convfmt)}
	}
	return v
}

func (in *interp) getVar(idx int) Value {
	if idx == vNF {
		return num(float64(in.nf()))
	}
	return in.vars[idx]
}

// ref is a resolved assignment target: a variable or a field.
type ref struct {
	field bool
	idx   int
}

func (in *interp) resolve(lv expr) ref {
	switch lv := lv.(type) {
	case *varRef:
		return ref{idx: lv.idx}
	case *fieldRef:
		return ref{field: true, idx: in.fieldIndex(lv)}
	}
	runtimeErrorf("assignment to a non-variable")
	return ref{}
}

func (in *interp) fieldIndex(f *fieldRef) int {
	n := in.eval(f.index).float()
	if n < 0 || n != n {
		runtimeErrorf("trying to access out of range field %s", numToString(n, "%.6g"))
	}
	if n > maxFields {
		runtimeErrorf("field index %s is too large", numToString(n, "%.6g"))
	}
	return int(n)
}

func (in *interp) load(r ref) Value {
	if r.field {
		return in.field(r.idx)
	}
	return in.getVar(r.idx)
}

func (in *interp) store(r ref, v Value) {
	if r.field {
		in.setField(r.idx, v)
		return
	}
	in.setVar(r.idx, v)
}

func (in *interp) regex(src string) *posixre.Regexp {
	if re, ok := in.reCache[src]; ok {
		return re
	}
	re, err := posixre.Compile(translateRegex(src), posixre.Options{Syntax: posixre.Extended})
	if err != nil {
		runtimeErrorf("invalid regular expression /%s/: %v", src, err)
	}
	if len(in.reCache) >= 256 {
		clear(in.reCache)
	}
	in.reCache[src] = re
	return re
}

func (in *interp) regexOf(e expr) *posixre.Regexp {
	if r, ok := e.(*regexLit); ok {
		return in.regex(r.src)
	}
	return in.regex(in.eval(e).text(in.convfmt))
}

func (in *interp) matches(re *posixre.Regexp, s string) bool {
	ok, err := re.Match([]byte(s))
	if err != nil {
		runtimeErrorf("regular expression error: %v", err)
	}
	return ok
}

func (in *interp) eval(e expr) Value {
	switch e := e.(type) {
	case *numLit:
		return e.v
	case *strLit:
		return e.v
	case *regexLit:
		in.ensureRecord()
		return boolVal(in.matches(in.regex(e.src), in.r.rec))
	case *varRef:
		return in.getVar(e.idx)
	case *fieldRef:
		return in.field(in.fieldIndex(e))
	case *groupExpr:
		runtimeErrorf("syntax error: unexpected expression list")
	case *unaryExpr:
		x := in.eval(e.e).float()
		if e.op == tMinus {
			x = -x
		}
		return num(x)
	case *binaryExpr:
		return num(arith(e.op, in.eval(e.l).float(), in.eval(e.r).float()))
	case *cmpExpr:
		return boolVal(compareResult(e.op, compareValues(in.eval(e.l), in.eval(e.r), in.convfmt)))
	case *matchExpr:
		s := in.eval(e.l).text(in.convfmt)
		return boolVal(in.matches(in.regexOf(e.r), s) != e.negate)
	case *concatExpr:
		l := in.eval(e.l).text(in.convfmt)
		return str(l + in.eval(e.r).text(in.convfmt))
	case *andExpr:
		return boolVal(in.eval(e.l).truth() && in.eval(e.r).truth())
	case *orExpr:
		return boolVal(in.eval(e.l).truth() || in.eval(e.r).truth())
	case *notExpr:
		return boolVal(!in.eval(e.e).truth())
	case *condExpr:
		if in.eval(e.c).truth() {
			return in.eval(e.a)
		}
		return in.eval(e.b)
	case *assignExpr:
		return in.evalAssign(e)
	case *incDecExpr:
		r := in.resolve(e.lv)
		old := in.load(r).float()
		in.store(r, num(old+e.delta))
		if e.pre {
			return num(old + e.delta)
		}
		return num(old)
	case *callExpr:
		return in.call(e)
	}
	panic(fmt.Sprintf("awk: unknown expression %T", e))
}

func (in *interp) evalAssign(e *assignExpr) Value {
	r := in.resolve(e.lv)
	v := in.eval(e.rhs)
	if e.op != tAssign {
		v = num(arith(compoundOp[e.op], in.load(r).float(), v.float()))
	}
	in.store(r, v)
	return v
}

var compoundOp = map[tokKind]tokKind{
	tAddAssign: tPlus, tSubAssign: tMinus, tMulAssign: tStar,
	tDivAssign: tSlash, tModAssign: tPercent, tPowAssign: tCaret,
}

func compareResult(op tokKind, c int) bool {
	switch op {
	case tLT:
		return c < 0
	case tLe:
		return c <= 0
	case tEq:
		return c == 0
	case tNe:
		return c != 0
	case tGT:
		return c > 0
	}
	return c >= 0
}

func unescape(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			i++
			continue
		}
		i = decodeEscape(s, i+1, &b)
	}
	return b.String()
}
