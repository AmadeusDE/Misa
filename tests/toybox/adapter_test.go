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
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/AmadeusDE/Misa/tests/testaccount"
)

var binDir string

// fixtureDir holds the copied upstream fixture files (tests/files/... in the
// toybox tree). Upstream suites reach them through $FILES, so the adapter
// exports it for setup commands and shell-path invocations.
var fixtureDir string

func TestMain(m *testing.M) {
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
	// Only these fixed host helpers are made available to fixture shells.
	for _, name := range []string{"sed", "grep", "egrep", "od"} {
		for _, base := range []string{"/usr/bin", "/bin"} {
			path := filepath.Join(base, name)
			if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
				os.Symlink(path, filepath.Join(symlinkDir, name))
				break
			}
		}
	}
	binDir = symlinkDir
	if info, err := os.Stat(filepath.Join(modRoot, "tests", "toybox", "files")); err == nil && info.IsDir() {
		fixtureDir = filepath.Join(modRoot, "tests", "toybox", "files")
	}

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
	// nospace mirrors toybox's NOSPACE=1 harness directive: compare output with
	// whitespace differences ignored (scripts/runtest.sh uses diff -auw).
	nospace bool
}

var shellArgRe = regexp.MustCompile(`'[^']*'|"[^"]*"|[^\s]+`)

var echoRedirectRe = regexp.MustCompile(`^echo\s+(-[ne]+\s+)?(?:"([^"]*)"|'([^']*)'|(\S+))\s*>\s*(\S+)`)
var touchRe = regexp.MustCompile(`^touch\s+(.+)`)
var mkdirRe = regexp.MustCompile(`^mkdir\s+(-p\s+)?(.+)`)
var catHeredocRe = regexp.MustCompile(`^cat\s*>\s*(\S+)\s*<<\s*(\S+)`)
var echoPipeRedirectRe = regexp.MustCompile(`^echo\s+.*\|\s*\S+.*>\s*\S+$`)

type setupEvent struct {
	line    int
	name    string
	content string
	isDir   bool
}

var varAssignRe = regexp.MustCompile(`^([A-Z_][A-Z0-9_]*)=(.*)`)
var envPrefixRe = regexp.MustCompile(`^TZ=\S+\s+`)

// nospaceSuites lists suites whose vendored cases carry toybox's NOSPACE=1
// directive: those cases compare output with whitespace ignored (sameOutput), the
// way upstream runs them with diff -w. Other suites keep dropping such cases so
// their F07 accounting is untouched.
var nospaceSuites = map[string]bool{"readelf": true}

// multilineCaseSuites lists suites whose golden expected blocks span several
// lines inside one quoted argument. Other suites write such blocks too, but
// only for commands whose fixture variables this adapter cannot expand yet, so
// joining them there would change their accounting without adding coverage.
var multilineCaseSuites = map[string]bool{"readelf": true}

// toyonlyRunSuites lists suites whose toyonly cases Misa runs anyway: upstream
// marks them toyonly because binutils differs from toybox, and these builtins
// are toybox ports, so the toybox expectation is the one that applies.
var toyonlyRunSuites = map[string]bool{"readelf": true}

type unparsedCase struct {
	line   int
	reason string
}

