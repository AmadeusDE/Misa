// Toybox test suite adapter (toybox is 0BSD licensed).
// Parses toybox .test files and runs testcmd lines against our builtins.
package toybox

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/builtinexec"
	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/AmadeusDE/Misa/internal/proceffect"
)

var binDir string

func TestMain(m *testing.M) {
	_ = proceffect.EnableSelfHelper()
	if len(os.Args) > 1 && os.Args[1] == proceffect.Flag {
		os.Exit(builtinexec.ProcessEffect(os.Args[2:]))
	}
	tmpDir, err := os.MkdirTemp("", "misa-toybox-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	modRoot := findModRoot()
	multicall := filepath.Join(tmpDir, "multicall")
	build := exec.Command("go", "build", "-a", "-o", multicall, "./tests/toybox/testcmd/")
	build.Dir = modRoot
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build multicall: %v\n%s\n", err, out)
		os.Exit(m.Run())
	}

	symlinkDir := filepath.Join(tmpDir, "bin")
	os.MkdirAll(symlinkDir, 0755)
	for _, name := range builtins.Names() {
		os.Symlink(multicall, filepath.Join(symlinkDir, name))
	}
	binDir = symlinkDir

	os.Exit(m.Run())
}

func findModRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

type testCase struct {
	name     string
	args     string
	expected string
	infile   string
	stdin    string
	line     int
	skip     bool
}

var shellArgRe = regexp.MustCompile(`'[^']*'|"[^"]*"|[^\s]+`)

var echoRedirectRe = regexp.MustCompile(`^echo\s+(-[ne]+\s+)?(?:"([^"]*)"|'([^']*)'|(\S+))\s*>\s*(\S+)`)
var touchRe = regexp.MustCompile(`^touch\s+(.+)`)
var mkdirRe = regexp.MustCompile(`^mkdir\s+(-p\s+)?(.+)`)
var catHeredocRe = regexp.MustCompile(`^cat\s*>\s*(\S+)\s*<<\s*(\S+)`)

type setupEvent struct {
	line    int
	name    string
	content string
	isDir   bool
}

var varAssignRe = regexp.MustCompile(`^([A-Z_][A-Z0-9_]*)=(.*)`)
var envPrefixRe = regexp.MustCompile(`^TZ=\S+\s+`)

type testSuite struct {
	cmdName    string
	cases      []testCase
	setupFiles map[string]string
	setupLog   []setupEvent
	vars       map[string]string
}

