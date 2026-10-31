package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AmadeusDE/Misa/internal/sh/syntax"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
)

const criticalStart = "[Pinned critical state — retained by the harness, not the summarizer]\n" +
	"Cwd is the tool cwd; Scratch is $UGH_SCRATCH. Observed shell cd paths do not change the tool cwd.\n"
const criticalTasks = "\nAssigned tasks and later instructions (head+tail if long; rule lines retained):\n"
const criticalEnd = "\n[End pinned critical state]"

// criticalState is independent of the summarizer and the context ladder. Its
// path metadata is machine-readable so session reloads retain warning roots;
// task text is kept verbatim, including rule lines outside a bounded excerpt.
type criticalState struct {
	Paths criticalPaths
	Tasks string
}

type criticalPaths struct {
	Cwd         string
	Scratch     string
	Worktrees   []criticalWorktree
	Directories []string // observed shell cd targets (not a change to agent cwd)
}

type criticalWorktree struct {
	Path   string
	Branch string
}

const (
	criticalTasksMax = 32 << 10
	criticalTaskSep  = "\n--- Task / instruction ---\n"
	steerPrefix      = "[Message from the user, sent while you were working]\n"
)

// pinTask records an assignment or instruction once. When the pins outgrow
// criticalTasksMax the original assignment and the newest instructions stay.
func (a *Agent) pinTask(task string) {
	task = strings.TrimSpace(strings.TrimPrefix(task, steerPrefix))
	if task == "" {
		return
	}
	entry := criticalTaskText(task)
	entries := strings.Split(a.critical.Tasks, criticalTaskSep)[1:]
	for i := range entries {
		entries[i] = strings.TrimSuffix(entries[i], "\n")
		if entries[i] == entry {
			return
		}
	}
	entries = append(entries, entry)
	a.critical.Tasks = joinCriticalTasks(entries)
	for len(entries) > 2 && len(a.critical.Tasks) > criticalTasksMax {
		entries = append(entries[:1], entries[2:]...)
		a.critical.Tasks = joinCriticalTasks(entries)
	}
}

func joinCriticalTasks(entries []string) string {
	return criticalTaskSep + strings.Join(entries, "\n"+criticalTaskSep) + "\n"
}

func criticalTaskText(task string) string {
	const half = 4096
	text := task
	if len(task) > 2*half {
		text = task[:half] + "\n[… task middle omitted …]\n" + task[len(task)-half:]
		rules := 0
		for _, line := range strings.Split(task, "\n") {
			upper := strings.ToUpper(line)
			if strings.Contains(upper, "MUST") || strings.Contains(upper, "NEVER") ||
				strings.Contains(upper, "DO NOT") || strings.Contains(upper, "RULES") {
				if rules += len(line) + 1; rules > half {
					break
				}
				text += "\n" + line
			}
		}
	}
	return text
}

// restoreCritical replays only source messages, never a model's summary. A
// pinned seed supersedes everything before it on a session reload.
func (a *Agent) restoreCritical() {
	for _, m := range a.hist {
		if m.Role == llm.RoleUser {
			if start := strings.LastIndex(m.Text, criticalStart); start >= 0 {
				body := m.Text[start+len(criticalStart):]
				if end := strings.Index(body, criticalEnd); end >= 0 {
					parts := strings.SplitN(body[:end], criticalTasks, 2)
					var paths criticalPaths
					if len(parts) == 2 && json.Unmarshal([]byte(parts[0]), &paths) == nil {
						a.critical = criticalState{Paths: paths, Tasks: parts[1]}
						continue
					}
				}
			}
			if m.Text != turnOutNudge && !strings.HasPrefix(m.Text, "[Subagent results:") {
				a.pinTask(m.Text)
			}
		}
		for _, call := range m.ToolCalls {
			a.noteCriticalCall(call)
		}
	}
}

func (a *Agent) criticalBlock() string {
	// Config may be filled in after New (notably a spawned child's scratch).
	a.critical.Paths.Cwd = criticalAbs(a.cfg.Cwd, ".")
	a.critical.Paths.Scratch = a.cfg.Scratch
	if a.rt != nil {
		if wt := a.rt.worktreeAt(a.cfg.Cwd); wt != nil {
			a.pinWorktree(wt.Dir, wt.Meta.Branch)
		}
	}
	paths, _ := json.Marshal(a.critical.Paths)
	return "\n\n" + criticalStart + string(paths) + criticalTasks + a.critical.Tasks + criticalEnd
}

func criticalAbs(cwd, path string) string {
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return abs
}

func (a *Agent) pinWorktree(path, branch string) {
	for i, wt := range a.critical.Paths.Worktrees {
		if wt.Path == path {
			if branch != "" {
				a.critical.Paths.Worktrees[i].Branch = branch
			}
			return
		}
	}
	a.critical.Paths.Worktrees = append(a.critical.Paths.Worktrees, criticalWorktree{path, branch})
}

