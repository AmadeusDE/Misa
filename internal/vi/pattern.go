package vi

import "github.com/AmadeusDE/Misa/internal/vi/vipat"

const noPreviousPattern = "E35: No previous regular expression"

// compilePattern compiles a Vim pattern with the editor's case option as the
// default, reporting a failure in the status line.
func (e *Editor) compilePattern(pat string) (*vipat.Regexp, bool) {
	return e.compileFolded(pat, e.ignoreCase)
}

func (e *Editor) compileFolded(pat string, ignoreCase bool) (*vipat.Regexp, bool) {
	re, err := vipat.Compile(pat, ignoreCase)
	if err != nil {
		e.statusMsg = err.Error()
		return nil, false
	}
	return re, true
}

// usePattern resolves an Ex pattern (an empty one means the last search
// pattern) and, once it compiles, makes it the last search pattern as Vim does.
func (e *Editor) usePattern(pat string, ignoreCase bool) (string, *vipat.Regexp, bool) {
	if pat == "" {
		if e.searchPat == "" {
			e.statusMsg = noPreviousPattern
			return "", nil, false
		}
		pat = e.searchPat
	}
	re, ok := e.compileFolded(pat, ignoreCase)
	if !ok {
		return "", nil, false
	}
	e.searchPat = pat
	return pat, re, true
}

// unescapeQuestion turns \? into ? for a backward search, where the escaped
// delimiter is a literal question mark.
func unescapeQuestion(pat string) string {
	out := make([]byte, 0, len(pat))
	for i := 0; i < len(pat); i++ {
		if pat[i] == '\\' && i+1 < len(pat) {
			i++
			if pat[i] != '?' {
				out = append(out, '\\')
			}
		}
		out = append(out, pat[i])
	}
	return string(out)
}