func parseTestFile(path string) (testSuite, error) {
	f, err := os.Open(path)
	if err != nil {
		return testSuite{}, err
	}
	defer f.Close()

	suite := testSuite{
		cmdName:    strings.TrimSuffix(filepath.Base(path), ".test"),
		setupFiles: map[string]string{},
		vars:       map[string]string{},
	}
	if suite.cmdName == "hostname" {
		if h, err := os.Hostname(); err == nil {
			suite.vars["HOST"] = strings.TrimSpace(h)
		}
	}
	if suite.cmdName == "cp" {
		suite.setupFiles["random"] = "random bytes content\n"
		suite.setupLog = append(suite.setupLog, setupEvent{9, "random", "random bytes content\n", false})
		for _, name := range []string{"one/six", "one/seven", "one/eight", "one/two/three/five"} {
			suite.setupFiles[name] = ""
			suite.setupLog = append(suite.setupLog, setupEvent{88, name, "", false})
		}
	}
	if suite.cmdName == "expand" {
		tabstop := 1
		bigtab := " "
		for i := 0; i < 15; i++ {
			bigtab += bigtab
			tabstop *= 2
		}
		bigline := "foo "
		for i := 0; i < 15; i++ {
			bigline += bigline
		}
		suite.vars["POW"] = "15"
		suite.vars["TABSTOP"] = strconv.Itoa(tabstop)
		suite.vars["BIGTAB"] = bigtab
		suite.vars["BIGLINE"] = bigline + "\tfoo\n"
		suite.vars["EXPANDLINE"] = bigline + "        foo\n"
	}
	sc := bufio.NewScanner(f)
	lineNum := 0
	var continuation string
	var heredocFile, heredocDelim string
	var heredocBuf strings.Builder
	var multiLineEcho string

	for sc.Scan() {
		lineNum++
		line := sc.Text()

		if multiLineEcho != "" {
			multiLineEcho += "\n" + line
			if strings.Contains(line, "\"") {
				if m := echoRedirectRe.FindStringSubmatch(multiLineEcho); m != nil {
					echoFlags := strings.TrimSpace(m[1])
					content := m[2]
					if content == "" {
						content = m[3]
					}
					if content == "" {
						content = m[4]
					}
					content = strings.ReplaceAll(content, "$(which sh)", "/bin/sh")
					content = unescapeTestStr(content)
					if !strings.Contains(echoFlags, "n") {
						content += "\n"
					}
					fname := m[5]
					suite.setupFiles[fname] = content
					suite.setupLog = append(suite.setupLog, setupEvent{lineNum, fname, content, false})
				}
				multiLineEcho = ""
			}
			continue
		}

		if heredocDelim != "" {
			if strings.TrimSpace(line) == heredocDelim {
				suite.setupFiles[heredocFile] = heredocBuf.String()
				suite.setupLog = append(suite.setupLog, setupEvent{lineNum, heredocFile, heredocBuf.String(), false})
				heredocDelim = ""
				heredocFile = ""
				heredocBuf.Reset()
			} else {
				heredocBuf.WriteString(line)
				heredocBuf.WriteByte('\n')
			}
			continue
		}

		if continuation != "" {
			line = continuation + " " + strings.TrimSpace(line)
			continuation = ""
		}

		if strings.HasSuffix(strings.TrimSpace(line), "\\") {
			continuation = strings.TrimSuffix(strings.TrimSpace(line), "\\")
			continue
		}

		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "export ")

		if strings.HasPrefix(line, "chmod ") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				mode := fields[1]
				for _, f := range fields[2:] {
					if !strings.HasPrefix(f, "-") {
						suite.setupLog = append(suite.setupLog, setupEvent{lineNum, f, "chmod:" + mode, false})
					}
				}
			}
		}

		if strings.Contains(line, "=") && !strings.HasPrefix(line, "testing") && !strings.HasPrefix(line, "testcmd") {
			l := line
			for len(l) > 0 {
				idx := strings.IndexByte(l, '=')
				if idx <= 0 {
					break
				}
				key := strings.TrimSpace(l[:idx])
				rest := l[idx+1:]
				val := ""
				if len(rest) > 0 && (rest[0] == '"' || rest[0] == '\'') {
					q := rest[0]
					if end := strings.IndexByte(rest[1:], q); end >= 0 {
						val = rest[1 : end+1]
						l = strings.TrimSpace(rest[end+2:])
					} else {
						val = strings.TrimSpace(rest[1:])
						l = ""
					}
				} else {
					fields := strings.Fields(rest)
					if len(fields) > 0 {
						val = fields[0]
						l = strings.TrimSpace(rest[len(val):])
					} else {
						val = rest
						l = ""
					}
				}
				if key != "" && (key[0] >= 'A' && key[0] <= 'Z' || key[0] >= 'a' && key[0] <= 'z') {
					if (suite.cmdName == "hostname" && key == "HOST") || (suite.cmdName == "expand" && (key == "TABSTOP" || key == "BIGTAB" || key == "BIGLINE" || key == "EXPANDLINE" || key == "POW")) {
						// Keep precalculated values for hostname and expand suites
						continue
					}
					if strings.HasPrefix(val, "$(echo -e ") || strings.HasPrefix(val, "$(echo ") {
						val = strings.TrimPrefix(val, "$(echo -e ")
						val = strings.TrimPrefix(val, "$(echo ")
						val = strings.TrimSuffix(val, ")")
						val = strings.Trim(val, "'\"")
					}
					suite.vars[key] = unescapeTestStr(val)
				}
			}
		}

		if m := catHeredocRe.FindStringSubmatch(line); m != nil {
			heredocFile = m[1]
			heredocDelim = m[2]
			heredocBuf.Reset()
			continue
		}

		if m := echoRedirectRe.FindStringSubmatch(line); m != nil {
			echoFlags := strings.TrimSpace(m[1])
			content := m[2]
			isDQ := content != ""
			if content == "" {
				content = m[3]
			}
			if content == "" {
				content = m[4]
				isDQ = true
			}
			if isDQ {
				content = expandVars(content, suite.vars)
			}
			content = strings.ReplaceAll(content, "$(which sh)", "/bin/sh")
			content = unescapeTestStr(content)
			if !strings.Contains(echoFlags, "n") {
				content += "\n"
			}
			fname := m[5]
			suite.setupFiles[fname] = content
			suite.setupLog = append(suite.setupLog, setupEvent{lineNum, fname, content, false})
		} else if m := regexp.MustCompile(`\btouch\s+([^;&|]+)`).FindStringSubmatch(line); m != nil {
			touchArgs := splitShellArgs(m[1])
			for i := 0; i < len(touchArgs); i++ {
				a := touchArgs[i]
				a = strings.Trim(a, "'\"")
				if strings.HasPrefix(a, "-") {
					if strings.ContainsAny(a, "admrtBD") && i+1 < len(touchArgs) {
						i++
					}
					continue
				}
				if a == "warrus" || a == "" {
					continue
				}
				if _, err := strconv.Atoi(a); err == nil {
					continue
				}
				suite.setupFiles[a] = ""
				suite.setupLog = append(suite.setupLog, setupEvent{lineNum, a, "", false})
			}
		} else if m := mkdirRe.FindStringSubmatch(line); m != nil {
			for _, name := range strings.Fields(m[2]) {
				suite.setupLog = append(suite.setupLog, setupEvent{lineNum, name, "", true})
			}
		} else if m := regexp.MustCompile(`^(?:rmdir|rm(?:\s+-[rf]+)?)\s+(.+)`).FindStringSubmatch(line); m != nil {
			for _, name := range strings.Fields(m[1]) {
				name = strings.Trim(name, "'\"")
				if name != "" && !strings.HasPrefix(name, "-") {
					suite.setupLog = append(suite.setupLog, setupEvent{lineNum, name, "rm", false})
				}
			}
		} else if m := regexp.MustCompile(`^umask\s+([0-7]+)`).FindStringSubmatch(line); m != nil {
			suite.setupLog = append(suite.setupLog, setupEvent{lineNum, "umask", m[1], false})
		} else if strings.HasPrefix(line, "ln ") || strings.HasPrefix(line, "cp ") || (strings.Contains(line, ";") && !strings.HasPrefix(line, "testcmd") && !strings.HasPrefix(line, "testing")) {
			suite.setupLog = append(suite.setupLog, setupEvent{lineNum, "exec", line, false})
		} else if strings.HasPrefix(line, "echo ") && strings.Count(line, "\"")-strings.Count(line, "\\\"") == 1 {
			multiLineEcho = line
			continue
		}

		skip := false
		if strings.HasPrefix(line, "toyonly ") {
			line = strings.TrimPrefix(line, "toyonly ")
			line = strings.TrimSpace(line)
			skip = true
		}

		if suite.cmdName == "chmod" && strings.HasPrefix(line, "chtest ") {
			fields, err := extractQuotedFields(line[len("chtest "):])
			if err == nil && len(fields) >= 2 {
				mode := fields[0]
				exp := unescapeTestStr(fields[1])
				suite.cases = append(suite.cases, testCase{
					name:     mode + " dir file",
					args:     mode + " dir file && ls -ld dir file | cut -d' ' -f 1 | cut -d. -f 1",
					expected: exp,
					line:     lineNum,
				})
			}
			continue
		}

		if strings.HasPrefix(line, "testcmd ") {
			if strings.Contains(line, "$1") {
				continue
			}
			if suite.cmdName == "chmod" && strings.Contains(line, "$U$G$O") {
				perms := []string{"---", "--x", "-w-", "-wx", "r--", "r-x", "rw-", "rwx"}
				num2perm := func(u, g, o int) string {
					return perms[u] + perms[g] + perms[o]
				}
				for u := 0; u <= 7; u++ {
					for _, g := range []int{0, 3, 6} {
						for _, o := range []int{0, 7} {
							for _, tName := range []string{"dir", "file"} {
								dChar := "d"
								if tName == "file" {
									dChar = "-"
								}
								modeStr := fmt.Sprintf("%d%d%d", u, g, o)
								tcName := modeStr + " " + tName
								suite.cases = append(suite.cases, testCase{
									name:     tcName,
									args:     modeStr + " " + tName + " && ls -ld " + tName + " | cut -d' ' -f 1",
									expected: dChar + num2perm(u, g, o) + "\n",
									line:     lineNum,
								})
							}
						}
					}
				}
				continue
			}
			tc, err := parseTestcmd(line[len("testcmd "):], lineNum, skip, suite.vars)
			if err != nil {
				continue
			}
			if suite.cmdName == "cp" && tc.name == "file->dir/file" {
				suite.setupLog = append(suite.setupLog, setupEvent{lineNum, "exec", "cp random two/random", false})
			}
			suite.cases = append(suite.cases, tc)
		} else if strings.HasPrefix(line, "testing ") {
			tc, err := parseTesting(line[len("testing "):], suite.cmdName, lineNum, skip, suite.vars)
			if err != nil {
				continue
			}
			if suite.cmdName == "hostname" && tc.name == "set" && os.Getuid() != 0 {
				tc.skip = true
			}
			suite.cases = append(suite.cases, tc)
		}
	}
	sort.SliceStable(suite.setupLog, func(i, j int) bool {
		return suite.setupLog[i].line < suite.setupLog[j].line
	})
	return suite, sc.Err()
}

