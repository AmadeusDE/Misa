// Copyright (c) 2017, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package expand

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/AmadeusDE/Misa/internal/sh/pattern"
	"github.com/AmadeusDE/Misa/internal/sh/syntax"
)

func nodeLit(node syntax.Node) string {
	if word, ok := node.(*syntax.Word); ok {
		return word.Lit()
	}
	return ""
}

// UnsetParameterError is returned when a parameter expansion encounters an
// unset variable and [Config.NoUnset] has been set.
type UnsetParameterError struct {
	Node    *syntax.ParamExp
	Message string
}

func (u UnsetParameterError) Error() string {
	return fmt.Sprintf("%s: %s", u.Node.Param.Value, u.Message)
}

func overridingUnset(pe *syntax.ParamExp) bool {
	if pe.Exp == nil {
		return false
	}
	switch pe.Exp.Op {
	case syntax.AlternateUnset, syntax.AlternateUnsetOrNull,
		syntax.DefaultUnset, syntax.DefaultUnsetOrNull,
		syntax.ErrorUnset, syntax.ErrorUnsetOrNull,
		syntax.AssignUnset, syntax.AssignUnsetOrNull:
		return true
	}
	return false
}

// paramValue is the value of a parameter expansion's variable, before any
// operator such as a default or a pattern removal is applied.
type paramValue struct {
	name             string
	vr               Variable
	orig             Variable
	index            syntax.ArithmExpr
	str              string
	elems            []string
	slice            [2]int // offset and length, when pe.Slice is set
	callVarInd       bool
	indexAllElements bool
}

func (cfg *Config) paramValue(pe *syntax.ParamExp) (paramValue, error) {
	name := pe.Param.Value
	index := pe.Index
	switch name {
	case "@", "*":
		index = &syntax.Word{Parts: []syntax.WordPart{
			&syntax.Lit{Value: name},
		}}
	}
	var vr Variable
	switch name {
	case "LINENO":
		// This is the only parameter expansion that the environment
		// interface cannot satisfy.
		line := uint64(cfg.curParam.Pos().Line())
		vr = Variable{Set: true, Kind: String, Str: strconv.FormatUint(line, 10)}
	default:
		vr = cfg.Env.Get(name)
	}
	v := paramValue{name: name, vr: vr, orig: vr, index: index}
	if n, resolved := vr.Resolve(cfg.Env); n != "" {
		v.name, v.vr = n, resolved
	}
	vr = v.vr
	if cfg.NoUnset && !vr.IsSet() && !overridingUnset(pe) {
		return v, UnsetParameterError{
			Node:    pe,
			Message: "unbound variable",
		}
	}

	if pe.Slice != nil {
		var err error
		if pe.Slice.Offset != nil {
			if v.slice[0], err = Arithm(cfg, pe.Slice.Offset); err != nil {
				return v, err
			}
		}
		if pe.Slice.Length != nil {
			if v.slice[1], err = Arithm(cfg, pe.Slice.Length); err != nil {
				return v, err
			}
		}
	}

	v.callVarInd = true
	switch nodeLit(index) {
	case "@", "*":
		switch vr.Kind {
		case Unknown:
			v.elems = nil
			v.indexAllElements = true
		case Indexed:
			v.indexAllElements = true
			v.callVarInd = false
			v.elems = cfg.sliceElems(pe, vr.List, v.name == "@" || v.name == "*")
			v.str = strings.Join(v.elems, " ")
		}
	}
	if v.callVarInd {
		var err error
		if v.str, err = cfg.varInd(vr, index); err != nil {
			return v, err
		}
	}
	if !v.indexAllElements {
		v.elems = []string{v.str}
	}
	return v, nil
}

