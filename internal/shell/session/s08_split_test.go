package session

import "testing"

func TestS08FieldSplitting(t *testing.T) {
	tests := []struct {
		name, src string
		want      []string
	}{
		{"unset default", "unset IFS; x='  a\tb\nc  '; probe $x", []string{"probe", "a", "b", "c"}},
		{"explicit default", "IFS=' \t\n'; x='  a\tb\nc  '; probe $x", []string{"probe", "a", "b", "c"}},
		{"empty IFS", `IFS=; x='  a b  '; probe $x`, []string{"probe", "  a b  "}},
		{"empty IFS empty expansion", `IFS=; x=; probe $x "$x"`, []string{"probe", ""}},
		{"custom whitespace", "IFS=' \t'; x=' \ta  b\t '; probe $x", []string{"probe", "a", "b"}},
		{"custom nonwhitespace", `IFS=:; x=':a::b:'; probe $x`, []string{"probe", "", "a", "", "b"}},
		{"space not in IFS", `IFS=:; x=' a : b '; probe $x`, []string{"probe", " a ", " b "}},
		{"adjacent literal and delimiter", `IFS=' :'; x=' : '; probe pre${x}post`, []string{"probe", "pre", "post"}},
		{"adjacent empty and delimiter", `IFS=:; x=':'; probe ''$x post`, []string{"probe", "", "post"}},
		{"whitespace then nonwhite", `IFS=' :'; x='a  :  : b'; probe $x`, []string{"probe", "a", "", "b"}},
		{"adjacent expansions whitespace", `x='a '; y=' b'; probe $x$y`, []string{"probe", "a", "b"}},
		{"adjacent expansions nonwhite", `IFS=:; x='a:'; y=':b'; probe $x$y`, []string{"probe", "a", "", "b"}},
		{"adjacent expansions mixed IFS", `IFS=' :'; x='a '; y=' : b'; probe $x$y`, []string{"probe", "a", "b"}},
		{"mixed separators", `IFS=' :'; x='  : a : : b :  '; probe $x`, []string{"probe", "", "a", "", "b"}},
		{"literal is not split", `IFS=:; x='a:b'; probe a:b $x "$x" pre${x}post`, []string{"probe", "a:b", "a", "b", "a:b", "prea", "bpost"}},
		{"quoted empty survives", `IFS=:; x='::'; probe "$x" $x ''`, []string{"probe", "::", "", "", ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { s08CheckArgv(t, tt.src, nil, tt.want) })
	}
}

func TestS08PositionalExpansion(t *testing.T) {
	tests := []struct {
		name, src    string
		params, want []string
	}{
		{"quoted at", `probe "$@"`, []string{"a b", "", "c"}, []string{"probe", "a b", "", "c"}},
		{"unquoted at", `probe $@`, []string{"a b", "", "c"}, []string{"probe", "a", "b", "c"}},
		{"quoted star", `probe "$*"`, []string{"a b", "", "c"}, []string{"probe", "a b  c"}},
		{"no params quoted at", `probe "$@"`, nil, []string{"probe"}},
		{"no params unquoted at", `probe $@`, nil, []string{"probe"}},
		{"no params quoted star", `probe "$*"`, nil, []string{"probe", ""}},
		{"one empty quoted at", `probe "$@"`, []string{""}, []string{"probe", ""}},
		{"one empty unquoted at", `probe $@`, []string{""}, []string{"probe"}},
		{"one empty quoted star", `probe "$*"`, []string{""}, []string{"probe", ""}},
		{"star custom first IFS char", `IFS=:,; probe "$*"`, []string{"a", "", "b"}, []string{"probe", "a::b"}},
		{"star empty IFS", `IFS=; probe "$*"`, []string{"a", "", "b"}, []string{"probe", "ab"}},
		{"star unicode first IFS char", `IFS=é:; probe "$*"`, []string{"a", "b"}, []string{"probe", "aéb"}},
		{"unquoted star default", `probe $*`, []string{"a b", "", "c"}, []string{"probe", "a", "b", "c"}},
		{"unquoted star custom", `IFS=:; probe $*`, []string{"a:b", "c"}, []string{"probe", "a", "b", "c"}},
		{"at empty IFS", `IFS=; probe $@`, []string{"a b", "c"}, []string{"probe", "a b", "c"}},
		{"at custom concatenation", `IFS=:; probe pre${@}post`, []string{"a:b", "c:d"}, []string{"probe", "prea", "b", "c", "dpost"}},
		{"star unset IFS", `unset IFS; probe "$*"`, []string{"a", "b"}, []string{"probe", "a b"}},
		{"at custom IFS", `IFS=:; probe $@`, []string{"a:b", "", "c:d"}, []string{"probe", "a", "b", "c", "d"}},
		{"at concatenated", `probe pre"$@"post`, []string{"a", "", "b"}, []string{"probe", "prea", "", "bpost"}},
		{"at concatenated no params", `probe pre"$@"post`, nil, []string{"probe", "prepost"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { s08CheckArgv(t, tt.src, tt.params, tt.want) })
	}
}