func parseTestcmd(s string, lineNum int, skip bool, vars map[string]string) (testCase, error) {
	fields, err := extractQuotedFields(s)
	if err != nil || len(fields) < 3 {
		return testCase{}, fmt.Errorf("parse error line %d", lineNum)
	}

	for len(fields) < 5 {
		fields = append(fields, "")
	}

	tc := testCase{
		name:     fields[0],
		args:     expandVars(fields[1], vars),
		expected: unescapeTestStr(expandVars(fields[2], vars)),
		infile:   unescapeTestStr(expandVars(fields[3], vars)),
		stdin:    unescapeTestStr(expandVars(fields[4], vars)),
		line:     lineNum,
		skip:     skip,
	}
	return tc, nil
}

func parseTesting(s string, cmdName string, lineNum int, skip bool, vars map[string]string) (testCase, error) {
	fields, err := extractQuotedFields(s)
	if err != nil || len(fields) < 3 {
		return testCase{}, fmt.Errorf("parse error line %d", lineNum)
	}

	for len(fields) < 5 {
		fields = append(fields, "")
	}

	fullCmd := fields[1]
	parts := splitShellArgs(fullCmd)
	if len(parts) == 0 {
		return testCase{}, fmt.Errorf("empty command line %d", lineNum)
	}

	if parts[0] != cmdName {
		return testCase{}, fmt.Errorf("command %q != %q at line %d", parts[0], cmdName, lineNum)
	}

	argsStr := ""
	if len(parts) > 1 {
		argsStr = strings.TrimPrefix(fullCmd, parts[0])
		argsStr = strings.TrimLeft(argsStr, " \t")
	}

	tc := testCase{
		name:     fields[0],
		args:     expandVars(argsStr, vars),
		expected: unescapeTestStr(expandVars(fields[2], vars)),
		infile:   unescapeTestStr(expandVars(fields[3], vars)),
		stdin:    unescapeTestStr(expandVars(fields[4], vars)),
		line:     lineNum,
		skip:     skip,
	}
	return tc, nil
}

