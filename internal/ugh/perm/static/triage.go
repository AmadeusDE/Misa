package static

// Triage sorts a tool call into a risk tier before any model classifier
// runs, so cheap calls get no (or one) model verdict and risky calls get
// the full panel. Tiers, in order: trivial, low, middle, high.
//
//   trivial: read-only work inside the workspace (file tools, read-only shell
//            commands, builds/tests, orchestration tools)
//   low:     edits/writes inside the workspace, scripts, unknown commands
//   middle:  mutations inside the workspace (rm, mv, git commit/reset, sed
//            -i, kill), redirects into files, installs
//   high:    network or uploads, git push, privilege escalation, anything
//            touching a sensitive path, writes outside the workspace, and
//            commands that cannot be parsed

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/AmadeusDE/Misa/internal/ugh/perm"
	"mvdan.cc/sh/v3/syntax"
)

// Tier names, weakest first.
const (
	Trivial = "trivial"
	Low     = "low"
	Middle  = "middle"
	High    = "high"
)

var tierRank = map[string]int{Trivial: 0, Low: 1, Middle: 2, High: 3}

// ---- workspace ----

var (
	wsMu    sync.RWMutex
	wsRoots []string // configured workspace roots, scratch base, temp dir
)

// SetWorkspace sets the extra directories that count as inside the
// workspace (besides the cwd and its git root): configured roots, the
// agents' scratch base and the system temp directory.
func SetWorkspace(roots ...string) {
	wsMu.Lock()
	defer wsMu.Unlock()
	wsRoots = nil
	for _, r := range roots {
		if r != "" {
			wsRoots = append(wsRoots, filepath.Clean(r))
		}
	}
}

// InWorkspace reports whether full (absolute, clean) is inside the cwd, the
// cwd's git root, a configured root, the scratch base or the temp dir.
func InWorkspace(full, cwd string) bool {
	if under(full, cwd) {
		return true
	}
	if g := gitRoot(cwd); g != "" && under(full, g) {
		return true
	}
	wsMu.RLock()
	defer wsMu.RUnlock()
	for _, r := range wsRoots {
		if under(full, r) {
			return true
		}
	}
	return false
}

var gitRoots sync.Map // cwd → git root ("" when none)

func gitRoot(cwd string) string {
	if v, ok := gitRoots.Load(cwd); ok {
		return v.(string)
	}
	root := ""
	for d := cwd; d != "" && d != "/"; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			root = d
			break
		}
	}
	gitRoots.Store(cwd, root)
	return root
}

// ---- sensitive paths ----

// sensitiveRel are home-relative locations whose contents are secret or
// control login and shells: even reading them is high tier.
var sensitiveRel = []string{
	".ssh", ".gnupg", ".aws", ".kube", ".docker/config.json", ".netrc", ".git-credentials",
	".config/ugh/credentials.json", ".pi/agent/auth.json", ".local/share/keyrings",
	".bashrc", ".bash_profile", ".profile", ".zshrc", ".zprofile", ".zshenv",
}

// Sensitive reports whether full is a secret or system-control path.
func Sensitive(full string) bool {
	if full == "/etc" || strings.HasPrefix(full, "/etc/") || strings.HasPrefix(full, "/root") {
		return true
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		return false
	}
	if full == home {
		return true // the home directory itself is not a project
	}
	for _, r := range sensitiveRel {
		if p := filepath.Join(home, r); full == p || strings.HasPrefix(full, p+"/") {
			return true
		}
	}
	return false
}

// ---- triage ----

// Triage returns the call's tier and a short reason.
func Triage(c perm.Call) (string, string) {
	t := &triage{cwd: c.Cwd, tier: Trivial, reason: "read-only"}
	if Sensitive(c.Cwd) {
		t.raise(High, "working directory is sensitive: "+c.Cwd)
	}
	switch c.Tool {
	case "spawn", "wait_agents", "agent_status", "resume_agent", "council":
		t.raise(Trivial, "orchestration")
	case "ask_user":
		t.raise(Trivial, "asks the user a question")
	case "recall":
		t.raise(Trivial, "re-reads this session's own tool output")
	case "council_escalate":
		t.raise(High, "direct oracle spending after a council")
	case "worktree_status":
		t.raise(Trivial, "worktree listing")
	case "worktree_integrate", "worktree_cleanup":
		t.raise(Middle, "mutates the shared repository: "+c.Tool)
	case "read", "ls", "grep", "find":
		t.path(stringArg(c.Args, "path"), false)
	case "write", "edit":
		t.raise(Low, "workspace edit")
		t.path(stringArg(c.Args, "path"), true)
	case "bash":
		t.bash(c.Command)
	default:
		t.raise(Low, "tool "+c.Tool)
	}
	return t.tier, t.reason
}