// noteCriticalCall observes literal shell paths, including quoted paths and
// $UGH_SCRATCH. It never executes substitutions or trusts summarizer guesses.
// cd is local to each bash call, just as in the actual shell tool.
func (a *Agent) noteCriticalCall(tc llm.ToolCall) {
	if tc.Name != "bash" {
		return
	}
	var args struct{ Command string }
	if json.Unmarshal([]byte(tc.Args), &args) != nil {
		return
	}
	file, err := syntax.NewParser().Parse(strings.NewReader(args.Command), "")
	if err != nil {
		return
	}
	cwd := a.cfg.Cwd
	// Walk's nil callbacks close nodes. Restore shell-local cd state when
	// leaving a subshell; do not mistake a function declaration for execution.
	type scope struct {
		cwd     string
		restore bool
	}
	var scopes []scope
	syntax.Walk(file, func(node syntax.Node) bool {
		if node == nil {
			s := scopes[len(scopes)-1]
			scopes = scopes[:len(scopes)-1]
			if s.restore {
				cwd = s.cwd
			}
			return true
		}
		if _, ok := node.(*syntax.FuncDecl); ok {
			return false
		}
		call, ok := node.(*syntax.CallExpr)
		if !ok {
			_, subshell := node.(*syntax.Subshell)
			scopes = append(scopes, scope{cwd, subshell})
			return true
		}
		words := make([]string, len(call.Args))
		for i, word := range call.Args {
			var ok bool
			words[i], ok = a.criticalWord(word.Parts)
			if !ok {
				return false
			}
		}
		if len(words) == 0 {
			return false
		}
		if words[0] == "cd" {
			i := 1
			for i < len(words) && (words[i] == "--" || words[i] == "-P" || words[i] == "-L") {
				i++
			}
			if i < len(words) && words[i] != "-" {
				cwd = criticalAbs(cwd, words[i])
				if !containsString(a.critical.Paths.Directories, cwd) {
					a.critical.Paths.Directories = append(a.critical.Paths.Directories, cwd)
				}
			}
		}
		if words[0] == "git" {
			base, i := cwd, 1
			for i+1 < len(words) && words[i] == "-C" {
				base = criticalAbs(base, words[i+1])
				i += 2
			}
			if i+2 < len(words) && words[i] == "worktree" && words[i+1] == "add" {
				branch := ""
				var positional []string
				for i += 2; i < len(words); i++ {
					flag := words[i]
					switch {
					case flag == "-b" || flag == "-B" || flag == "--reason":
						i++
						if i < len(words) && flag != "--reason" {
							branch = words[i]
						}
					case strings.HasPrefix(flag, "-b") || strings.HasPrefix(flag, "-B"):
						branch = flag[2:]
					case !strings.HasPrefix(flag, "-"):
						positional = append(positional, flag)
					}
				}
				if len(positional) > 0 {
					path := criticalAbs(base, positional[0])
					if branch == "" && len(positional) > 1 {
						branch = positional[1]
					}
					if containsString(words, "--detach") || containsString(words, "-d") {
						branch = "(detached)"
					} else if branch == "" {
						branch = filepath.Base(path)
					}
					a.pinWorktree(path, branch)
				}
			}
		}
		return false
	})
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func (a *Agent) criticalWord(parts []syntax.WordPart) (string, bool) {
	var b strings.Builder
	for _, part := range parts {
		switch p := part.(type) {
		case *syntax.Lit:
			b.WriteString(p.Value)
		case *syntax.SglQuoted:
			b.WriteString(p.Value)
		case *syntax.DblQuoted:
			s, ok := a.criticalWord(p.Parts)
			if !ok {
				return "", false
			}
			b.WriteString(s)
		case *syntax.ParamExp:
			if p.Param == nil || p.Param.Value != "UGH_SCRATCH" || a.cfg.Scratch == "" ||
				p.Exp != nil || p.Slice != nil || p.Repl != nil || p.Length || p.Excl {
				return "", false
			}
			b.WriteString(a.cfg.Scratch)
		default:
			return "", false
		}
	}
	return b.String(), true
}

// mutationPathWarning is advisory, not a permission decision. Resolve symlinks
// through existing parents as write targets frequently do not exist yet.
func (a *Agent) mutationPathWarning(tc llm.ToolCall) string {
	if tc.Name != "write" && tc.Name != "edit" {
		return ""
	}
	var args struct{ Path string }
	if json.Unmarshal([]byte(tc.Args), &args) != nil || args.Path == "" {
		return ""
	}
	path := criticalRealPath(criticalAbs(a.cfg.Cwd, args.Path))
	roots := []string{a.cfg.Cwd, a.cfg.Scratch, os.TempDir()}
	for _, wt := range a.critical.Paths.Worktrees {
		roots = append(roots, wt.Path)
	}
	if a.rt != nil {
		if wt := a.rt.worktreeAt(a.cfg.Cwd); wt != nil {
			roots = append(roots, wt.Dir)
		}
	}
	for _, root := range roots {
		if root == "" {
			continue
		}
		rel, err := filepath.Rel(criticalRealPath(criticalAbs(a.cfg.Cwd, root)), path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			return ""
		}
	}
	return fmt.Sprintf("WARNING: %s target %q is outside the agent cwd, known worktree paths, UGH_SCRATCH, and temporary directories. Verify the path; do not invent a worktree location.", tc.Name, path)
}

func criticalRealPath(path string) string {
	return criticalResolvePath(path, 0)
}

func criticalResolvePath(path string, depth int) string {
	if depth >= 40 {
		return path
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	// EvalSymlinks also fails for dangling links; write may create the target.
	if target, err := os.Readlink(path); err == nil {
		return criticalResolvePath(criticalAbs(filepath.Dir(path), target), depth+1)
	}
	parent := filepath.Dir(path)
	if parent == path {
		return path
	}
	return filepath.Join(criticalResolvePath(parent, depth+1), filepath.Base(path))
}