type testSuite struct {
	unparsed   []unparsedCase
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
	// caseBuf accumulates a testcmd/testing line whose quoted arguments span
	// several lines (readelf.test writes its expected blocks that way).
	var caseBuf string
	var caseBufSkip, caseBufNoSpace bool

	for sc.Scan() {
		lineNum++
		line := sc.Text()
		nospace := false

		if caseBuf != "" {
			caseBuf += "\n" + line
			if !quotesBalanced(caseBuf) {
				if len(caseBuf) > 256*1024 {
					suite.unparsed = append(suite.unparsed, unparsedCase{lineNum, "multiline case too long"})
					caseBuf, caseBufSkip, caseBufNoSpace = "", false, false
				}
				continue
			}
			line = caseBuf
			if caseBufSkip {
				line = "toyonly " + line
			}
			nospace = caseBufNoSpace
			caseBuf, caseBufSkip, caseBufNoSpace = "", false, false
		}

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
		// NOSPACE=1 is a toybox harness directive (whitespace-insensitive diff),
		// not a command option. It is honoured for the suites in nospaceSuites;
		// elsewhere the token stays part of an undiscovered line, because running
		// those cases would change their accounting for pre-existing
		// output-format differences owned by F08.
		if nospaceSuites[suite.cmdName] && strings.HasPrefix(line, "NOSPACE=1 ") {
			line = strings.TrimPrefix(line, "NOSPACE=1 ")
			nospace = true
		}

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
				val = strings.ReplaceAll(val, "${FILES}", fixtureDir)
				val = strings.ReplaceAll(val, "$FILES", fixtureDir)
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
		} else if echoPipeRedirectRe.MatchString(line) {
			// `echo ... | xxd -r -p > name` and friends generate small binary
			// fixtures. The redirect target is validated like any other setup
			// path, so replaying the pipeline stays inside the test directory.
			suite.setupLog = append(suite.setupLog, setupEvent{lineNum, "exec", line, false})
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

		// Only an expected-output argument may span lines (readelf.test writes
		// its golden blocks that way). A command argument continued on the next
		// line is left to the existing unparsed accounting.
		if multilineCaseSuites[suite.cmdName] &&
			(strings.HasPrefix(line, "testcmd ") || strings.HasPrefix(line, "testing ")) && expectedSpansLines(line) {
			caseBuf, caseBufSkip, caseBufNoSpace = line, skip, nospace
			continue
		}

		if strings.HasPrefix(line, "testcmd ") {
			if strings.Contains(line, "$1") || strings.Contains(line, "$ONE") || strings.Contains(line, "$TWO") {
				suite.unparsed = append(suite.unparsed, unparsedCase{lineNum, "parameterized helper cannot be represented by adapter"})
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
			tc, err := parseTestcmd(line[len("testcmd "):], lineNum, skip, nospace, suite.vars)
			if err != nil {
				suite.unparsed = append(suite.unparsed, unparsedCase{lineNum, err.Error()})
				continue
			}
			if suite.cmdName == "cp" && tc.name == "file->dir/file" {
				suite.setupLog = append(suite.setupLog, setupEvent{lineNum, "exec", "cp random two/random", false})
			}
			suite.cases = append(suite.cases, tc)
		} else if strings.HasPrefix(line, "testing ") {
			tc, err := parseTesting(line[len("testing "):], suite.cmdName, lineNum, skip, nospace, suite.vars)
			if err != nil {
				suite.unparsed = append(suite.unparsed, unparsedCase{lineNum, err.Error()})
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

func parseTestcmd(s string, lineNum int, skip, nospace bool, vars map[string]string) (testCase, error) {
	fields, err := extractQuotedFields(s)
	if err != nil || len(fields) < 3 {
		return testCase{}, fmt.Errorf("parse error line %d", lineNum)
	}

	for len(fields) < 5 {
		fields = append(fields, "")
	}

	infile := unescapeTestStr(expandVars(fields[3], vars))
	if content, ok := echoExprContent(infile); ok {
		infile = content
	}

	tc := testCase{
		name:     fields[0],
		args:     expandVars(fields[1], vars),
		expected: unescapeTestStr(expandVars(fields[2], vars)),
		infile:   infile,
		stdin:    unescapeTestStr(expandVars(fields[4], vars)),
		line:     lineNum,
		skip:     skip,
		nospace:  nospace,
	}
	return tc, nil
}

// echoExprContent expands the `$(echo -e '...')` form the upstream fixtures use
// for binary fixture content (file.test's ar header). Anything else is left to
// the caller unchanged.
var echoExprRe = regexp.MustCompile(`(?s)^\$\(echo( -e)? '(.*)'\)(\n?)$`)

func echoExprContent(s string) (string, bool) {
	m := echoExprRe.FindStringSubmatch(s)
	if m == nil {
		return "", false
	}
	return unescapeTestStr(m[2]) + "\n" + m[3], true
}

func parseTesting(s string, cmdName string, lineNum int, skip, nospace bool, vars map[string]string) (testCase, error) {
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
		nospace:  nospace,
	}
	return tc, nil
}

// quotesBalanced reports whether every quoted word in s is terminated.
func quotesBalanced(s string) bool {
	_, err := extractQuotedFields(s)
	return err == nil
}

// expectedSpansLines reports whether s holds a case name, a complete command
// argument and then an unterminated quoted expected-output argument, which is
// how readelf.test writes multi-line golden blocks. Continuation lines that
// break a command argument are not accepted, so their accounting is unchanged.
func expectedSpansLines(s string) bool {
	if quotesBalanced(s) {
		return false
	}
	i, complete := 0, 0
	for i < len(s) {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= len(s) {
			return false
		}
		if s[i] != '"' && s[i] != '\'' {
			// Bare words (the testing/testcmd helper name) carry no quoted field.
			for i < len(s) && s[i] != ' ' && s[i] != '\t' {
				i++
			}
			continue
		}
		end := findClosingQuote(s, i+1, s[i])
		if end < 0 {
			// Only a double-quoted expected argument spans lines upstream.
			return complete == 2 && s[i] == '"'
		}
		complete++
		i = end + 1
	}
	return false
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

func runTestCase(t *testing.T, cmdName string, tc testCase, setupLog []setupEvent, vars map[string]string, rec *testaccount.Case) {
	t.Helper()
	if reason, ok := knownFailures()[strings.TrimPrefix(t.Name(), "TestToyboxSuite/")]; ok && os.Getenv("MISA_KNOWN_FAILURES") == "" {
		rec.Reason = classifyKnown(strings.TrimPrefix(t.Name(), "TestToyboxSuite/"), reason)
		t.Skip(reason)
	}

	if tc.skip && !toyonlyRunSuites[cmdName] {
		skipRecorded(t, rec, testaccount.Deferred, "toyonly or host-restricted test")
	}
	if _, ok := builtins.Lookup(cmdName); !ok {
		skipRecorded(t, rec, testaccount.Deferred, fmt.Sprintf("builtin %q not registered", cmdName))
	}

	// Validate the whole case and every prior setup event before performing any
	// fixture writes or invoking a shell, including for the direct path.
	if err := validateSetup(setupLog, tc.line); err != nil {
		skipRecorded(t, rec, testaccount.UnsafeSetup, err.Error())
	}
	if err := validateScript(cmdName + " " + tc.args); err != nil {
		skipRecorded(t, rec, testaccount.UnsafeSetup, err.Error())
	}
	if strings.Contains(tc.expected, "$(") || strings.Contains(tc.expected, "`") {
		skipRecorded(t, rec, testaccount.UnsafeSetup, "expected output contains command substitution")
	}

	args := tc.args
	if strings.Contains(args, "&&") || strings.Contains(args, "||") ||
		strings.Contains(args, "|") || strings.Contains(args, ">") ||
		strings.Contains(args, "$") || strings.Contains(args, "`") ||
		strings.Contains(args, ";") {
		runTestCaseShell(t, cmdName, tc, setupLog, vars, rec)
		return
	}

	if sandboxUnsafeRe.MatchString(args) || sandboxUnsafeRe.MatchString(tc.stdin) {
		skipRecorded(t, rec, testaccount.UnsafeSetup, "sandbox-unsafe operation")
	}

	if strings.Contains(args, "/dev/full") || strings.Contains(args, "/dev/null") {
		skipRecorded(t, rec, testaccount.HostDependent, "device access needed")
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
				skipRecorded(t, rec, testaccount.HostDependent, fmt.Sprintf("references external absolute path %q", a))
			}
		}
	}

	cmd, ok := builtins.Lookup(cmdName)
	if !ok {
		skipRecorded(t, rec, testaccount.Deferred, fmt.Sprintf("builtin %q not registered", cmdName))
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
		if ev.name != "ln" && ev.name != "exec" {
			if err := validateSetupPath(dir, ev.name); err != nil {
				skipRecorded(t, rec, testaccount.UnsafeSetup, fmt.Sprintf("setup line %d: %v", ev.line, err))
			}
		}
		if ev.name == "ln" || ev.name == "exec" {
			c := exec.Command("/bin/bash", "-c", ev.content)
			c.Dir = dir
			c.Env = toyEnvironment(dir, nil)
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
		Env:     toyEnvironment(dir, vars),
		Width:   80,
		Height:  24,
		Stdout:  &stdout,
		Stderr:  &stderr,
		Stdin:   strings.NewReader(tc.stdin),
	}
	rec.Executed = true
	cmd.Run(ctx)

	got := stdout.String()
	if !sameOutput(got, tc.expected, tc.nospace) {
		t.Errorf("stdout mismatch:\n  got:  %q\n  want: %q\n  args: %q\n  stderr: %q",
			got, tc.expected, args, stderr.String())
	}
}

func runTestCaseShell(t *testing.T, cmdName string, tc testCase, setupLog []setupEvent, vars map[string]string, rec *testaccount.Case) {
	t.Helper()
	syscall.Umask(0022)
	if binDir == "" {
		skipRecorded(t, rec, testaccount.Deferred, "multicall binary not available")
		return
	}

	argsStr := tc.args
	if strings.HasPrefix(argsStr, cmdName+" ") {
		argsStr = strings.TrimPrefix(argsStr, cmdName+" ")
	}
	fullCmd := "enable -n " + cmdName + " 2>/dev/null; " + cmdName + " " + argsStr
	if strings.Contains(fullCmd, "/dev/full") {
		skipRecorded(t, rec, testaccount.UnsafeSetup, "sandbox-unsafe /dev/full test")
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
		if ev.name != "ln" && ev.name != "exec" {
			if err := validateSetupPath(dir, ev.name); err != nil {
				skipRecorded(t, rec, testaccount.UnsafeSetup, fmt.Sprintf("setup line %d: %v", ev.line, err))
			}
		}
		if ev.name == "ln" || ev.name == "exec" {
			c := exec.Command("/bin/bash", "-c", ev.content)
			c.Dir = dir
			c.Env = toyEnvironment(dir, nil)
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

	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", fullCmd)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(tc.stdin)
	cmd.Env = append(toyEnvironment(dir, vars), "C="+filepath.Join(binDir, cmdName))
	cmd.Stdin = strings.NewReader(tc.stdin)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	rec.Executed = true
	cmd.Run()

	got := stdout.String()
	expected := tc.expected

	if !sameOutput(got, expected, tc.nospace) {
		if cmdName == "strings" {
			t.Logf("STRINGS FAIL: fullCmd=%q got=%q expected=%q stderr=%q stdinLen=%d", fullCmd, got, expected, stderr.String(), len(tc.stdin))
		}
		t.Errorf("stdout mismatch:\n  got:  %q\n  want: %q\n  cmd:  %q\n  stderr: %q",
			got, tc.expected, fullCmd, stderr.String())
	}
}

// sameOutput compares a case's output. nospace cases mirror toybox's
// NOSPACE=1 directive, which runs diff -auw: whitespace is ignored entirely.
func sameOutput(got, want string, nospace bool) bool {
	if got == want {
		return true
	}
	if !nospace {
		return false
	}
	strip := func(s string) string {
		return strings.Map(func(r rune) rune {
			if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f' {
				return -1
			}
			return r
		}, s)
	}
	return strip(got) == strip(want)
}

func classifyKnown(name, detail string) *testaccount.Reason {
	r := &testaccount.Reason{Category: testaccount.KnownFailure, Detail: detail}
	if strings.HasPrefix(detail, "adapter:") {
		r.Category = testaccount.Deferred
		r.Task = "F07"
	}
	if strings.HasPrefix(name, "du/") {
		r.Category = testaccount.HostDependent
	}
	if strings.HasPrefix(name, "chmod/") || strings.HasPrefix(name, "mv/") || strings.HasPrefix(name, "cp/symlink") {
		r.Native = "tests/native/upstream_constructs_test.go:TestAdapterNativeConstructs"
	}
	return r
}

func skipRecorded(t *testing.T, rec *testaccount.Case, category, detail string) {
	t.Helper()
	rec.Reason = &testaccount.Reason{Category: category, Detail: detail}
	if category == testaccount.Deferred {
		rec.Reason.Task = "F07"
	}
	t.Skip(detail)
}

func TestToyboxSuite(t *testing.T) {
	collector := testaccount.New("toybox")
	defer func() {
		if err := collector.WriteEnv(); err != nil {
			t.Error(err)
		}
	}()
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
			t.Errorf("failed to parse %s: %v", e.Name(), err)
			continue
		}

		for _, u := range suite.unparsed {
			reason := &testaccount.Reason{Category: testaccount.Unparseable, Detail: u.reason}
			if suite.cmdName == "chmod" || suite.cmdName == "cp" || suite.cmdName == "ln" {
				reason.Native = "tests/native/upstream_constructs_test.go:TestAdapterNativeConstructs"
			}
			if err := collector.Add(testaccount.Case{Utility: suite.cmdName, ID: fmt.Sprintf("line%d", u.line), File: path, Line: u.line, Status: "skipped", Reason: reason}); err != nil {
				t.Error(err)
			}
		}
		if len(suite.cases) == 0 {
			continue
		}

		totalParsed += len(suite.cases)

		t.Run(suite.cmdName, func(t *testing.T) {
			for caseIndex, tc := range suite.cases {
				tc := tc
				name := tc.name
				if name == "" {
					name = fmt.Sprintf("line%d", tc.line)
				}
				t.Run(name, func(t *testing.T) {
					totalRun++
					rec := testaccount.Case{Utility: suite.cmdName, ID: fmt.Sprintf("%s@%d:%d", name, tc.line, caseIndex), File: path, Line: tc.line, Parsed: true}
					defer func() {
						switch {
						case t.Failed():
							rec.Status = "failed"
							rec.Reason = nil
						case t.Skipped():
							rec.Status = "skipped"
							if rec.Reason == nil {
								rec.Reason = &testaccount.Reason{Category: testaccount.Deferred, Detail: "test skipped", Task: "F07"}
							}
						default:
							rec.Status = "passed"
						}
						if err := collector.Add(rec); err != nil {
							t.Error(err)
						}
					}()
					runTestCase(t, suite.cmdName, tc, suite.setupLog, suite.vars, &rec)
				})
			}
		})
	}

	t.Logf("Toybox tests: %d parsed from test files", totalParsed)
}