// paramWordOp reports whether op's word becomes the value of the expansion, so
// that field splitting, quote removal and pathname expansion apply to it.
func paramWordOp(op syntax.ParExpOperator) bool {
	switch op {
	case syntax.DefaultUnset, syntax.DefaultUnsetOrNull,
		syntax.AssignUnset, syntax.AssignUnsetOrNull,
		syntax.ErrorUnset, syntax.ErrorUnsetOrNull,
		syntax.AlternateUnset, syntax.AlternateUnsetOrNull:
		return true
	}
	return false
}

// paramWordUsed reports whether pe's operator word is expanded. POSIX only
// expands the word when it is needed, and its expansion may have side effects
// such as assignments or command substitutions.
func (cfg *Config) paramWordUsed(pe *syntax.ParamExp, vr Variable, str string) bool {
	switch pe.Exp.Op {
	case syntax.AlternateUnset:
		return vr.IsSet()
	case syntax.AlternateUnsetOrNull:
		return vr.IsSet() && str != ""
	case syntax.DefaultUnset, syntax.AssignUnset, syntax.ErrorUnset:
		return !vr.IsSet()
	case syntax.DefaultUnsetOrNull, syntax.AssignUnsetOrNull, syntax.ErrorUnsetOrNull:
		return !vr.IsSet() || str == ""
	}
	return true
}

// paramWordParts expands pe's operator word keeping its quoting, for callers
// which perform field splitting on the result. ok is false when paramExp must
// handle pe instead; the word's side effects are then left to it.
func (cfg *Config) paramWordParts(pe *syntax.ParamExp) (parts []fieldPart, ok bool, err error) {
	if pe.Exp == nil || !paramWordOp(pe.Exp.Op) ||
		pe.Length || pe.Width || pe.IsSet || pe.Excl || pe.Slice != nil || pe.Repl != nil {
		return nil, false, nil
	}
	oldParam := cfg.curParam
	cfg.curParam = pe
	defer func() { cfg.curParam = oldParam }()
	v, err := cfg.paramValue(pe)
	if err != nil {
		return nil, false, err
	}
	if !cfg.paramWordUsed(pe, v.vr, v.str) {
		return nil, false, nil
	}
	parts, err = cfg.wordField(pe.Exp.Word.Parts, quoteNone)
	if err != nil {
		return nil, false, err
	}
	switch pe.Exp.Op {
	case syntax.AssignUnset, syntax.AssignUnsetOrNull:
		if err := cfg.envSet(v.name, cfg.fieldJoin(parts)); err != nil {
			return nil, false, err
		}
	case syntax.ErrorUnset, syntax.ErrorUnsetOrNull:
		return nil, false, UnsetParameterError{
			Node:    pe,
			Message: cfg.fieldJoin(parts),
		}
	}
	return parts, true, nil
}

