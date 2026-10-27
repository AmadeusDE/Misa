package makefile

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func Present(dir string) bool {
	for _, name := range []string{"Makefile", "makefile", "GNUmakefile"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func Targets(dir string) []string {
	var targets []string
	seen := map[string]bool{}

	for _, name := range []string{"Makefile", "makefile", "GNUmakefile"} {
		path := filepath.Join(dir, name)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, " ") {
				continue
			}
			if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ".") {
				continue
			}
			idx := strings.Index(line, ":")
			if idx <= 0 {
				continue
			}
			target := strings.TrimSpace(line[:idx])
			if target == "" || strings.ContainsAny(target, " \t$%") {
				continue
			}
			if !seen[target] {
				seen[target] = true
				targets = append(targets, target)
			}
		}
		f.Close()
		break
	}
	return targets
}
