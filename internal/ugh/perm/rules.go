package perm

import (
	"bytes"
	"fmt"
	"github.com/AmadeusDE/Misa/internal/ugh/roots"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
)

// Rule is one permanent manual-mode allowance (allow.toml). A bash rule
// matches commands whose argv starts with Command's words (all of them with
// Exact); a path rule matches read/write/edit/ls/grep/find calls whose target
// matches the Path glob; a rule with only Tool matches every call of that
// tool. Cwd, when set, limits the rule to agents working under that glob.
type Rule struct {
	Tool    string `toml:"tool"`
	Command string `toml:"command,omitempty"`
	Exact   bool   `toml:"exact,omitempty"`
	Path    string `toml:"path,omitempty"`
	Cwd     string `toml:"cwd,omitempty"`
}

func (r Rule) String() string {
	s := r.Tool
	switch {
	case r.Command != "" && r.Exact:
		s += ": " + r.Command + " (exact)"
	case r.Command != "":
		s += ": " + r.Command + " …"
	case r.Path != "":
		s += ": " + r.Path
	default:
		s += ": any call"
	}
	if r.Cwd != "" {
		s += " in " + r.Cwd
	}
	return s
}

// pathTools take their target from the "path" argument (the cwd when empty).
var pathTools = map[string]bool{"read": true, "write": true, "edit": true, "ls": true, "grep": true, "find": true}

// Rules is the manual-mode allow list, shared through a file so edits by
// hand and by other ugh processes apply on the next check.
type Rules struct {
	Path string
	// Split turns a bash command line into the argv of each command it
	// runs, each word as canonical shell source (so quoted and expanded
	// text differ); ok=false means it is not a plain command line (static
	// .SimpleCommands). Without it no bash rule ever matches.
	Split func(cmd string) ([][]string, bool)

	mu    sync.Mutex
	list  []Rule
	mtime time.Time
}

// DefaultRulesPath is ~/.config/ugh/allow.toml (XDG_CONFIG_HOME aware).
func DefaultRulesPath() string { return RulesPath(roots.Roots{}) }

// RulesPath is allow.toml under r.
func RulesPath(r roots.Roots) string {
	p, err := r.ConfigFile("allow.toml")
	if err != nil {
		return ""
	}
	return p
}

type rulesFile struct {
	Rule []Rule `toml:"rule"`
}

// List returns the current rules (reloading the file if it changed).
func (r *Rules) List() []Rule {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reload()
	return append([]Rule(nil), r.list...)
}

func (r *Rules) reload() {
	st, err := os.Stat(r.Path)
	if err != nil {
		r.list, r.mtime = nil, time.Time{}
		return
	}
	if st.ModTime().Equal(r.mtime) {
		return
	}
	var f rulesFile
	if _, err := toml.DecodeFile(r.Path, &f); err != nil {
		return // keep the last good list; a broken edit never widens access
	}
	r.list, r.mtime = f.Rule, st.ModTime()
}

// Add appends rules (skipping duplicates) and saves the file.
func (r *Rules) Add(add ...Rule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reload()
	for _, n := range add {
		dup := false
		for _, o := range r.list {
			dup = dup || o == n
		}
		if !dup {
			r.list = append(r.list, n)
		}
	}
	return r.save()
}

// Remove deletes the i-th rule (0-based, as List orders them).
func (r *Rules) Remove(i int) (Rule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reload()
	if i < 0 || i >= len(r.list) {
		return Rule{}, fmt.Errorf("no rule %d", i+1)
	}
	gone := r.list[i]
	r.list = append(r.list[:i:i], r.list[i+1:]...)
	return gone, r.save()
}

func (r *Rules) save() error {
	var b bytes.Buffer
	b.WriteString("# Manual-mode permanent allow rules. ugh adds them when you answer [p] at a\n" +
		"# prompt; edit freely. Bash rules match an argv prefix (exact = true: the whole\n" +
		"# argv), and every command of a line must match; lines with variables, $(…),\n" +
		"# subshells or file redirects are always asked. path is a glob (** = any depth).\n\n")
	if err := toml.NewEncoder(&b).Encode(rulesFile{Rule: r.list}); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(r.Path), 0o700); err != nil {
		return err
	}
	tmp := r.Path + ".tmp"
	if err := os.WriteFile(tmp, b.Bytes(), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, r.Path); err != nil {
		return err
	}
	if st, err := os.Stat(r.Path); err == nil {
		r.mtime = st.ModTime()
	}
	return nil
}