func unescapeDQ(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			next := s[i+1]
			if next == '\\' || next == '"' || next == '$' || next == '`' {
				out.WriteByte(next)
				i++
				continue
			}
		}
		out.WriteByte(s[i])
	}
	return out.String()
}

func extractQuotedFields(s string) ([]string, error) {
	var fields []string
	i := 0
	for i < len(s) {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= len(s) {
			break
		}

		var field strings.Builder
		for i < len(s) && s[i] != ' ' && s[i] != '\t' {
			if s[i] == '"' {
				end := findClosingQuote(s, i+1, '"')
				if end < 0 {
					return nil, fmt.Errorf("unterminated double quote")
				}
				field.WriteString(unescapeDQ(s[i+1 : end]))
				i = end + 1
			} else if s[i] == '\'' {
				end := findClosingQuote(s, i+1, '\'')
				if end < 0 {
					return nil, fmt.Errorf("unterminated single quote")
				}
				field.WriteString(s[i+1 : end])
				i = end + 1
			} else {
				start := i
				for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '"' && s[i] != '\'' {
					i++
				}
				field.WriteString(s[start:i])
			}
		}
		fields = append(fields, field.String())
	}
	return fields, nil
}

func expandVars(s string, vars map[string]string) string {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '$' && i+1 < len(s) {
			if i+2 < len(s) && s[i+1] == '(' && s[i+2] == '(' {
				end := strings.Index(s[i+3:], "))")
				if end >= 0 {
					expr := s[i+3 : i+3+end]
					for k, v := range vars {
						expr = strings.ReplaceAll(expr, k, v)
					}
					if parts := strings.Split(expr, "+"); len(parts) == 2 {
						a, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
						b, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
						out.WriteString(strconv.Itoa(a + b))
						i += 3 + end + 1
						continue
					} else if parts := strings.Split(expr, "*"); len(parts) == 2 {
						a, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
						b, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
						out.WriteString(strconv.Itoa(a * b))
						i += 3 + end + 1
						continue
					}
				}
			}
			if s[i+1] == '{' {
				k := i + 2
				for k < len(s) && s[k] != '}' {
					k++
				}
				if k < len(s) {
					name := s[i+2 : k]
					if val, ok := vars[name]; ok {
						out.WriteString(val)
					}
					i = k
					continue
				}
			}
			j := i + 1
			for j < len(s) && (s[j] == '_' || (s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= '0' && s[j] <= '9')) {
				j++
			}
			if j > i+1 {
				name := s[i+1 : j]
				if val, ok := vars[name]; ok {
					out.WriteString(val)
					i = j - 1
					continue
				}
			}
		}
		out.WriteByte(s[i])
	}
	return out.String()
}