func (cfg *Config) paramExp(pe *syntax.ParamExp) (string, error) {
	oldParam := cfg.curParam
	cfg.curParam = pe
	defer func() { cfg.curParam = oldParam }()

	v, err := cfg.paramValue(pe)
	if err != nil {
		return "", err
	}
	name, vr, orig, index := v.name, v.vr, v.orig, v.index
	str, elems := v.str, v.elems
	sliceOffset, sliceLen := v.slice[0], v.slice[1]
	callVarInd := v.callVarInd

	switch {
	case pe.Length:
		n := len(elems)
		switch nodeLit(index) {
		case "@", "*":
		default:
			n = utf8.RuneCountInString(str)
		}
		str = strconv.Itoa(n)
	case pe.Excl:
		var strs []string
		switch {
		case pe.Names != 0:
			strs = cfg.namesByPrefix(pe.Param.Value)
		case orig.Kind == NameRef:
			strs = append(strs, orig.Str)
		case pe.Index != nil && vr.Kind == Indexed:
			for i, e := range vr.List {
				if e != "" {
					strs = append(strs, strconv.Itoa(i))
				}
			}
		case pe.Index != nil && vr.Kind == Associative:
			strs = slices.AppendSeq(strs, maps.Keys(vr.Map))
		case !vr.IsSet():
			return "", fmt.Errorf("invalid indirect expansion")
		case str == "":
			return "", nil
		default:
			vr = cfg.Env.Get(str)
			strs = append(strs, vr.String())
		}
		slices.Sort(strs)
		str = strings.Join(strs, " ")
	case pe.Width:
		return "", fmt.Errorf("unsupported")
	case pe.IsSet:
		return "", fmt.Errorf("unsupported")
	case pe.Slice != nil:
		if callVarInd {
			slicePos := func(n int) int {
				if n < 0 {
					n = len(str) + n
					if n < 0 {
						n = len(str)
					}
				} else if n > len(str) {
					n = len(str)
				}
				return n
			}
			if pe.Slice.Offset != nil {
				str = str[slicePos(sliceOffset):]
			}
			if pe.Slice.Length != nil {
				str = str[:slicePos(sliceLen)]
			}
		} // else, elems are already sliced
	case pe.Repl != nil:
		orig, err := Pattern(cfg, pe.Repl.Orig)
		if err != nil {
			return "", err
		}
		if orig == "" {
			break // nothing to replace
		}
		with, err := Literal(cfg, pe.Repl.With)
		if err != nil {
			return "", err
		}
		n := 1
		if pe.Repl.All {
			n = -1
		}
		locs := findAllIndex(orig, str, n)
		sb := cfg.strBuilder()
		last := 0
		for _, loc := range locs {
			sb.WriteString(str[last:loc[0]])
			sb.WriteString(with)
			last = loc[1]
		}
		sb.WriteString(str[last:])
		str = sb.String()
	case pe.Exp != nil:
		// POSIX expands the operator word only when it is needed.
		if !cfg.paramWordUsed(pe, vr, str) {
			return str, nil
		}
		var arg string
		var err error
		switch pe.Exp.Op {
		case syntax.RemSmallPrefix, syntax.RemLargePrefix,
			syntax.RemSmallSuffix, syntax.RemLargeSuffix:
			// Quoted characters in a removal pattern are literal.
			arg, err = Pattern(cfg, pe.Exp.Word)
		default:
			arg, err = Literal(cfg, pe.Exp.Word)
		}
		if err != nil {
			return "", err
		}
		switch op := pe.Exp.Op; op {
		case syntax.AlternateUnsetOrNull:
			if str == "" {
				break
			}
			fallthrough
		case syntax.AlternateUnset:
			if vr.IsSet() {
				str = arg
			}
		case syntax.DefaultUnset:
			if vr.IsSet() {
				break
			}
			fallthrough
		case syntax.DefaultUnsetOrNull:
			if str == "" {
				str = arg
			}
		case syntax.ErrorUnset:
			if vr.IsSet() {
				break
			}
			fallthrough
		case syntax.ErrorUnsetOrNull:
			if str == "" {
				return "", UnsetParameterError{
					Node:    pe,
					Message: arg,
				}
			}
		case syntax.AssignUnset:
			if vr.IsSet() {
				break
			}
			fallthrough
		case syntax.AssignUnsetOrNull:
			if str == "" {
				if err := cfg.envSet(name, arg); err != nil {
					return "", err
				}
				str = arg
			}
		case syntax.RemSmallPrefix, syntax.RemLargePrefix,
			syntax.RemSmallSuffix, syntax.RemLargeSuffix:
			suffix := op == syntax.RemSmallSuffix || op == syntax.RemLargeSuffix
			small := op == syntax.RemSmallPrefix || op == syntax.RemSmallSuffix
			for i, elem := range elems {
				elems[i] = removePattern(elem, arg, suffix, small)
			}
			str = strings.Join(elems, " ")
		case syntax.UpperFirst, syntax.UpperAll,
			syntax.LowerFirst, syntax.LowerAll:

			caseFunc := unicode.ToLower
			if op == syntax.UpperFirst || op == syntax.UpperAll {
				caseFunc = unicode.ToUpper
			}
			all := op == syntax.UpperAll || op == syntax.LowerAll

			// empty string means '?'; nothing to do there
			expr, err := pattern.Regexp(arg, 0)
			if err != nil {
				return str, nil
			}
			rx := regexp.MustCompile(expr)

			for i, elem := range elems {
				rs := []rune(elem)
				for ri, r := range rs {
					if rx.MatchString(string(r)) {
						rs[ri] = caseFunc(r)
						if !all {
							break
						}
					}
				}
				elems[i] = string(rs)
			}
			str = strings.Join(elems, " ")
		case syntax.OtherParamOps:
			switch arg {
			case "Q":
				str, err = syntax.Quote(str, syntax.LangBash)
				if err != nil {
					// Is this even possible? If a user runs into this panic,
					// it's most likely a bug we need to fix.
					panic(err)
				}
			case "E":
				tail := str
				var rns []rune
				for tail != "" {
					var rn rune
					rn, _, tail, _ = strconv.UnquoteChar(tail, 0)
					rns = append(rns, rn)
				}
				str = string(rns)
			case "a":
				// ${var@a} returns variable attribute flags.
				// We use orig (before nameref resolve) for the attributes.
				str = orig.Flags()
			case "A":
				// ${var@A} returns a declare statement that recreates the variable.
				flags := orig.Flags()
				quoted, err := syntax.Quote(str, syntax.LangBash)
				if err != nil {
					return "", err
				}
				if flags == "" {
					str = fmt.Sprintf("%s=%s", name, quoted)
				} else {
					str = fmt.Sprintf("declare -%s %s=%s", flags, name, quoted)
				}
			case "P":
				// TODO: implement prompt expansion (\u, \h, \w, etc.).
			default:
				panic(fmt.Sprintf("unexpected @%s param expansion", arg))
			}
		}
	}
	return str, nil
}

