package awk

import (
	"strconv"
	"strings"
)

func (in *interp) initArrays() {
	argv := newArray()
	argv.set("0", str("awk"))
	for i, o := range in.cfg.Operands {
		argv.set(strconv.Itoa(i+1), input(o))
	}
	in.garrs[vARGV] = argv
	env := newArray()
	for _, kv := range in.cfg.Env {
		if k, v, ok := strings.Cut(kv, "="); ok && k != "" {
			env.set(k, input(v))
		}
	}
	in.garrs[vENVIRON] = env
}

// subscript builds the key of a[i, j, ...]: the parts joined by SUBSEP.
func (in *interp) subscript(subs []expr) string {
	if len(subs) == 1 {
		return in.eval(subs[0]).text(in.convfmt)
	}
	parts := make([]string, len(subs))
	for i, e := range subs {
		parts[i] = in.eval(e).text(in.convfmt)
	}
	return strings.Join(parts, in.subsep)
}

// peekArray returns the array a variable already holds, or nil.
func (in *interp) peekArray(v *varRef) *array {
	if v.local {
		return in.frame.locals[v.idx].a
	}
	return in.garrs[v.idx]
}

// arrayFor returns the array a variable names, creating it when the
// variable is still untyped.
func (in *interp) arrayFor(v *varRef) *array {
	if v.local {
		l := &in.frame.locals[v.idx]
		if l.a != nil {
			return l.a
		}
		if l.v.k != kUninit {
			runtimeErrorf("can't use scalar %s as an array", v.name)
		}
		l.a = newArray()
		if l.bind != nil {
			l.bind(l.a)
		}
		return l.a
	}
	if a := in.garrs[v.idx]; a != nil {
		return a
	}
	if v.idx < numSpecial || in.vars[v.idx].k != kUninit {
		runtimeErrorf("can't use scalar %s as an array", v.name)
	}
	a := newArray()
	in.garrs[v.idx] = a
	return a
}

// bindArray hands an array created inside a callee to the caller's variable
// that was passed untyped.
func (in *interp) bindArray(v *varRef, caller *frame, a *array) {
	if !v.local {
		in.garrs[v.idx] = a
		return
	}
	l := &caller.locals[v.idx]
	l.a = a
	if l.bind != nil {
		l.bind(a)
	}
}

func (in *interp) untyped(v *varRef) bool {
	if v.local {
		l := &in.frame.locals[v.idx]
		return l.a == nil && l.v.k == kUninit
	}
	return v.idx >= numSpecial && in.garrs[v.idx] == nil && in.vars[v.idx].k == kUninit
}

func (in *interp) callUser(c *userCall) Value {
	fn := c.fn
	if in.depth >= maxCallDepth {
		runtimeErrorf("function %s: call nesting is too deep", fn.name)
	}
	fr := &frame{locals: make([]local, len(fn.params))}
	caller := in.frame
	for i, a := range c.args {
		l := &fr.locals[i]
		if v, ok := a.(*varRef); ok {
			if arr := in.peekArray(v); arr != nil {
				l.a = arr
				continue
			}
			if in.untyped(v) {
				l.bind = func(arr *array) { in.bindArray(v, caller, arr) }
				continue
			}
		}
		l.v = in.eval(a)
	}
	in.frame = fr
	in.depth++
	ctl := in.execList(fn.body)
	in.frame = caller
	in.depth--
	switch ctl {
	case ctlExit, ctlNext, ctlNextFile:
		panic(unwind{ctl})
	case ctlReturn:
		v := in.retval
		in.retval = Value{}
		return v
	}
	return Value{}
}

// split implements split(s, a [, fs]).
func (in *interp) split(args []expr) Value {
	s := in.eval(args[0]).text(in.convfmt)
	var parts []string
	switch {
	case len(args) == 2:
		parts = in.splitBy(s, in.fs, false)
	default:
		if r, ok := args[2].(*regexLit); ok {
			parts = in.splitByRegex(s, r.src)
		} else {
			parts = in.splitBy(s, in.eval(args[2]).text(in.convfmt), false)
		}
	}
	arr := in.arrayFor(args[1].(*varRef))
	arr.clear()
	for i, p := range parts {
		arr.set(strconv.Itoa(i+1), input(p))
	}
	return num(float64(len(parts)))
}

func (in *interp) splitByRegex(s, src string) []string {
	switch {
	case s == "":
		return nil
	case src == "":
		return in.splitChars(s)
	}
	return in.splitRegex(s, src)
}