func findClosingQuote(s string, start int, quote byte) int {
	for i := start; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			continue
		}
		if s[i] == quote {
			return i
		}
	}
	return -1
}

func unhex(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

func unescapeTestStr(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				out.WriteByte('\n')
			case 't':
				out.WriteByte('\t')
			case 'r':
				out.WriteByte('\r')
			case '\\':
				out.WriteByte('\\')
			case 'a':
				out.WriteByte('\a')
			case 'b':
				out.WriteByte('\b')
			case 'v':
				out.WriteByte('\v')
			case 'f':
				out.WriteByte('\f')
			case 'e':
				out.WriteByte(0x1b)
			case 'x':
				if i+2 < len(s) {
					hi := unhex(s[i+1])
					lo := unhex(s[i+2])
					if hi >= 0 && lo >= 0 {
						out.WriteByte(byte(hi<<4 | lo))
						i += 2
					} else {
						out.WriteByte('\\')
						out.WriteByte(s[i])
					}
				} else {
					out.WriteByte('\\')
					out.WriteByte(s[i])
				}
			case '0':
				val := 0
				count := 0
				for count < 3 && i < len(s) && s[i] >= '0' && s[i] <= '7' {
					val = val*8 + int(s[i]-'0')
					i++
					count++
				}
				i--
				out.WriteByte(byte(val))
			case '$':
				out.WriteByte('$')
			default:
				out.WriteByte('\\')
				out.WriteByte(s[i])
			}
		} else {
			out.WriteByte(s[i])
		}
		i++
	}
	return out.String()
}

func splitShellArgs(s string) []string {
	var args []string
	i := 0
	for i < len(s) {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= len(s) {
			break
		}

		var arg strings.Builder
		if s[i] == '\'' {
			i++
			for i < len(s) && s[i] != '\'' {
				arg.WriteByte(s[i])
				i++
			}
			if i < len(s) {
				i++
			}
		} else if s[i] == '"' {
			i++
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' && i+1 < len(s) {
					i++
				}
				arg.WriteByte(s[i])
				i++
			}
			if i < len(s) {
				i++
			}
		} else {
			for i < len(s) && s[i] != ' ' && s[i] != '\t' {
				if s[i] == '\\' && i+1 < len(s) {
					i++
				}
				arg.WriteByte(s[i])
				i++
			}
		}
		args = append(args, arg.String())
	}
	return args
}

func canRunDirectly(args string) bool {
	return !strings.ContainsAny(args, "|&;<>$`()")
}

// restorePermissions recursively chmod 755/644 everything under dir so
// t.TempDir() cleanup can remove it. Tests that chmod 000 dirs leave
// unremovable trees that accumulate in /tmp and can break sandbox state.
func restorePermissions(dir string) {
	os.Chmod(dir, 0755)
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			os.Chmod(path, 0755)
		} else {
			os.Chmod(path, 0644)
		}
		return nil
	})
}

var sandboxUnsafeRe = regexp.MustCompile(
	`\bmount\b|\bumount\b|/dev/zero|/dev/full|/dev/urandom|\bmknod\b|\bchroot\b|\bunshare\b`,
)

func buildSetupAt(log []setupEvent, atLine int) map[string]string {
	files := map[string]string{}
	for _, ev := range log {
		if ev.line > atLine {
			break
		}
		files[ev.name] = ev.content
	}
	return files
}

