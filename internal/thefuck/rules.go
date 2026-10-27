package thefuck

import (
	"os"
	"path/filepath"
	"strings"
)

func init() {
	register(ruleNoSuchCommand())
	register(rulePermissionDenied())
	register(rulePacmanSudo())
	register(ruleAptSudo())
	register(ruleMkdirP())
	register(ruleCdFile())
	register(ruleCdMkdir())
	register(ruleGitPushUpstream())
	register(ruleGitPushRejected())
	register(ruleGitAdd())
	register(ruleGitCheckoutBranch())
	register(ruleGitStash())
	register(rulePythonModule())
	register(ruleLsTypo())
	register(ruleNoSuchFileDir())
	register(ruleRmDir())
	register(ruleCpDir())
	register(ruleCargoWatch())
}

func ruleNoSuchCommand() Rule {
	return Rule{
		Match: func(cmd, output string) bool {
			lower := strings.ToLower(output)
			return strings.Contains(lower, "command not found") ||
				strings.Contains(lower, "not found") ||
				strings.Contains(lower, "unknown command")
		},
		Fix: func(cmd, output string) string {
			fields := strings.Fields(cmd)
			if len(fields) == 0 {
				return ""
			}
			similar := similarCommands(fields[0])
			if len(similar) == 0 {
				return ""
			}
			return replaceCommand(cmd, fields[0], similar[0])
		},
	}
}

func rulePermissionDenied() Rule {
	return Rule{
		Match: func(cmd, output string) bool {
			lower := strings.ToLower(output)
			return strings.Contains(lower, "permission denied") ||
				strings.Contains(lower, "operation not permitted")
		},
		Fix: func(cmd, output string) string {
			if strings.HasPrefix(cmd, "sudo ") {
				return ""
			}
			return "sudo " + cmd
		},
	}
}

func rulePacmanSudo() Rule {
	return Rule{
		Match: func(cmd, output string) bool {
			fields := strings.Fields(cmd)
			if len(fields) < 2 || fields[0] != "pacman" {
				return false
			}
			return strings.Contains(output, "you cannot perform this operation unless you are root") ||
				strings.Contains(output, "could not open file") ||
				strings.Contains(output, "insufficient permissions")
		},
		Fix: func(cmd, output string) string {
			return "sudo " + cmd
		},
	}
}

func ruleAptSudo() Rule {
	return Rule{
		Match: func(cmd, output string) bool {
			fields := strings.Fields(cmd)
			if len(fields) < 2 {
				return false
			}
			if fields[0] != "apt" && fields[0] != "apt-get" && fields[0] != "dpkg" {
				return false
			}
			lower := strings.ToLower(output)
			return strings.Contains(lower, "permission denied") ||
				strings.Contains(lower, "are you root") ||
				strings.Contains(lower, "unable to lock")
		},
		Fix: func(cmd, output string) string {
			return "sudo " + cmd
		},
	}
}

func ruleMkdirP() Rule {
	return Rule{
		Match: func(cmd, output string) bool {
			fields := strings.Fields(cmd)
			return len(fields) >= 2 && fields[0] == "mkdir" &&
				strings.Contains(output, "No such file or directory")
		},
		Fix: func(cmd, output string) string {
			fields := strings.Fields(cmd)
			for _, f := range fields[1:] {
				if f == "-p" {
					return ""
				}
			}
			return "mkdir -p " + strings.Join(fields[1:], " ")
		},
	}
}

func ruleCdFile() Rule {
	return Rule{
		Match: func(cmd, output string) bool {
			fields := strings.Fields(cmd)
			if len(fields) != 2 || fields[0] != "cd" {
				return false
			}
			return strings.Contains(output, "Not a directory") ||
				strings.Contains(output, "not a directory")
		},
		Fix: func(cmd, output string) string {
			fields := strings.Fields(cmd)
			return "cd " + filepath.Dir(fields[1])
		},
	}
}

