package lscore

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func CollectGitStatus(dir string) (map[string]string, error) {
	root := findGitRoot(dir)
	if root == "" {
		return nil, nil
	}

	cmd := exec.Command("git", "status", "--porcelain", "-u")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	m := make(map[string]string)
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if len(line) < 4 {
			continue
		}
		status := strings.TrimRight(line[:2], " ")
		path := line[3:]

		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}

		rel, err := filepath.Rel(dir, filepath.Join(dir, path))
		if err != nil {
			continue
		}

		parts := strings.SplitN(rel, string(filepath.Separator), 2)
		entry := parts[0]

		if _, exists := m[entry]; !exists {
			m[entry] = status
		} else if m[entry] != status {
			m[entry] = "M"
		}
	}

	return m, nil
}

func GitStatusSymbol(status string) string {
	switch {
	case status == "":
		return " "
	case strings.Contains(status, "?"):
		return "?"
	case strings.Contains(status, "M"):
		return "M"
	case strings.Contains(status, "A"):
		return "A"
	case strings.Contains(status, "D"):
		return "D"
	case strings.Contains(status, "R"):
		return "R"
	case strings.Contains(status, "C"):
		return "C"
	case strings.Contains(status, "U"):
		return "U"
	default:
		return status
	}
}

func GitStatusColor(status string) string {
	switch GitStatusSymbol(status) {
	case "M":
		return "33" // yellow
	case "A":
		return "32" // green
	case "D":
		return "31" // red
	case "R":
		return "34" // blue
	case "?":
		return "36" // cyan
	case "U":
		return "35" // magenta
	default:
		return ""
	}
}

func findGitRoot(dir string) string {
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}