// Match reports the rule that allows c, if any.
func (r *Rules) Match(c Call) (Rule, bool) {
	if r == nil {
		return Rule{}, false
	}
	rules := r.List()
	inCwd := func(ru Rule) bool { return ru.Cwd == "" || globMatch(ru.Cwd, realPath(c.Cwd, ".")) }
	if c.Tool == "bash" {
		if r.Split == nil {
			return Rule{}, false
		}
		argvs, ok := r.Split(c.Command)
		if !ok || len(argvs) == 0 {
			return Rule{}, false
		}
		var first Rule
		for i, argv := range argvs {
			hit := false
			for _, ru := range rules {
				if ru.Tool == "bash" && ru.Command != "" && inCwd(ru) && r.argvMatch(ru, argv) {
					hit = true
					if i == 0 {
						first = ru
					}
					break
				}
			}
			if !hit {
				return Rule{}, false
			}
		}
		return first, true
	}
	target := ""
	if pathTools[c.Tool] {
		target = realPath(c.Cwd, targetArg(c))
	}
	for _, ru := range rules {
		if ru.Tool != c.Tool || ru.Command != "" || !inCwd(ru) {
			continue
		}
		if ru.Path == "" || (target != "" && globMatch(ru.Path, target)) {
			return ru, true
		}
	}
	return Rule{}, false
}

func (r *Rules) argvMatch(ru Rule, argv []string) bool {
	want, ok := r.Split(ru.Command)
	if !ok || len(want) != 1 || len(want[0]) == 0 {
		return false
	}
	w := want[0]
	if len(argv) < len(w) || ru.Exact && len(argv) != len(w) {
		return false
	}
	for i := range w {
		if argv[i] != w[i] {
			return false
		}
	}
	return true
}

// Propose is what an "always allow" answer would save for c: per bash
// command its name and subcommand ("git status", "go test") when the
// second word looks like one, else the exact argv; for path tools the agent's
// working tree (or the exact path outside it); otherwise the tool. ok=false
// means no rule can cover c (a bash line that is not plain).
func (r *Rules) Propose(c Call) ([]Rule, bool) {
	if c.Tool == "bash" {
		if r == nil || r.Split == nil {
			return nil, false
		}
		argvs, ok := r.Split(c.Command)
		if !ok || len(argvs) == 0 {
			return nil, false
		}
		var out []Rule
		for _, argv := range argvs {
			ru := Rule{Tool: "bash", Command: strings.Join(argv, " "), Exact: true}
			if len(argv) >= 2 && subcommandRx.MatchString(argv[1]) {
				ru = Rule{Tool: "bash", Command: strings.Join(argv[:2], " ")}
			}
			out = append(out, ru)
		}
		return out, true
	}
	if pathTools[c.Tool] {
		cwd, target := realPath(c.Cwd, "."), realPath(c.Cwd, targetArg(c))
		if target == cwd || strings.HasPrefix(target, cwd+string(filepath.Separator)) {
			return []Rule{{Tool: c.Tool, Path: tildePath(cwd) + "/**"}}, true
		}
		return []Rule{{Tool: c.Tool, Path: tildePath(target)}}, true
	}
	return []Rule{{Tool: c.Tool}}, true
}

var subcommandRx = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func targetArg(c Call) string {
	if p := stringArg(c.Args, "path"); p != "" {
		return p
	}
	return "."
}

// realPath resolves p against cwd, expands ~, and resolves symlinks in the
// longest existing prefix, so a link inside an allowed tree cannot reach
// outside it.
func realPath(cwd, p string) string {
	p = expandHome(p)
	if !filepath.IsAbs(p) {
		p = filepath.Join(expandHome(cwd), p)
	}
	p = filepath.Clean(p)
	rest := ""
	for cur := p; ; cur = filepath.Dir(cur) {
		if real, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(real, rest)
		}
		if cur == filepath.Dir(cur) {
			return p
		}
		rest = filepath.Join(filepath.Base(cur), rest)
	}
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		return filepath.Join(os.Getenv("HOME"), p[1:])
	}
	return p
}

func tildePath(p string) string {
	if h := os.Getenv("HOME"); h != "" && (p == h || strings.HasPrefix(p, h+"/")) {
		return "~" + p[len(h):]
	}
	return p
}

// globMatch matches a cleaned absolute path against a glob: ** spans any
// number of directories, * and ? stay inside one. A pattern ending in /**
// also matches the directory itself.
func globMatch(pattern, path string) bool {
	pattern = expandHome(pattern)
	if strings.HasSuffix(pattern, "/**") && path == filepath.Clean(strings.TrimSuffix(pattern, "/**")) {
		return true
	}
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch ch := pattern[i]; {
		case ch == '*' && i+1 < len(pattern) && pattern[i+1] == '*':
			b.WriteString(".*")
			i++
		case ch == '*':
			b.WriteString("[^/]*")
		case ch == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(ch)))
		}
	}
	b.WriteString("$")
	rx, err := regexp.Compile(b.String())
	return err == nil && rx.MatchString(path)
}