func ruleCdMkdir() Rule {
	return Rule{
		Match: func(cmd, output string) bool {
			fields := strings.Fields(cmd)
			if len(fields) != 2 || fields[0] != "cd" {
				return false
			}
			return strings.Contains(output, "No such file or directory") ||
				strings.Contains(output, "no such file or directory")
		},
		Fix: func(cmd, output string) string {
			fields := strings.Fields(cmd)
			return "mkdir -p " + fields[1] + " && cd " + fields[1]
		},
	}
}

func ruleGitPushUpstream() Rule {
	return Rule{
		Match: func(cmd, output string) bool {
			return strings.HasPrefix(cmd, "git push") &&
				(strings.Contains(output, "set-upstream") ||
					strings.Contains(output, "has no upstream branch") ||
					strings.Contains(output, "git push --set-upstream"))
		},
		Fix: func(cmd, output string) string {
			for _, line := range strings.Split(output, "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "git push --set-upstream") {
					return trimmed
				}
			}
			return "git push --set-upstream origin HEAD"
		},
	}
}

func ruleGitPushRejected() Rule {
	return Rule{
		Match: func(cmd, output string) bool {
			return strings.HasPrefix(cmd, "git push") &&
				(strings.Contains(output, "rejected") ||
					strings.Contains(output, "fetch first") ||
					strings.Contains(output, "non-fast-forward"))
		},
		Fix: func(cmd, output string) string {
			return "git pull && " + cmd
		},
	}
}

func ruleGitAdd() Rule {
	return Rule{
		Match: func(cmd, output string) bool {
			return strings.HasPrefix(cmd, "git commit") &&
				(strings.Contains(output, "nothing to commit") ||
					strings.Contains(output, "no changes added") ||
					strings.Contains(output, "Changes not staged"))
		},
		Fix: func(cmd, output string) string {
			return "git add -A && " + cmd
		},
	}
}

func ruleGitCheckoutBranch() Rule {
	return Rule{
		Match: func(cmd, output string) bool {
			return (strings.HasPrefix(cmd, "git checkout") || strings.HasPrefix(cmd, "git switch")) &&
				(strings.Contains(output, "did not match") ||
					strings.Contains(output, "not a valid") ||
					strings.Contains(output, "pathspec"))
		},
		Fix: func(cmd, output string) string {
			for _, line := range strings.Split(output, "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.Contains(trimmed, "Did you mean") || strings.Contains(trimmed, "did you mean") {
					for _, word := range strings.Fields(trimmed) {
						word = strings.Trim(word, "'\"?")
						if word != "" && !strings.Contains(word, " ") && word != "Did" && word != "you" && word != "mean" && word != "did" {
							fields := strings.Fields(cmd)
							if len(fields) >= 2 {
								return fields[0] + " " + fields[1] + " " + word
							}
						}
					}
				}
			}
			return ""
		},
	}
}

func ruleGitStash() Rule {
	return Rule{
		Match: func(cmd, output string) bool {
			return (strings.HasPrefix(cmd, "git checkout") ||
				strings.HasPrefix(cmd, "git switch") ||
				strings.HasPrefix(cmd, "git pull") ||
				strings.HasPrefix(cmd, "git rebase")) &&
				(strings.Contains(output, "local changes") ||
					strings.Contains(output, "would be overwritten"))
		},
		Fix: func(cmd, output string) string {
			return "git stash && " + cmd + " && git stash pop"
		},
	}
}

func rulePythonModule() Rule {
	return Rule{
		Match: func(cmd, output string) bool {
			fields := strings.Fields(cmd)
			if len(fields) == 0 {
				return false
			}
			return (fields[0] == "python" || fields[0] == "python3") &&
				strings.Contains(output, "No module named")
		},
		Fix: func(cmd, output string) string {
			for _, line := range strings.Split(output, "\n") {
				if idx := strings.Index(line, "No module named"); idx >= 0 {
					rest := strings.TrimSpace(line[idx+len("No module named"):])
					module := strings.Trim(rest, "'\"")
					module = strings.Split(module, ".")[0]
					if module != "" {
						pip := "pip3"
						if strings.HasPrefix(cmd, "python ") {
							pip = "pip"
						}
						return pip + " install " + module + " && " + cmd
					}
				}
			}
			return ""
		},
	}
}