func runTestCase(t *testing.T, cmdName string, tc testCase, setupLog []setupEvent, vars map[string]string) {
	t.Helper()
	skipKnownFailure(t)

	if tc.skip {
		t.Skip("toyonly test")
	}

	args := tc.args
	if strings.Contains(args, "&&") || strings.Contains(args, "||") ||
		strings.Contains(args, "|") || strings.Contains(args, ">") ||
		strings.Contains(args, "$") || strings.Contains(args, "`") ||
		strings.Contains(args, ";") {
		runTestCaseShell(t, cmdName, tc, setupLog, vars)
		return
	}

	if sandboxUnsafeRe.MatchString(args) || sandboxUnsafeRe.MatchString(tc.stdin) {
		t.Skip("sandbox-unsafe operation")
	}

	if strings.Contains(args, "/dev/full") || strings.Contains(args, "/dev/null") {
		t.Skip("device access needed")
	}

	parsedArgs := splitShellArgs(args)

	pathCmds := map[string]bool{
		"basename": true, "dirname": true, "realpath": true, "readlink": true,
	}
	if !pathCmds[cmdName] {
		for _, a := range parsedArgs {
			if a == "input" && tc.infile != "" {
				continue
			}
			if strings.HasPrefix(a, "-") || a == "-" {
				continue
			}
			if !filepath.IsAbs(a) {
				continue
			}
			if _, err := os.Stat(a); err != nil {
				t.Skipf("references external absolute path %q", a)
			}
		}
	}

	cmd, ok := builtins.Lookup(cmdName)
	if !ok {
		t.Skipf("builtin %q not registered", cmdName)
	}

	dir := t.TempDir()
	t.Cleanup(func() { restorePermissions(dir) })

	for _, ev := range setupLog {
		if ev.line > tc.line {
			continue
		}
		if ev.name == "umask" {
			continue
		}
		if ev.name == "ln" || ev.name == "exec" {
			c := exec.Command("bash", "-c", "PATH="+binDir+":$PATH; "+ev.content)
			c.Dir = dir
			c.Run()
		} else if strings.HasPrefix(ev.content, "chmod:") {
			modeStr := strings.TrimPrefix(ev.content, "chmod:")
			if modeUint, err := strconv.ParseUint(modeStr, 8, 32); err == nil {
				os.Chmod(filepath.Join(dir, ev.name), os.FileMode(modeUint))
			} else {
				os.Chmod(filepath.Join(dir, ev.name), 0755)
			}
		} else if ev.content == "chmod755" {
			os.Chmod(filepath.Join(dir, ev.name), 0755)
		} else if ev.content == "rm" {
			os.RemoveAll(filepath.Join(dir, ev.name))
		} else if ev.isDir {
			os.MkdirAll(filepath.Join(dir, ev.name), 0755)
		} else {
			p := filepath.Join(dir, ev.name)
			os.MkdirAll(filepath.Dir(p), 0755)
			os.WriteFile(p, []byte(ev.content), 0644)
		}
	}

	if tc.infile != "" {
		os.WriteFile(filepath.Join(dir, "input"), []byte(tc.infile), 0644)
	}

	if cmdName == "wc" && args == "-m file1" {
		var b strings.Builder
		b.WriteString(" ")
		for i := 0; i < 512; i++ {
			b.WriteString("üüüüüüüüüüüüüüüü")
		}
		os.WriteFile(filepath.Join(dir, "file1"), []byte(b.String()), 0644)
	}
	if cmdName == "stat" {
		t.Setenv("TZ", "utc")
		filePath := filepath.Join(dir, "file")
		os.WriteFile(filePath, []byte(""), 0644)
		atime := time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)
		mtime := time.Date(2000, 2, 2, 2, 2, 2, 0, time.UTC)
		if strings.Contains(args, "%y") || strings.Contains(args, "%Y") {
			os.Chtimes(filePath, atime, mtime)
		} else {
			os.Chtimes(filePath, atime, atime)
		}
	}

	var stdout, stderr bytes.Buffer
	ctx := &builtins.Context{
		Dir:     dir,
		Args:    parsedArgs,
		CmdName: cmdName,
		Width:   80,
		Height:  24,
		Stdout:  &stdout,
		Stderr:  &stderr,
		Stdin:   strings.NewReader(tc.stdin),
	}
	cmd.Run(ctx)

	got := stdout.String()
	if got != tc.expected {
		t.Errorf("stdout mismatch:\n  got:  %q\n  want: %q\n  args: %q\n  stderr: %q",
			got, tc.expected, args, stderr.String())
	}
}

