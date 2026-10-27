// Behaviour tests ported from uutils diffutils (MIT/Apache-2.0).
package builtins

import (
	"reflect"
	"strings"
	"testing"
)

func TestDiffParams(t *testing.T) {
	defaults := diffOptions{style: "normal", context: 3, width: 130, tabsize: 8}
	check := func(t *testing.T, args []string, want diffOptions, operands []string, bad bool) {
		t.Helper()
		o, f, err := diffParseArgs(args)
		if bad {
			if err == nil {
				t.Fatalf("accepted %q", args)
			}
			return
		}
		if err != nil {
			t.Fatalf("%q: %v", args, err)
		}
		if !reflect.DeepEqual(o, want) {
			t.Fatalf("%q: %+v != %+v", args, o, want)
		}
		if strings.Join(f.Operands, "\x00") != strings.Join(operands, "\x00") {
			t.Fatalf("operands %q", f.Operands)
		}
	}
	t.Run("basics", func(t *testing.T) { check(t, []string{"foo", "bar"}, defaults, []string{"foo", "bar"}, false) })
	t.Run("basics_ed", func(t *testing.T) {
		o := defaults
		o.style = "ed"
		for _, flag := range []string{"-e", "--ed"} {
			check(t, []string{flag, "foo", "bar"}, o, []string{"foo", "bar"}, false)
		}
	})
	for _, style := range []string{"context", "unified"} {
		short, upper := "c", "C"
		if style == "unified" {
			short, upper = "u", "U"
		}
		t.Run(style+"_valid", func(t *testing.T) {
			for _, args := range [][]string{{"-" + short}, {"--" + style}, {"--" + style + "="}, {"-" + short + "42"}, {"-" + upper + "42"}, {"-" + upper, "42"}, {"--" + style + "=42"}, {"-42" + short}} {
				o := defaults
				o.style = style
				if strings.Contains(strings.Join(args, ""), "42") {
					o.context = 42
				}
				check(t, append(args, "foo", "bar"), o, []string{"foo", "bar"}, false)
			}
		})
		t.Run(style+"_invalid", func(t *testing.T) {
			for _, args := range [][]string{{"-" + short, "42"}, {"-" + short + "=42"}, {"-" + short + "="}, {"-" + upper}, {"-" + upper + "=42"}, {"-" + upper + "="}, {"--" + style + "42"}, {"--" + style, "42"}, {"-42" + upper}} {
				check(t, append(args, "foo", "bar"), defaults, nil, true)
			}
		})
	}
	t.Run("context_count", func(t *testing.T) {
		for _, arg := range []string{"-u54", "-U54", "-c54"} {
			o := defaults
			o.context = 54
			o.style = "unified"
			if arg == "-c54" {
				o.style = "context"
			}
			check(t, []string{arg, "foo", "bar"}, o, []string{"foo", "bar"}, false)
		}
	})
	for _, tc := range []struct {
		name  string
		flags []string
		set   func(*diffOptions)
	}{
		{"report_identical_files", []string{"-s", "--report-identical-files"}, func(o *diffOptions) { o.identical = true }},
		{"brief", []string{"-q", "--brief"}, func(o *diffOptions) { o.brief = true }},
		{"expand_tabs", []string{"-t", "--expand-tabs"}, func(o *diffOptions) { o.expandTabs = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, flag := range tc.flags {
				o := defaults
				tc.set(&o)
				check(t, []string{flag, "foo", "bar"}, o, []string{"foo", "bar"}, false)
			}
		})
	}
	t.Run("tabsize", func(t *testing.T) {
		for _, value := range []string{"1", "4", "7"} {
			o := defaults
			o.tabsize = int(value[0] - '0')
			check(t, []string{"--tabsize=" + value, "foo", "bar"}, o, []string{"foo", "bar"}, false)
		}
		for _, value := range []string{"0", "999999999999999999999999", "xxx", "-1", ""} {
			check(t, []string{"--tabsize=" + value, "foo", "bar"}, defaults, nil, true)
		}
	})
	t.Run("width", func(t *testing.T) {
		o := defaults
		o.width = 100
		check(t, []string{"--width=100", "foo", "bar"}, o, []string{"foo", "bar"}, false)
	})
	t.Run("width_suffix_is_an_operand", func(t *testing.T) {
		check(t, []string{"xyz--width=5", "foo"}, defaults, []string{"xyz--width=5", "foo"}, false)
	})
	t.Run("width_non_utf8_is_not_an_option", func(t *testing.T) {
		check(t, []string{"\xff--width=5", "foo"}, defaults, []string{"\xff--width=5", "foo"}, false)
	})
	t.Run("double_dash", func(t *testing.T) { check(t, []string{"--", "-g", "-h"}, defaults, []string{"-g", "-h"}, false) })
	t.Run("default_to_stdin", func(t *testing.T) {
		for _, args := range [][]string{{"foo", "-"}, {"-", "bar"}, {"-", "-"}} {
			check(t, args, defaults, args, false)
		}
		for _, args := range [][]string{{"foo", "bar", "-"}, {"-", "-", "-"}} {
			check(t, args, defaults, nil, true)
		}
	})
	t.Run("missing_arguments", func(t *testing.T) {
		for _, args := range [][]string{nil, {"foo"}} {
			check(t, args, defaults, nil, true)
		}
	})
	t.Run("unknown_argument", func(t *testing.T) {
		for _, args := range [][]string{{"-g", "foo", "bar"}, {"-g", "bar"}, {"-g"}} {
			check(t, args, defaults, nil, true)
		}
	})
	t.Run("empty", func(t *testing.T) { check(t, nil, defaults, nil, true) })
	t.Run("conflicting_output_styles", func(t *testing.T) {
		for _, args := range [][]string{{"-u", "-c"}, {"-u", "-e"}, {"-c", "-u"}, {"-c", "-U42"}, {"-u", "--normal"}, {"--normal", "-e"}, {"--context", "--normal"}} {
			check(t, append(args, "foo", "bar"), defaults, nil, true)
		}
	})
}

func TestDiffAdditionalArgs(t *testing.T) {
	for _, args := range [][]string{{"-42", "-u", "a", "b"}, {"-aiw", "a", "b"}, {"-U", "0", "-", "b"}} {
		if _, _, err := diffParseArgs(args); err != nil {
			t.Error(err)
		}
	}
	for _, args := range [][]string{{"-W0", "a", "b"}, {"--tabsize=0", "a", "b"}} {
		if _, _, err := diffParseArgs(args); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
	if _, _, err := diffParseArgs([]string{"--foobar", "a", "b"}); err == nil || !strings.Contains(err.Error(), "unrecognized option '--foobar'") {
		t.Fatal(err)
	}
}