func ruleLsTypo() Rule {
	return Rule{
		Match: func(cmd, output string) bool {
			fields := strings.Fields(cmd)
			if len(fields) < 2 || fields[0] != "ls" {
				return false
			}
			return strings.Contains(output, "No such file or directory") ||
				strings.Contains(output, "cannot access")
		},
		Fix: func(cmd, output string) string {
			fields := strings.Fields(cmd)
			target := fields[len(fields)-1]
			if strings.HasPrefix(target, "-") {
				return ""
			}
			dir := filepath.Dir(target)
			if dir == "." {
				dir = ""
			}
			searchDir := dir
			if searchDir == "" {
				searchDir = "."
			}
			entries, err := os.ReadDir(searchDir)
			if err != nil {
				return ""
			}
			base := filepath.Base(target)
			best := ""
			bestDist := 3
			for _, e := range entries {
				d := levenshtein(base, e.Name())
				if d < bestDist {
					bestDist = d
					if dir != "" {
						best = filepath.Join(dir, e.Name())
					} else {
						best = e.Name()
					}
				}
			}
			if best == "" {
				return ""
			}
			fields[len(fields)-1] = best
			return strings.Join(fields, " ")
		},
	}
}

func ruleNoSuchFileDir() Rule {
	return Rule{
		Match: func(cmd, output string) bool {
			fields := strings.Fields(cmd)
			if len(fields) == 0 {
				return false
			}
			if fields[0] == "ls" || fields[0] == "cd" || fields[0] == "mkdir" {
				return false
			}
			return strings.Contains(output, "No such file or directory") ||
				strings.Contains(output, "no such file or directory")
		},
		Fix: func(cmd, output string) string {
			fields := strings.Fields(cmd)
			for _, f := range fields[1:] {
				if strings.HasPrefix(f, "-") {
					continue
				}
				dir := filepath.Dir(f)
				if dir != "" && dir != "." {
					if _, err := os.Stat(dir); os.IsNotExist(err) {
						return "mkdir -p " + dir + " && " + cmd
					}
				}
			}
			return ""
		},
	}
}

func ruleRmDir() Rule {
	return Rule{
		Match: func(cmd, output string) bool {
			fields := strings.Fields(cmd)
			if len(fields) < 2 || fields[0] != "rm" {
				return false
			}
			return strings.Contains(output, "Is a directory") ||
				strings.Contains(output, "is a directory")
		},
		Fix: func(cmd, output string) string {
			fields := strings.Fields(cmd)
			for _, f := range fields[1:] {
				if f == "-r" || f == "-rf" || f == "-fr" || strings.Contains(f, "r") && strings.HasPrefix(f, "-") {
					return ""
				}
			}
			return "rm -r " + strings.Join(fields[1:], " ")
		},
	}
}

func ruleCpDir() Rule {
	return Rule{
		Match: func(cmd, output string) bool {
			fields := strings.Fields(cmd)
			if len(fields) < 3 || fields[0] != "cp" {
				return false
			}
			return strings.Contains(output, "omitting directory") ||
				strings.Contains(output, "is a directory")
		},
		Fix: func(cmd, output string) string {
			fields := strings.Fields(cmd)
			for _, f := range fields[1:] {
				if f == "-r" || f == "-R" || f == "-a" {
					return ""
				}
			}
			return "cp -r " + strings.Join(fields[1:], " ")
		},
	}
}

func ruleCargoWatch() Rule {
	return Rule{
		Match: func(cmd, output string) bool {
			fields := strings.Fields(cmd)
			if len(fields) == 0 || fields[0] != "cargo" {
				return false
			}
			return strings.Contains(output, "could not find") ||
				strings.Contains(output, "use of unstable library feature")
		},
		Fix: func(cmd, output string) string {
			fields := strings.Fields(cmd)
			if len(fields) >= 2 && fields[1] == "run" {
				return "cargo build && " + cmd
			}
			return ""
		},
	}
}