type triage struct {
	cwd          string
	tier, reason string
	// dirs are the directories a command line may have moved to (cd, pushd,
	// env -C), as a set since control flow isn't followed; lostDir is set
	// once it moved somewhere that can't be known (cd "$X", cd -, popd).
	dirs    []string
	lostDir bool
}

const maxDirs = 16

// candidates are the places a path argument may name.
func (t *triage) candidates(p string) []string {
	p = expandHome(p)
	if filepath.IsAbs(p) {
		return []string{filepath.Clean(p)}
	}
	if t.lostDir {
		t.raise(Low, "relative path after a directory change that can't be followed")
	}
	if len(t.dirs) == 0 {
		return []string{resolvePath(t.cwd, p)}
	}
	out := make([]string, len(t.dirs))
	for i, d := range t.dirs {
		out[i] = resolvePath(d, p)
	}
	return out
}

// chdir adds where moving to w may lead, from every directory so far.
func (t *triage) chdir(w word) {
	if w.dyn || w.lit == "" || w.lit == "-" {
		t.lostDir = true
		return
	}
	next := t.candidates(w.lit)
	for _, d := range next {
		if t.sensitive(d) {
			t.raise(High, "sensitive path "+d)
		}
	}
	if len(t.dirs) == 0 {
		t.dirs = []string{t.cwd}
	}
	for _, d := range next {
		if !slices.Contains(t.dirs, d) {
			t.dirs = append(t.dirs, d)
		}
	}
	if len(t.dirs) > maxDirs {
		t.lostDir = true
	}
}

// cd follows cd/pushd/popd; their target is checked like any argument.
func (t *triage) cd(name string, args []word) {
	if name == "popd" {
		t.lostDir = true
		return
	}
	for _, a := range args {
		if !a.dyn && strings.HasPrefix(a.lit, "-") && a.lit != "-" {
			continue
		}
		t.chdir(a)
		return
	}
	home, _ := os.UserHomeDir()
	t.chdir(word{lit: home})
}

func (t *triage) raise(tier, reason string) {
	if tierRank[tier] > tierRank[t.tier] || (t.reason == "read-only" && tierRank[tier] == tierRank[t.tier]) {
		t.tier, t.reason = tier, reason
	}
}

// path judges one path argument (write: the call modifies it).
func (t *triage) path(p string, write bool) {
	if p == "" {
		p = "."
	}
	if strings.HasPrefix(p, "~") || strings.HasPrefix(p, "$HOME") {
		home, _ := os.UserHomeDir()
		p = home + strings.TrimPrefix(strings.TrimPrefix(p, "~"), "$HOME")
	}
	full := resolvePath(t.cwd, p)
	switch {
	case t.sensitive(full):
		t.raise(High, "sensitive path "+full)
	case !t.inside(full) && write:
		t.raise(High, "write outside workspace "+full)
	case !t.inside(full):
		t.raise(Low, "read outside workspace "+full)
	}
}

// sensitive checks a path both as written and where its symlinks lead.
func (t *triage) sensitive(full string) bool {
	return Sensitive(full) || Sensitive(realPath(full))
}

// inside reports whether a path is in the workspace both as written and
// where its symlinks lead (the cwd may itself be reached through a link).
func (t *triage) inside(full string) bool {
	if !InWorkspace(full, t.cwd) {
		return false
	}
	real := realPath(full)
	return InWorkspace(real, t.cwd) || InWorkspace(real, realPath(t.cwd))
}

// realPath resolves the symlinks in full's longest existing prefix.
func realPath(full string) string {
	rest := ""
	for p := full; ; p = filepath.Dir(p) {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return filepath.Join(r, rest)
		}
		if p == filepath.Dir(p) {
			return full
		}
		rest = filepath.Join(filepath.Base(p), rest)
	}
}

// Command classes for bash triage.
var (
	readOnlyCmds = set(`cat head tail less more wc ls tree stat file grep rg egrep fgrep zgrep find fd sed awk sort uniq
		cut tr diff cmp jq yq echo printf pwd which type command basename dirname realpath readlink date javap
		unzip zipinfo test [ true false sleep du df ps pgrep nl xxd od hexdump strings column comm paste xargs
		cd pushd popd export set unset local read wait env printenv id whoami hostname uname nproc free uptime
		md5sum sha1sum sha256sum base64 seq yes time timeout`)
	buildCmds = set(`go gradle gradlew mvn make cargo npm pnpm yarn npx javac java kotlinc tsc node-gyp cmake ninja
		pytest jest vitest`)
	scriptCmds   = set(`python python3 node fish ruby perl lua deno bun php`)
	shellCmds    = set(`sh bash zsh dash ksh ash`)
	awkCmds      = set(`awk gawk mawk nawk`)
	mutatingCmds = set(`rm rmdir mv cp chmod chown chgrp ln kill pkill killall tar truncate install
		patch shred unlink rsync`)
	createCmds  = set(`mkdir touch`)
	networkCmds = set(`curl wget ssh scp sftp nc ncat netcat telnet ftp socat`)
	privCmds    = set(`sudo doas su pkexec`)
	pkgCmds     = set(`apt apt-get dnf yum pacman zypper apk brew pip pip3 pipx gem cargo-install`)
)

