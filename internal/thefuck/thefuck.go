package thefuck

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Rule struct {
	Match func(cmd, output string) bool
	Fix   func(cmd, output string) string
}

var rules []Rule

func register(r Rule) {
	rules = append(rules, r)
}

func Match(cmd, output string, exitCode int) string {
	if exitCode == 0 {
		return ""
	}
	for _, r := range rules {
		if r.Match(cmd, output) {
			if fix := r.Fix(cmd, output); fix != "" && fix != cmd {
				return fix
			}
		}
	}
	return ""
}

func commandExists(name string) bool {
	_, err := lookPath(name)
	return err == nil
}

func lookPath(name string) (string, error) {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			return path, nil
		}
	}
	return "", os.ErrNotExist
}

func similarCommands(typo string) []string {
	var candidates []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if levenshtein(typo, name) <= 2 {
				candidates = append(candidates, name)
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return levenshtein(typo, candidates[i]) < levenshtein(typo, candidates[j])
	})
	seen := map[string]bool{}
	var deduped []string
	for _, c := range candidates {
		if !seen[c] {
			seen[c] = true
			deduped = append(deduped, c)
		}
	}
	return deduped
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(cur[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, cur = cur, prev
	}
	return prev[lb]
}

func replaceCommand(cmd, oldCmd, newCmd string) string {
	fields := strings.Fields(cmd)
	if len(fields) > 0 && fields[0] == oldCmd {
		fields[0] = newCmd
		return strings.Join(fields, " ")
	}
	return newCmd + cmd[len(oldCmd):]
}