func runTestCaseShell(t *testing.T, cmdName string, tc testCase, setupLog []setupEvent, vars map[string]string) {
	t.Helper()
	syscall.Umask(0022)
	if binDir == "" {
		t.Skip("multicall binary not available")
		return
	}

	argsStr := tc.args
	if strings.HasPrefix(argsStr, cmdName+" ") {
		argsStr = strings.TrimPrefix(argsStr, cmdName+" ")
	}
	fullCmd := "enable -n " + cmdName + " 2>/dev/null; " + cmdName + " " + argsStr
	if strings.Contains(fullCmd, "/dev/full") {
		t.Skip("sandbox-unsafe /dev/full test")
	}
	if strings.Contains(fullCmd, "$SIZE") {
		fullCmd = strings.ReplaceAll(fullCmd, "$SIZE", "&& stat -c %s freep")
	}
	if cmdName == "touch" && strings.Contains(fullCmd, "touch -r walrus walrus2") {
		fullCmd = "touch -d 2009-02-13T23:31:30.123456789Z walrus && " + fullCmd
	}
	if cmdName == "seq" && strings.Contains(fullCmd, "seq -f \"$i\"") {
		fullCmd = "i='%g'; " + fullCmd
	}
	var lastUmask string
	for _, ev := range setupLog {
		if ev.line > tc.line {
			continue
		}
		if ev.name == "umask" {
			lastUmask = ev.content
		}
	}
	if lastUmask != "" {
		fullCmd = "umask " + lastUmask + "; " + fullCmd
	}
	dir := t.TempDir()
	t.Cleanup(func() { restorePermissions(dir) })

	for _, ev := range setupLog {
		if ev.line > tc.line {
			continue
		}
		t.Logf("setup ev: line %d, name %q, content %q, isDir %v", ev.line, ev.name, ev.content, ev.isDir)
		if ev.name == "umask" {
			continue
		}
		if ev.name == "ln" || ev.name == "exec" {
			c := exec.Command("bash", "-c", "PATH="+binDir+":$PATH; "+ev.content)
			c.Dir = dir
			if out, err := c.CombinedOutput(); err != nil {
				t.Logf("setup %s failed: %v: %s", ev.name, err, out)
			}
		} else if strings.HasPrefix(ev.content, "chmod:") {
			modeStr := strings.TrimPrefix(ev.content, "chmod:")
			if modeUint, err := strconv.ParseUint(modeStr, 8, 32); err == nil {
				os.Chmod(filepath.Join(dir, ev.name), os.FileMode(modeUint))
			} else {
				os.Chmod(filepath.Join(dir, ev.name), 0755)
			}
		} else if ev.content == "chmod755" {
			os.Chmod(filepath.Join(dir, ev.name), 0755)
		} else if ev.content == "rm" {
			os.RemoveAll(filepath.Join(dir, ev.name))
		} else if ev.isDir {
			restorePermissions(dir)
			targetPath := filepath.Join(dir, ev.name)
			os.MkdirAll(targetPath, 0755)
			os.Chmod(targetPath, 0755)
		} else {
			restorePermissions(dir)
			p := filepath.Join(dir, ev.name)
			os.MkdirAll(filepath.Dir(p), 0755)
			os.WriteFile(p, []byte(ev.content), 0644)
		}
	}

	if cmdName == "mkdir" && tc.name == "-vp exists" {
		os.MkdirAll(filepath.Join(dir, "walrus"), 0755)
	}

	if cmdName == "fallocate" && (tc.name == "shorter" || tc.name == "longer") {
		// toybox's cases share foo with the earlier "simple" case; this adapter
		// gives every case a fresh directory, so seed the 123-byte file.
		os.WriteFile(filepath.Join(dir, "foo"), bytes.Repeat([]byte("x"), 123), 0644)
	}

	if cmdName == "chmod" {
		dirPath := filepath.Join(dir, "dir")
		filePath := filepath.Join(dir, "file")
		os.MkdirAll(dirPath, 0755)
		os.Chmod(dirPath, 0755)
		os.WriteFile(filePath, []byte(""), 0644)
		os.Chmod(filePath, 0644)
		if strings.Contains(tc.name, "symlink") {
			os.MkdirAll(filepath.Join(dir, "foo"), 0755)
			os.Symlink("bar", filepath.Join(dir, "foo/baz"))
		}
		if tc.name == "-c" || tc.name == "-v" {
			os.WriteFile(filepath.Join(dir, "one"), []byte(""), 0644)
		}
	}

	if tc.infile != "" {
		os.WriteFile(filepath.Join(dir, "input"), []byte(tc.infile), 0644)
	}

	if cmdName == "wc" && strings.Contains(fullCmd, "-m file1") {
		var b strings.Builder
		b.WriteString(" ")
		for i := 0; i < 512; i++ {
			b.WriteString("üüüüüüüüüüüüüüüü")
		}
		os.WriteFile(filepath.Join(dir, "file1"), []byte(b.String()), 0644)
	}
	if cmdName == "stat" {
		filePath := filepath.Join(dir, "file")
		os.WriteFile(filePath, []byte(""), 0644)
		atime := time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)
		mtime := time.Date(2000, 2, 2, 2, 2, 2, 0, time.UTC)
		if strings.Contains(fullCmd, "%y") || strings.Contains(fullCmd, "%Y") {
			os.Chtimes(filePath, atime, mtime)
		} else {
			os.Chtimes(filePath, atime, atime)
		}
	}
	if cmdName == "tail" && strings.Contains(fullCmd, "bigfile") {
		var b strings.Builder
		for j := 1; j <= 100; j++ {
			fmt.Fprintf(&b, "%d ", j)
			for i := 0; i < 100; i++ {
				b.WriteString("123456789abcefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
			}
			b.WriteByte('\n')
		}
		os.WriteFile(filepath.Join(dir, "bigfile"), []byte(b.String()), 0644)
	}

	if cmdName == "mv" {
		if strings.Contains(fullCmd, "stat -c %s file2") {
			os.WriteFile(filepath.Join(dir, "file1"), make([]byte, 5243392), 0644)
		}
		if strings.Contains(fullCmd, "readlink link2") {
			os.WriteFile(filepath.Join(dir, "file1"), []byte(""), 0644)
			os.Symlink("file1", filepath.Join(dir, "link1"))
		}
		if strings.Contains(fullCmd, "file1 -ef link2") {
			os.WriteFile(filepath.Join(dir, "file1"), []byte(""), 0644)
			os.Link(filepath.Join(dir, "file1"), filepath.Join(dir, "link1"))
		}
		if strings.Contains(fullCmd, "mv dir1/dir2") {
			os.RemoveAll(filepath.Join(dir, "dir2"))
		}
		if strings.Contains(fullCmd, "mv -i file2 file1") || strings.Contains(fullCmd, "mv file2 file1") {
			os.WriteFile(filepath.Join(dir, "file1"), []byte(""), 0400)
			os.WriteFile(filepath.Join(dir, "file2"), []byte(""), 0400)
		}
	}

	if cmdName == "strings" {
		t.Logf("cmdName=strings fullCmd=%q infile=%q stdin=%q", fullCmd, tc.infile, tc.stdin)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", fullCmd)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(tc.stdin)
	env := append(os.Environ(),
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"C="+filepath.Join(binDir, cmdName),
	)
	for k, v := range vars {
		if !strings.ContainsRune(v, 0) {
			if k == "PATH" {
				env = append(env, "PATH="+v+":"+binDir+":"+os.Getenv("PATH"))
			} else {
				env = append(env, k+"="+v)
			}
		}
	}
	cmd.Env = env
	cmd.Stdin = strings.NewReader(tc.stdin)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Run()

	got := stdout.String()
	expected := tc.expected
	if strings.Contains(expected, "$(") || strings.Contains(expected, "`") {
		expCmd := exec.CommandContext(ctx, "bash", "-c", "printf '%s' \""+expected+"\"")
		expCmd.Dir = dir
		expCmd.Env = cmd.Env
		if out, err := expCmd.Output(); err == nil {
			expected = string(out)
		}
	}

	if got != expected {
		if cmdName == "strings" {
			t.Logf("STRINGS FAIL: fullCmd=%q got=%q expected=%q stderr=%q stdinLen=%d", fullCmd, got, expected, stderr.String(), len(tc.stdin))
		}
		t.Errorf("stdout mismatch:\n  got:  %q\n  want: %q\n  cmd:  %q\n  stderr: %q",
			got, tc.expected, fullCmd, stderr.String())
	}
}

func TestToyboxSuite(t *testing.T) {
	testDir := "."
	if _, err := os.Stat(testDir); err != nil {
		testDir = "tests/toybox"
	}

	entries, err := os.ReadDir(testDir)
	if err != nil {
		t.Skipf("toybox test directory not found: %v", err)
	}

	var totalParsed, totalRun int

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".test") {
			continue
		}

		path := filepath.Join(testDir, e.Name())
		suite, err := parseTestFile(path)
		if err != nil {
			t.Logf("WARN: failed to parse %s: %v", e.Name(), err)
			continue
		}

		if len(suite.cases) == 0 {
			continue
		}

		if _, ok := builtins.Lookup(suite.cmdName); !ok {
			continue
		}

		if suite.cmdName == "fallocate" && runtime.GOOS != "linux" {
			// fallocate(2) is Linux-only for Go; other platforms fail honestly.
			t.Logf("skipping %s: fallocate is unsupported on %s", e.Name(), runtime.GOOS)
			continue
		}

		totalParsed += len(suite.cases)

		t.Run(suite.cmdName, func(t *testing.T) {
			for _, tc := range suite.cases {
				tc := tc
				name := tc.name
				if name == "" {
					name = fmt.Sprintf("line%d", tc.line)
				}
				t.Run(name, func(t *testing.T) {
					totalRun++
					runTestCase(t, suite.cmdName, tc, suite.setupLog, suite.vars)
				})
			}
		})
	}

	t.Logf("Toybox tests: %d parsed from test files", totalParsed)
}
