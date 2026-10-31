package toybox

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

type findCase struct{ name, command, expected, infile, stdin, setup string }

// parseFindCases handles the deliberately limited, pinned find.test syntax.
// Unknown setup or malformed cases are errors, never silently omitted.
func parseFindCases(r io.Reader) ([]findCase, error) {
	sc := bufio.NewScanner(r)
	var setup strings.Builder
	var cases []findCase
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		for strings.HasSuffix(line, "\\") {
			line = strings.TrimSuffix(line, "\\")
			if !sc.Scan() {
				return nil, fmt.Errorf("line %d: incomplete continuation", lineNo)
			}
			lineNo++
			line += " " + strings.TrimSpace(sc.Text())
		}
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[ -f testing.sh ]") || line == "utf8locale" || strings.HasPrefix(line, "skipnot ") {
			continue
		}
		if strings.HasPrefix(line, "testing ") {
			fields, err := extractQuotedFields(strings.TrimPrefix(line, "testing "))
			if err != nil || len(fields) != 5 {
				return nil, fmt.Errorf("line %d: invalid testing: %q", lineNo, line)
			}
			cases = append(cases, findCase{fields[0], fields[1], unescapeTestStr(fields[2]), unescapeTestStr(fields[3]), unescapeTestStr(fields[4]), setup.String()})
			continue
		}
		allowed := false
		for _, prefix := range []string{"mkdir ", "cd ", "touch ", "mkfifo ", "sleep ", "ln ", "chmod ", "rm "} {
			if strings.HasPrefix(line, prefix) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("line %d: unsupported setup %q", lineNo, line)
		}
		setup.WriteString(line + "\n")
	}
	return cases, sc.Err()
}

func TestToyboxFind(t *testing.T) {
	f, err := os.Open("find.test")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cases, err := parseFindCases(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 58 {
		t.Fatalf("find.test: parsed %d cases, expected 58", len(cases))
	}
	if _, ok := builtins.Lookup("find"); !ok {
		t.Fatal("find builtin not registered; refusing host find fallback")
	}
	if _, err := os.Lstat(filepath.Join(binDir, "find")); err != nil {
		t.Fatalf("find multicall link unavailable: %v", err)
	}
	if binDir == "" {
		t.Fatal("multicall build unavailable")
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%02d", i+1), func(t *testing.T) {
			t.Logf("upstream case: %s", tc.name)
			if reason, ok := knownFailures()[fmt.Sprintf("find/%02d", i+1)]; ok && os.Getenv("MISA_KNOWN_FAILURES") == "" {
				t.Skip("known failure: " + reason)
			}
			dir := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			// Setup uses host tools, separately and checked. The tested find must resolve
			// to the multicall executable, including in pipelines and compound commands.
			setup := exec.CommandContext(ctx, "bash", "-ec", tc.setup)
			setup.Dir = dir
			if out, err := setup.CombinedOutput(); err != nil {
				t.Fatalf("setup: %v: %s", err, out)
			}
			if tc.infile != "" {
				if err := os.WriteFile(filepath.Join(dir, "input"), []byte(tc.infile), 0644); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.CommandContext(ctx, "bash", "-c", tc.command)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"), "LC_ALL=C")
			cmd.Stdin = strings.NewReader(tc.stdin)
			var out, stderr bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &stderr
			err := cmd.Run()
			if ctx.Err() != nil {
				t.Fatal(ctx.Err())
			}
			// Toybox's testing helper compares stdout, not the command's exit status.
			if out.String() != tc.expected {
				t.Errorf("stdout: got %q, want %q; command %q; status %v; stderr %q", out.String(), tc.expected, tc.command, err, stderr.String())
			}
		})
	}
}