func set(words string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(words) {
		m[w] = true
	}
	return m
}

func (t *triage) bash(cmd string) {
	if strings.TrimSpace(cmd) == "" {
		t.raise(Low, "empty command")
		return
	}
	f, err := syntax.NewParser().Parse(strings.NewReader(cmd), "")
	if err != nil {
		t.raise(High, "unparseable command")
		return
	}
	syntax.Walk(f, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.Stmt:
			t.redirects(x.Redirs)
		case *syntax.CallExpr:
			t.call(x)
		case *syntax.FuncDecl:
			t.raise(Low, "defines a shell function")
		}
		return true
	})
}

func (t *triage) call(c *syntax.CallExpr) {
	var args []word
	for _, w := range c.Args {
		lit, dyn := partsLit(w.Parts)
		args = append(args, word{lit: lit, dyn: dyn})
	}
	if len(args) > 0 {
		t.argv(args) // no words: bare assignments (JAR=x)
	}
}

// argv judges one command line, by what its wrappers run.
func (t *triage) argv(words []word) {
	cmd, priv, dirs := unwrap(words)
	if priv != "" {
		t.raise(High, "privilege escalation ("+priv+")")
	}
	for _, d := range dirs {
		t.chdir(d)
	}
	if len(cmd) == 0 {
		cmd = words // a bare wrapper: env, xargs (echo), time
	}
	args := lits(cmd)
	name := baseName(args[0])
	rest := args[1:]
	// Paths named on the command line: sensitive ones raise to high.
	for _, a := range rest {
		if a == "" || strings.HasPrefix(a, "-") {
			continue
		}
		for _, full := range t.candidates(a) {
			if t.sensitive(full) {
				t.raise(High, "sensitive path "+full)
			}
		}
	}
	if name == "cd" || name == "pushd" || name == "popd" {
		t.cd(name, cmd[1:])
	}
	switch {
	case privCmds[name]:
		t.raise(High, "privilege escalation ("+name+")")
	case networkCmds[name]:
		t.raise(High, "network ("+name+")")
	case IsGitBinary(name):
		tier, reason, _ := ClassifyGit(args, t.cwd)
		t.raise(tier, reason)
	case createCmds[name]:
		t.raise(Low, name)
	case pkgCmds[name] || (name == "npm" || name == "pnpm" || name == "yarn") && hasAny(rest, "install", "add", "i", "remove", "uninstall"):
		t.raise(Middle, "package install/remove ("+name+")")
	case name == "find":
		t.find(cmd[1:])
	case name == "sed":
		t.sed(rest)
	case awkCmds[name]:
		t.awk(rest)
	case mutatingCmds[name]:
		t.raise(Middle, name+" (modifies files or processes)")
	case buildCmds[name]:
		if hasAny(rest, "publish", "deploy", "upload", "release") {
			t.raise(High, name+" publish")
		}
	case shellCmds[name]:
		t.raise(Low, "runs a script ("+name+")")
		if script, ok := dashC(rest); ok {
			t.bash(script)
		}
	case scriptCmds[name] || strings.HasSuffix(name, ".sh") || strings.HasPrefix(args[0], "./"):
		t.raise(Low, "runs a script ("+name+")")
	case readOnlyCmds[name]:
	case name == "":
		t.raise(Low, "dynamic command")
	default:
		t.raise(Low, "command "+name)
	}
}

func lits(ws []word) []string {
	out := make([]string, len(ws))
	for i, w := range ws {
		out[i] = w.lit
	}
	return out
}

// dashC returns the script of sh -c (or a flag cluster such as -lc).
func dashC(rest []string) (string, bool) {
	for i, a := range rest {
		if !strings.HasPrefix(a, "-") || strings.HasPrefix(a, "--") {
			return "", false
		}
		if strings.Contains(a, "c") && i+1 < len(rest) {
			return rest[i+1], true
		}
	}
	return "", false
}

