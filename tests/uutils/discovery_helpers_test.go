package uutils

import (
	"regexp"
	"strings"
)

type functionGap struct {
	name string
	line int
	body string
}

// A gap is one source function with no represented case. Dynamic loop
// expansions are deliberately not guessed. A represented function can still
// contain unsupported statements; it is not claimed to be fully covered.
func functionGaps(src string, cases []testCase) []functionGap {
	re := regexp.MustCompile(`(?m)^\s*fn\s+(test_\w+)\s*\([^)]*\)\s*\{`)
	locs := re.FindAllStringSubmatchIndex(src, -1)
	represented := map[string]bool{}
	for _, c := range cases {
		best := ""
		for _, loc := range locs {
			name := src[loc[2]:loc[3]]
			if (c.name == name || strings.HasPrefix(c.name, name+"_")) && len(name) > len(best) {
				best = name
			}
		}
		if best != "" {
			represented[best] = true
		}
	}
	var gaps []functionGap
	for _, loc := range locs {
		name := src[loc[2]:loc[3]]
		if !represented[name] {
			end := matchParen(src, loc[1]-1)
			body := ""
			if end >= loc[1] {
				body = src[loc[1]:end]
			}
			gaps = append(gaps, functionGap{name: name, line: 1 + strings.Count(src[:loc[2]], "\n"), body: body})
		}
	}
	return gaps
}

func nativeForGap(cmd string, gap functionGap) string {
	if cmd == "chmod" && strings.Contains(gap.body, "for ") || (cmd == "cp" || cmd == "ln" || cmd == "realpath") && strings.Contains(gap.name+gap.body, "symlink") {
		return "tests/native/upstream_constructs_test.go:TestAdapterNativeConstructs"
	}
	return ""
}
