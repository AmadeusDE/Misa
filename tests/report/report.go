// Test report generator — runs all test suites and produces a markdown report.
//
// Usage: go run ./tests/report/
//
// Generates docs/TEST_STATUS.md with per-command pass/fail/skip breakdown.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type testEvent struct {
	Time    time.Time `json:"Time"`
	Action  string    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
	Output  string    `json:"Output"`
	Elapsed float64   `json:"Elapsed"`
}

type cmdStats struct {
	Pass int
	Fail int
	Skip int
}

type suiteResult struct {
	Name     string
	Commands map[string]*cmdStats
	Total    cmdStats
}

func runSuite(pkg string, timeout string) (*suiteResult, error) {
	cmd := exec.Command("go", "test", pkg, "-v", "-json", "-count=1", "-timeout", timeout)
	cmd.Dir = filepath.Join(os.Getenv("PWD"))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	result := &suiteResult{
		Name:     pkg,
		Commands: map[string]*cmdStats{},
	}

	dec := json.NewDecoder(stdout)
	for {
		var ev testEvent
		if err := dec.Decode(&ev); err != nil {
			if err == io.EOF {
				break
			}
			break
		}

		if ev.Test == "" {
			continue
		}

		parts := strings.Split(ev.Test, "/")

		var cmdName string
		if len(parts) >= 3 {
			cmdName = parts[1]
		} else if len(parts) == 2 {
			if strings.HasSuffix(parts[0], "Suite") {
				cmdName = parts[1]
			} else if strings.HasPrefix(parts[0], "Test") {
				cmdName = extractCmdFromTestName(parts[0])
			}
		} else if len(parts) == 1 && strings.HasPrefix(parts[0], "Test") {
			if strings.HasSuffix(parts[0], "Suite") {
				continue
			}
			cmdName = extractCmdFromTestName(parts[0])
		} else {
			continue
		}

		if cmdName == "" {
			continue
		}

		if _, ok := result.Commands[cmdName]; !ok {
			result.Commands[cmdName] = &cmdStats{}
		}

		switch ev.Action {
		case "pass":
			result.Commands[cmdName].Pass++
			result.Total.Pass++
		case "fail":
			result.Commands[cmdName].Fail++
			result.Total.Fail++
		case "skip":
			result.Commands[cmdName].Skip++
			result.Total.Skip++
		}
	}

	cmd.Wait()
	return result, nil
}

func extractCmdFromTestName(name string) string {
	lower := strings.ToLower(strings.TrimPrefix(name, "Test"))
	cmds := []string{
		"sha256sum", "sha512sum", "sha1sum", "md5sum",
		"basename", "checksum", "dirname", "hostname", "unexpand",
		"logname", "pathchk", "printenv", "readlink", "realpath",
		"truncate",
		"base32", "base64", "factor", "groups", "mktemp", "printf",
		"strings", "whoami",
		"arch", "cat", "chmod", "chown", "cksum", "cmp", "comm",
		"cp", "cut", "date", "du", "echo", "env", "expand", "expr",
		"false", "fmt", "fold", "head", "id", "kill", "ln", "ls",
		"mkdir", "mktemp", "mv", "nice", "nl", "nproc",
		"paste", "proc", "pwd", "rev", "rm", "rmdir", "seq",
		"sleep", "sort", "split", "stat", "tac", "tail", "tee",
		"touch", "tr", "true", "tsort", "tty", "uname", "uniq",
		"wc", "which", "yes",
	}
	for _, cmd := range cmds {
		if strings.HasPrefix(lower, cmd) {
			return cmd
		}
	}
	i := 0
	for i < len(lower) {
		if lower[i] >= 'a' && lower[i] <= 'z' || lower[i] >= '0' && lower[i] <= '9' {
			i++
		} else {
			break
		}
	}
	if i > 0 {
		return lower[:i]
	}
	return lower
}

func statusEmoji(s *cmdStats) string {
	if s.Fail == 0 && s.Pass > 0 {
		return "PASS"
	}
	if s.Pass == 0 && s.Fail > 0 {
		return "FAIL"
	}
	if s.Pass > 0 && s.Fail > 0 {
		return "PARTIAL"
	}
	if s.Skip > 0 {
		return "SKIP"
	}
	return "-"
}