// find judges the commands find runs and the files it deletes or writes.
func (t *triage) find(args []word) {
	for i := 0; i < len(args); i++ {
		switch args[i].lit {
		case "-delete":
			t.raise(Middle, "find -delete")
		case "-fprint", "-fprint0", "-fprintf", "-fls":
			t.raise(Middle, "find "+args[i].lit+" (writes a file)")
		case "-exec", "-execdir", "-ok", "-okdir":
			j := i + 1
			for j < len(args) && !isFindEnd(args[j].lit) {
				j++
			}
			if j > i+1 {
				t.argv(args[i+1 : j])
			}
			i = j
		}
	}
}

func isFindEnd(s string) bool { return s == ";" || s == `\;` || s == "+" }

// sed is read-only unless it edits in place, reads a script file, or a
// script uses a command that writes files or runs programs.
func (t *triage) sed(rest []string) {
	var scripts []string
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case a == "--expression" || a == "-e":
			if i+1 < len(rest) {
				i++
				scripts = append(scripts, rest[i])
			}
		case strings.HasPrefix(a, "--expression="):
			scripts = append(scripts, strings.TrimPrefix(a, "--expression="))
		case strings.HasPrefix(a, "--in-place"):
			t.raise(Middle, "sed -i (edits files)")
		case strings.HasPrefix(a, "--file") || strings.HasPrefix(a, "--"):
			if strings.HasPrefix(a, "--file") {
				t.raise(Low, "sed script file")
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			for k, ch := range a[1:] {
				if ch == 'i' {
					t.raise(Middle, "sed -i (edits files)")
					break // the rest is the backup suffix
				}
				if ch == 'f' {
					t.raise(Low, "sed script file")
					break
				}
				if ch == 'e' {
					if v := a[k+2:]; v != "" {
						scripts = append(scripts, v)
					} else if i+1 < len(rest) {
						i++
						scripts = append(scripts, rest[i])
					}
					break
				}
			}
		case len(scripts) == 0:
			scripts = append(scripts, a)
		}
	}
	for _, sc := range scripts {
		if !sedReadOnly(sc) {
			t.raise(Low, "sed script may write files or run commands")
		}
	}
}

// awk is read-only unless the program comes from a file or could pipe,
// redirect or call system().
func (t *triage) awk(rest []string) {
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case a == "-f" || strings.HasPrefix(a, "--file") || strings.HasPrefix(a, "-f"):
			t.raise(Low, "awk program file")
			return
		case a == "-v" || a == "-F":
			i++
		case a == "-e" || a == "--source":
			if i+1 < len(rest) {
				t.awkProgram(rest[i+1])
			}
			return
		case strings.HasPrefix(a, "-") && a != "-":
		default:
			t.awkProgram(a)
			return
		}
	}
}

func (t *triage) awkProgram(p string) {
	if strings.ContainsAny(p, "|>") || strings.Contains(p, "system") {
		t.raise(Low, "awk program may run commands or write files")
	}
}

func (t *triage) redirects(rs []*syntax.Redirect) {
	for _, r := range rs {
		if r == nil || r.Word == nil {
			continue
		}
		target, _ := partsLit(r.Word.Parts)
		switch r.Op {
		case syntax.RdrIn, syntax.RdrInOut:
			for _, full := range t.candidates(cmp.Or(target, ".")) {
				if t.sensitive(full) {
					t.raise(High, "redirect from sensitive path "+full)
				}
			}
			if r.Op == syntax.RdrIn {
				continue
			}
		case syntax.RdrOut, syntax.RdrClob, syntax.RdrAll, syntax.AppOut, syntax.AppAll:
		default:
			continue
		}
		if target == "" || target == "/dev/null" || target == "/dev/stdout" || target == "/dev/stderr" || strings.HasPrefix(target, "/dev/fd/") {
			continue
		}
		for _, full := range t.candidates(target) {
			switch {
			case t.sensitive(full):
				t.raise(High, "redirect into sensitive path "+full)
			case !t.inside(full):
				t.raise(High, "redirect outside workspace "+full)
			default:
				t.raise(Middle, "redirect into "+target)
			}
		}
	}
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~") || strings.HasPrefix(p, "$HOME") {
		home, _ := os.UserHomeDir()
		return home + strings.TrimPrefix(strings.TrimPrefix(p, "~"), "$HOME")
	}
	return p
}

func hasAny(args []string, words ...string) bool {
	for _, a := range args {
		for _, w := range words {
			if a == w {
				return true
			}
		}
	}
	return false
}

func hasPrefixAny(args []string, prefixes ...string) bool {
	for _, a := range args {
		for _, p := range prefixes {
			if strings.HasPrefix(a, p) {
				return true
			}
		}
	}
	return false
}
