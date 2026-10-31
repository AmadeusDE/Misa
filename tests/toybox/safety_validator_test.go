package toybox

// The fixtures are pinned and reviewed, not adversarial programs. This is a
// best-effort static guard, not an OS sandbox: it rejects unsupported shell
// operations with obvious host effects before replaying any setup. It does not
// confine malicious shell code or prove that dynamic paths are safe.
import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// A denylist is appropriate only because these upstream fixtures are pinned and
// reviewed. Unknown shell programs and dynamic arguments are not proven safe.
var dangerousCommands = map[string]bool{
	"sudo": true, "su": true, "mount": true, "umount": true, "mknod": true,
	"chroot": true, "unshare": true, "curl": true, "wget": true, "nc": true,
	"ncat": true, "netcat": true, "ssh": true, "scp": true, "sftp": true,
	"telnet": true, "ftp": true, "eval": true,
}
var dangerousFragment = regexp.MustCompile(`(^|[;&|()[:space:]])(sudo|su|mount|umount|mknod|chroot|unshare|curl|wget|nc|ncat|netcat|ssh|scp|sftp|telnet|ftp|eval)([;&|()[:space:]]|$)`)

var mutators = map[string]bool{"touch": true, "mkdir": true, "rmdir": true, "rm": true, "cp": true, "mv": true, "ln": true, "link": true, "chmod": true, "chown": true, "truncate": true, "tee": true, "mkfifo": true, "mktemp": true}

func safePath(p string) error {
	if p == "" {
		return nil
	}
	if filepath.IsAbs(p) || p == "~" || strings.HasPrefix(p, "~/") || filepath.Clean(p) == ".." || strings.HasPrefix(filepath.Clean(p), "../") {
		return fmt.Errorf("path %q escapes sandbox", p)
	}
	return nil
}

func literalWord(w *syntax.Word) (string, bool) {
	if w == nil {
		return "", false
	}
	var b strings.Builder
	var parts func([]syntax.WordPart) bool
	parts = func(ps []syntax.WordPart) bool {
		for _, p := range ps {
			switch x := p.(type) {
			case *syntax.Lit:
				b.WriteString(x.Value)
			case *syntax.SglQuoted:
				b.WriteString(x.Value)
			case *syntax.DblQuoted:
				if !parts(x.Parts) {
					return false
				}
			default:
				return false
			}
		}
		return true
	}
	ok := parts(w.Parts)
	return b.String(), ok
}

func validateScript(src string) error {
	f, err := syntax.NewParser().Parse(strings.NewReader(src), "")
	if err != nil {
		// The legacy fixture adapter sometimes records an incomplete fragment of a
		// compound statement. Bash still receives that same fragment as before.
		if m := dangerousFragment.FindStringSubmatch(src); m != nil {
			return fmt.Errorf("dangerous command %q", m[2])
		}
		return nil
	}
	var problem error
	syntax.Walk(f, func(n syntax.Node) bool {
		if problem != nil {
			return false
		}
		switch x := n.(type) {
		case *syntax.CallExpr:
			for _, a := range x.Assigns {
				if a.Name != nil && (a.Name.Value == "PATH" || a.Name.Value == "BASH_ENV" || a.Name.Value == "ENV") {
					problem = fmt.Errorf("unsafe shell environment assignment %s", a.Name.Value)
					return false
				}
			}
			if len(x.Args) == 0 {
				break
			}
			name, ok := literalWord(x.Args[0])
			if !ok {
				break
			}
			if dangerousCommands[filepath.Base(name)] {
				problem = fmt.Errorf("dangerous command %q", name)
				break
			}
			for _, w := range x.Args[1:] {
				if arg, ok := literalWord(w); ok && strings.HasPrefix(arg, "/dev/") && arg != "/dev/null" {
					problem = fmt.Errorf("device access %q", arg)
					return false
				}
			}
			if mutators[filepath.Base(name)] {
				for i, w := range x.Args[1:] {
					if name == "truncate" && i > 0 {
						prev, _ := literalWord(x.Args[i])
						if prev == "-s" || prev == "--size" {
							continue
						}
					}
					if arg, ok := literalWord(w); ok {
						if strings.HasPrefix(arg, "-") {
							if _, value, has := strings.Cut(arg, "="); has {
								arg = value
							} else {
								continue
							}
						}
						if err := safePath(arg); err != nil {
							problem = err
							break
						}
					}
				}
			}
		case *syntax.Redirect:
			if arg, ok := literalWord(x.Word); ok && arg != "/dev/null" {
				op := x.Op.String()
				if strings.Contains(op, ">") || strings.HasPrefix(arg, "/dev/") {
					if err := safePath(arg); err != nil {
						problem = err
					}
				}
			}
		}
		return problem == nil
	})
	return problem
}

func validateSetup(log []setupEvent, line int) error {
	for _, e := range log {
		if e.line > line {
			continue
		}
		if e.name == "umask" {
			continue
		}
		if e.name == "exec" || e.name == "ln" {
			if err := validateScript(e.content); err != nil {
				return fmt.Errorf("setup line %d: %w", e.line, err)
			}
			continue
		}
		if err := safePath(e.name); err != nil {
			return fmt.Errorf("setup line %d: %w", e.line, err)
		}
	}
	return nil
}

// Avoid inherited shell startup hooks and PATH components from fixtures.
func toyEnvironment(dir string, vars map[string]string) []string {
	env := []string{"PATH=" + binDir, "HOME=" + dir, "TMPDIR=" + dir, "LC_ALL=C"}
	if fixtureDir != "" {
		env = append(env, "FILES="+fixtureDir)
	}
	for k, v := range vars {
		switch k {
		case "PATH", "HOME", "TMPDIR", "BASH_ENV", "ENV", "SHELLOPTS", "BASHOPTS", "CDPATH", "IFS":
			continue
		}
		if strings.HasPrefix(k, "LD_") || strings.HasPrefix(k, "BASH_FUNC_") || !validVariableName(k) {
			continue
		}
		if !strings.ContainsRune(v, 0) {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// Resolve symlinks as far as possible, including dangling links, before a Go
// setup mutation. This is hygiene for reviewed fixtures, not race-free confinement.
func validateSetupPath(dir, name string) error {
	if err := safePath(name); err != nil {
		return err
	}
	pending := strings.Split(filepath.Clean(name), string(filepath.Separator))
	current := dir
	links := 0
	for len(pending) > 0 {
		part := pending[0]
		pending = pending[1:]
		current = filepath.Join(current, part)
		rel, err := filepath.Rel(dir, current)
		if err != nil {
			return err
		}
		if err := safePath(rel); err != nil {
			return err
		}
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		links++
		if links > 40 {
			return fmt.Errorf("too many symlinks in %q", name)
		}
		target, err := os.Readlink(current)
		if err != nil {
			return err
		}
		if filepath.IsAbs(target) {
			current = target
		} else {
			current = filepath.Join(filepath.Dir(current), target)
		}
		rel, err = filepath.Rel(dir, current)
		if err != nil {
			return err
		}
		if err := safePath(rel); err != nil {
			return err
		}
		pending = append(strings.Split(rel, string(filepath.Separator)), pending...)
		current = dir
	}
	return nil
}

func validVariableName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
