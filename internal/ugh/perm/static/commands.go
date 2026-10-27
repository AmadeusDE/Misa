package static

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"mvdan.cc/sh/v3/syntax"
)

// SimpleCommands splits a bash command line into the argv of each command
// it runs, for manual-mode allow rules. It accepts only plain command lines:
// simple commands joined by &&, ||, ;, newlines and pipes, whose words are
// literal text (quoted or not), with no redirection except to /dev/null or
// between fds. Anything else (variables, $(…), subshells, blocks, functions,
// loops, env assignments, backgrounding, file redirections) returns ok=false,
// so no rule can approve it.
func SimpleCommands(cmd string) (argvs [][]string, ok bool) {
	file, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(cmd), "cmd")
	if err != nil || len(file.Stmts) == 0 {
		return nil, false
	}
	var walk func(s *syntax.Stmt) bool
	walk = func(s *syntax.Stmt) bool {
		if s == nil || s.Background || s.Coprocess || s.Negated || !plainRedirects(s.Redirs) {
			return false
		}
		switch c := s.Cmd.(type) {
		case *syntax.CallExpr:
			if len(c.Assigns) > 0 || len(c.Args) == 0 {
				return false
			}
			argv := make([]string, 0, len(c.Args))
			for _, w := range c.Args {
				lit, ok := literalWord(w.Parts)
				if !ok {
					return false
				}
				argv = append(argv, lit)
			}
			argvs = append(argvs, argv)
			return true
		case *syntax.BinaryCmd:
			switch c.Op {
			case syntax.AndStmt, syntax.OrStmt, syntax.Pipe, syntax.PipeAll:
				return walk(c.X) && walk(c.Y)
			}
		}
		return false
	}
	for _, s := range file.Stmts {
		if !walk(s) {
			return nil, false
		}
	}
	return argvs, true
}

// literalWord is a word as shell source in one canonical spelling, when it
// has no expansion of any kind: unquoted text keeps its glob, tilde and
// brace characters (the shell expands them), quoted text has every special
// character backslash-escaped. So 'a b', "a b" and a\ b agree, while '*'
// and * stay different words.
func literalWord(parts []syntax.WordPart) (string, bool) {
	var b strings.Builder
	for _, p := range parts {
		switch t := p.(type) {
		case *syntax.Lit:
			b.WriteString(unquotedLit(t.Value))
		case *syntax.SglQuoted:
			if t.Dollar {
				return "", false // $'…' escapes
			}
			b.WriteString(escapeShell(t.Value))
		case *syntax.DblQuoted:
			if t.Dollar {
				return "", false
			}
			for _, q := range t.Parts {
				lit, ok := q.(*syntax.Lit)
				if !ok {
					return "", false
				}
				b.WriteString(escapeShell(dblQuotedLit.Replace(lit.Value)))
			}
		default:
			return "", false
		}
	}
	return b.String(), true
}

// dblQuotedLit undoes the escapes double quotes honour.
var dblQuotedLit = strings.NewReplacer(`\\`, `\`, `\"`, `"`, `\$`, `$`, "\\`", "`", "\\\n", "")

// unquotedLit drops backslashes that escape characters with no special
// meaning, so \a and a agree.
func unquotedLit(raw string) string {
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\\' && i+1 < len(raw) {
			r, size := utf8.DecodeRuneInString(raw[i+1:])
			if !plainRune(r) {
				b.WriteByte('\\')
			}
			b.WriteString(raw[i+1 : i+1+size])
			i += size
			continue
		}
		b.WriteByte(raw[i])
	}
	return b.String()
}

// escapeShell writes literal text as unquoted shell source.
func escapeShell(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case plainRune(r):
			b.WriteRune(r)
		case r == '\n':
			b.WriteString("'\n'")
		default:
			b.WriteByte('\\')
			b.WriteRune(r)
		}
	}
	return b.String()
}

func plainRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_./,:=@%+-", r)
}

// plainRedirects allows only redirections that cannot touch a file:
// output or errors to /dev/null, and fd duplication (2>&1).
func plainRedirects(rs []*syntax.Redirect) bool {
	for _, r := range rs {
		if r.Hdoc != nil || r.Word == nil || r.N != nil && strings.HasPrefix(r.N.Value, "{") {
			return false
		}
		target, ok := literalWord(r.Word.Parts)
		if !ok {
			return false
		}
		switch r.Op {
		case syntax.RdrOut, syntax.AppOut, syntax.RdrAll, syntax.AppAll, syntax.RdrClob:
			if target != "/dev/null" {
				return false
			}
		case syntax.DplOut, syntax.DplIn:
			if target != "1" && target != "2" && target != "-" {
				return false
			}
		default:
			return false
		}
	}
	return true
}
