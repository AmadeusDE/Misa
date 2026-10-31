package builtins

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/AmadeusDE/Misa/internal/posixre"
)

type grepMatcher interface {
	Find(b []byte, pos int) ([]int, error)
	Match(b []byte) (bool, error)
}

// grepCompile picks a matcher. Several patterns that are all plain text use
// a hash lookup per length, which stays fast with thousands of patterns
// (grep -f wordlist) where one combined regex would be huge.
func grepCompile(patterns []string, opt posixre.Options) (grepMatcher, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	if len(patterns) > 1 && !opt.IgnoreCase && !opt.Word && grepAllLiteral(patterns, opt) {
		return newLiteralSet(patterns, opt.Line), nil
	}
	return posixre.CompileAll(patterns, opt)
}

func grepAllLiteral(patterns []string, opt posixre.Options) bool {
	if opt.Literal {
		return true
	}
	meta := `\.[*^$`
	if opt.Syntax == posixre.Extended {
		meta += "+?(){}|"
	}
	for _, p := range patterns {
		if strings.ContainsAny(p, meta) {
			return false
		}
	}
	return true
}

type literalSet struct {
	set     map[string]struct{}
	lengths []int
	line    bool
}

func newLiteralSet(patterns []string, line bool) *literalSet {
	s := &literalSet{set: map[string]struct{}{}, line: line}
	for _, p := range patterns {
		if _, dup := s.set[p]; !dup {
			s.set[p] = struct{}{}
			s.lengths = append(s.lengths, len(p))
		}
	}
	slices.Sort(s.lengths)
	s.lengths = slices.Compact(s.lengths)
	slices.Reverse(s.lengths)
	return s
}

// Find returns the leftmost-longest occurrence of any pattern.
func (s *literalSet) Find(b []byte, pos int) ([]int, error) {
	if s.line {
		if _, ok := s.set[string(b)]; ok && pos == 0 {
			return []int{0, len(b)}, nil
		}
		return nil, nil
	}
	for i := pos; i <= len(b); i++ {
		for _, n := range s.lengths {
			if i+n > len(b) {
				continue
			}
			if _, ok := s.set[string(b[i:i+n])]; ok {
				return []int{i, i + n}, nil
			}
		}
	}
	return nil, nil
}

func (s *literalSet) Match(b []byte) (bool, error) {
	loc, _ := s.Find(b, 0)
	return loc != nil, nil
}

// grepFindAll lists successive matches the way posixre.FindAll does,
// skipping an empty match right after the previous one.
func grepFindAll(m grepMatcher, b []byte) ([][]int, error) {
	var out [][]int
	prevEnd := -1
	for pos := 0; pos <= len(b); {
		loc, err := m.Find(b, pos)
		if err != nil || loc == nil {
			return out, err
		}
		if loc[0] == loc[1] {
			if loc[0] != prevEnd {
				out = append(out, loc)
			}
			_, w := utf8.DecodeRune(b[loc[1]:])
			pos = loc[1] + max(w, 1)
		} else {
			out = append(out, loc)
			pos = loc[1]
		}
		prevEnd = loc[1]
	}
	return out, nil
}