func removePattern(str, pat string, fromEnd, shortest bool) string {
	var mode pattern.Mode
	if shortest {
		mode |= pattern.Shortest
	}
	expr, err := pattern.Regexp(pat, mode)
	if err != nil {
		return str
	}
	switch {
	case fromEnd && shortest:
		// use .* to get the right-most shortest match
		expr = ".*(" + expr + ")$"
	case fromEnd:
		// simple suffix
		expr = "(" + expr + ")$"
	default:
		// simple prefix
		expr = "^(" + expr + ")"
	}
	// no need to check error as Translate returns one
	rx := regexp.MustCompile(expr)
	if loc := rx.FindStringSubmatchIndex(str); loc != nil {
		// remove the original pattern (the submatch)
		str = str[:loc[2]] + str[loc[3]:]
	}
	return str
}

func (cfg *Config) varInd(vr Variable, idx syntax.ArithmExpr) (string, error) {
	if idx == nil {
		return vr.String(), nil
	}
	switch vr.Kind {
	case String:
		n, err := Arithm(cfg, idx)
		if err != nil {
			return "", err
		}
		if n == 0 {
			return vr.Str, nil
		}
	case Indexed:
		switch nodeLit(idx) {
		case "*", "@":
			return strings.Join(vr.List, " "), nil
		}
		i, err := Arithm(cfg, idx)
		if err != nil {
			return "", err
		}
		if i < 0 {
			return "", fmt.Errorf("negative array index")
		}
		if i < len(vr.List) {
			return vr.List[i], nil
		}
	case Associative:
		switch lit := nodeLit(idx); lit {
		case "@", "*":
			strs := slices.Sorted(maps.Values(vr.Map))
			if lit == "*" {
				return cfg.ifsJoin(strs), nil
			}
			return strings.Join(strs, " "), nil
		}
		val, err := Literal(cfg, idx.(*syntax.Word))
		if err != nil {
			return "", err
		}
		return vr.Map[val], nil
	}
	return "", nil
}

func (cfg *Config) namesByPrefix(prefix string) []string {
	var names []string
	for name := range cfg.Env.Each {
		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	return names
}
