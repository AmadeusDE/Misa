package gradle

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Present(dir string) bool {
	for _, name := range []string{"build.gradle", "build.gradle.kts"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func wrapper(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, "gradlew")); err == nil {
		return filepath.Join(dir, "gradlew")
	}
	if p, err := exec.LookPath("gradle"); err == nil {
		return p
	}
	return ""
}

func Tasks(dir string) ([]string, error) {
	w := wrapper(dir)
	if w == "" {
		return nil, nil
	}
	cmd := exec.Command(w, "tasks", "--all", "-q")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var tasks []string
	inSection := false
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), "tasks") && strings.Contains(line, " ") {
			inSection = true
			continue
		}
		if line == "" {
			inSection = false
			continue
		}
		if !inSection {
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " - ", 2)
		name := strings.TrimSpace(parts[0])
		if name != "" && !strings.Contains(name, " ") {
			tasks = append(tasks, name)
		}
	}
	return tasks, nil
}
