// Behaviour adapted from uutils diffutils (MIT/Apache-2.0).
package builtins

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type diffOptions struct {
	contextPOSIXHeaders                                 bool
	style                                               string
	context, width, tabsize                             int
	brief, identical                                    bool
	expandTabs, recursive                               bool
	ignoreCase, ignoreSpace, ignoreAllSpace             bool
	suppressCommon, leftColumn                          bool
	newFile, unidirectional, stripCR                    bool
	ignoreBlank, ignoreTabs, ignoreTrailing, initialTab bool
	labels, excludes, excludeFiles                      []string
	ignoreMatching, functions                           []diffPattern
	color, commandFlags                                 string
}

var diffSpec = func() *Spec {
	s, err := ParseSpec("(normal)u(unified)c(context)U:C:e(ed)y(side-by-side)W(width):t(expand-tabs)(tabsize):q(brief)s(report-identical-files)a(text)r(recursive)i(ignore-case)b(ignore-space-change)w(ignore-all-space)(suppress-common-lines)(left-column)N(new-file)(unidirectional-new-file)p(show-c-function)F(show-function-line):L(label):B(ignore-blank-lines)I(ignore-matching-lines):n(rcs)(strip-trailing-cr)E(ignore-tab-expansion)Z(ignore-trailing-space)d(minimal)T(initial-tab)(color):?x(exclude):X(exclude-from):")
	if err != nil {
		panic(err)
	}
	return s
}()

var diffStylePattern = regexp.MustCompile(`^-(?:([cuCU])([0-9]+)|([0-9]+)([cu]?))$`)

func diffParseArgs(args []string) (diffOptions, *ParseResult, error) {
	o := diffOptions{style: "normal", context: 3, width: 130, tabsize: 8}
	var canonical []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--color" {
			arg = "--color=auto"
		}
		if arg == "--" {
			canonical = append(canonical, args[i:]...)
			break
		}
		if arg == "-C" || arg == "-U" {
			if i+1 >= len(args) {
				return o, nil, fmt.Errorf("invalid context length ''")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				return o, nil, fmt.Errorf("invalid context length '%s'", args[i])
			}
			canonical = append(canonical, arg, args[i])
			continue
		}
		if strings.HasPrefix(arg, "--context=") || strings.HasPrefix(arg, "--unified=") {
			name, value, _ := strings.Cut(arg, "=")
			flag := "-C"
			if name == "--unified" {
				flag = "-U"
			}
			if value == "" {
				value = "3"
			}
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 {
				return o, nil, fmt.Errorf("invalid context length '%s'", value)
			}
			canonical = append(canonical, flag, value)
			continue
		}
		if m := diffStylePattern.FindStringSubmatch(arg); m != nil {
			flag, num := m[1], m[2]
			if num == "" {
				flag, num = m[4], m[3]
			}
			if flag == "u" || flag == "U" {
				canonical = append(canonical, "-U", num)
			} else if flag == "c" || flag == "C" {
				canonical = append(canonical, "-C", num)
			} else {
				n, err := strconv.Atoi(num)
				if err != nil {
					return o, nil, fmt.Errorf("invalid context length '%s'", num)
				}
				o.context = n
			}
			continue
		}
		canonical = append(canonical, arg)
		if diffOptionValue(arg) && i+1 < len(args) {
			i++
			canonical = append(canonical, args[i])
		}
	}
	flags, err := diffSpec.ParseArgs(canonical)
	if err != nil {
		message := err.Error()
		if strings.HasPrefix(message, "unknown option ") {
			return o, nil, fmt.Errorf("unrecognized option '%s'", strings.TrimPrefix(message, "unknown option "))
		}
		return o, nil, err
	}
	style := ""
	for _, key := range flags.Order() {
		next := ""
		switch key {
		case "normal":
			next = "normal"
		case "u", "U":
			next = "unified"
		case "c", "C":
			next = "context"
		case "e":
			next = "ed"
		case "y":
			next = "side"
		case "n":
			next = "rcs"
		}
		if next != "" {
			if style != "" && style != next {
				return o, nil, fmt.Errorf("Conflicting output style options")
			}
			style = next
		}
		if key == "U" || key == "C" {
			value, _ := flags.String(key)
			n, e := strconv.Atoi(value)
			if e != nil || n < 0 {
				return o, nil, fmt.Errorf("invalid context length '%s'", value)
			}
			o.context = n
		}
	}
	if style != "" {
		o.style = style
	} else if flags.Bool("p") {
		o.style = "context"
	}
	for _, item := range []struct {
		key, label string
		target     *int
	}{{"W", "width", &o.width}, {"tabsize", "tabsize", &o.tabsize}} {
		if value, ok := flags.String(item.key); ok {
			n, e := strconv.Atoi(value)
			if e != nil || n <= 0 {
				return o, nil, fmt.Errorf("invalid %s «%s»", item.label, value)
			}
			*item.target = n
		}
	}
	o.newFile = flags.Bool("N")
	o.unidirectional = flags.Bool("unidirectional-new-file")
	o.stripCR = flags.Bool("strip-trailing-cr")
	o.ignoreBlank = flags.Bool("B")
	o.ignoreTabs = flags.Bool("E")
	o.ignoreTrailing = flags.Bool("Z")
	o.initialTab = flags.Bool("T")
	o.labels = flags.AllStrings("L")
	if len(o.labels) > 2 {
		return o, nil, fmt.Errorf("too many file label options")
	}
	o.excludes = flags.AllStrings("x")
	o.excludeFiles = flags.AllStrings("X")
	for _, pattern := range flags.AllStrings("I") {
		re, err := diffRegexp(pattern)
		if err != nil {
			return o, nil, err
		}
		o.ignoreMatching = append(o.ignoreMatching, re)
	}
	for _, pattern := range flags.AllStrings("F") {
		re, err := diffRegexp(pattern)
		if err != nil {
			return o, nil, err
		}
		o.functions = append(o.functions, re)
	}
	if flags.Bool("p") {
		o.functions = append(o.functions, diffPattern{linear: regexp.MustCompile(`^[a-zA-Z_$]`)})
	}
	if flags.Bool("color") {
		o.color = "auto"
		if value, ok := flags.ExplicitString("color"); ok {
			o.color = value
		}
		if o.color != "never" && o.color != "auto" && o.color != "always" {
			return o, nil, fmt.Errorf("invalid argument '%s' for 'color'", o.color)
		}
	}
	o.brief = flags.Bool("q")
	o.identical = flags.Bool("s")
	o.expandTabs = flags.Bool("t")
	o.recursive = flags.Bool("r")
	o.ignoreCase = flags.Bool("i")
	o.ignoreSpace = flags.Bool("b")
	o.ignoreAllSpace = flags.Bool("w")
	o.suppressCommon = flags.Bool("suppress-common-lines")
	o.leftColumn = flags.Bool("left-column")
	if !flags.HelpRequested && !flags.VersionRequested && len(flags.Operands) != 2 {
		return o, nil, fmt.Errorf("Usage: diff <from> <to>")
	}
	return o, flags, nil
}