func writeReport(w io.Writer, suites []*suiteResult) {
	fmt.Fprintf(w, "# Misa Builtins — Test Status\n\n")
	fmt.Fprintf(w, "Generated: %s\n\n", time.Now().UTC().Format("2006-01-02 15:04 UTC"))

	allCmds := map[string]bool{}
	for _, s := range suites {
		for cmd := range s.Commands {
			allCmds[cmd] = true
		}
	}
	cmds := make([]string, 0, len(allCmds))
	for c := range allCmds {
		cmds = append(cmds, c)
	}
	sort.Strings(cmds)

	fmt.Fprintf(w, "## Summary\n\n")
	fmt.Fprintf(w, "| Suite | Pass | Fail | Skip | Total |\n")
	fmt.Fprintf(w, "|-------|------|------|------|-------|\n")
	grandPass, grandFail, grandSkip := 0, 0, 0
	for _, s := range suites {
		total := s.Total.Pass + s.Total.Fail + s.Total.Skip
		fmt.Fprintf(w, "| %s | %d | %d | %d | %d |\n",
			suiteName(s.Name), s.Total.Pass, s.Total.Fail, s.Total.Skip, total)
		grandPass += s.Total.Pass
		grandFail += s.Total.Fail
		grandSkip += s.Total.Skip
	}
	fmt.Fprintf(w, "| **Total** | **%d** | **%d** | **%d** | **%d** |\n\n",
		grandPass, grandFail, grandSkip, grandPass+grandFail+grandSkip)

	fmt.Fprintf(w, "## Per-Command Breakdown\n\n")
	fmt.Fprintf(w, "| Command |")
	for _, s := range suites {
		fmt.Fprintf(w, " %s |", suiteName(s.Name))
	}
	fmt.Fprintf(w, "\n|---------|")
	for range suites {
		fmt.Fprintf(w, "------|")
	}
	fmt.Fprintf(w, "\n")

	for _, cmd := range cmds {
		fmt.Fprintf(w, "| `%s` |", cmd)
		for _, s := range suites {
			st, ok := s.Commands[cmd]
			if !ok {
				fmt.Fprintf(w, " - |")
			} else {
				status := statusEmoji(st)
				fmt.Fprintf(w, " %s %d/%d/%d |", status, st.Pass, st.Fail, st.Skip)
			}
		}
		fmt.Fprintf(w, "\n")
	}

	fmt.Fprintf(w, "\nFormat: STATUS pass/fail/skip\n")
}

func suiteName(pkg string) string {
	switch {
	case strings.Contains(pkg, "native"):
		return "Native"
	case strings.Contains(pkg, "toybox"):
		return "Toybox"
	case strings.Contains(pkg, "uutils"):
		return "uutils"
	case strings.Contains(pkg, "builtins"):
		return "Unit"
	case strings.Contains(pkg, "misagnutest"):
		return "GNU/BB"
	default:
		return pkg
	}
}

func main() {
	fmt.Println("Running test suites...")

	type suiteSpec struct {
		pkg     string
		timeout string
	}
	specs := []suiteSpec{
		{"./internal/builtins/", "120s"},
		{"./tests/native/", "120s"},
		{"./tests/toybox/", "120s"},
		{"./tests/uutils/", "180s"},
	}

	var suites []*suiteResult
	for _, sp := range specs {
		fmt.Printf("  %s ...\n", sp.pkg)
		result, err := runSuite(sp.pkg, sp.timeout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  error running %s: %v\n", sp.pkg, err)
			continue
		}
		suites = append(suites, result)
	}

	outPath := "docs/TEST_STATUS.md"
	os.MkdirAll(filepath.Dir(outPath), 0o755)
	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot create %s: %v\n", outPath, err)
		os.Exit(1)
	}
	defer f.Close()

	writeReport(f, suites)
	fmt.Printf("\nReport written to %s\n", outPath)
}
