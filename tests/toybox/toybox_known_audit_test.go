package toybox

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// parseKnownProbe reads only leaf results; parent results cannot establish that
// a particular exclusion ran (a parent can pass even when its child skipped).
func parseKnownProbe(output, suite string) map[string]string {
	results := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		for _, status := range []string{"PASS", "FAIL", "SKIP"} {
			prefix := "--- " + status + ": " + suite + "/"
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			name, _, ok := strings.Cut(strings.TrimPrefix(line, prefix), " (")
			if ok && strings.Count(name, "/") == 1 {
				results[name] = status
			}
		}
	}
	return results
}

func TestKnownProbeParser(t *testing.T) {
	got := parseKnownProbe("    --- PASS: TestExample/cmd/a (0.00s)\n    --- SKIP: TestExample/cmd/b (0.00s)\n    --- FAIL: TestExample/cmd/c (0.00s)\n--- FAIL: TestExample/cmd (0.00s)\n", "TestExample")
	for name, want := range map[string]string{"cmd/a": "PASS", "cmd/b": "SKIP", "cmd/c": "FAIL"} {
		if got[name] != want {
			t.Errorf("%s: got %q want %q", name, got[name], want)
		}
	}
	if len(got) != 3 {
		t.Fatalf("unexpected results: %v", got)
	}
}

func TestKnownFailureAudit(t *testing.T) {
	// Read the raw list: a map would hide duplicate and reasonless entries.
	seen := map[string]bool{}
	groups := map[string][]string{}
	for _, line := range strings.Split(knownFailuresText, "\n") {
		name, reason, _ := strings.Cut(line, "#")
		name, reason = strings.TrimSpace(name), strings.TrimSpace(reason)
		if name == "" {
			continue
		}
		if seen[name] {
			t.Errorf("duplicate known failure: %s", name)
			continue
		}
		seen[name] = true
		if reason == "" {
			t.Errorf("%s: missing reason", name)
			continue
		}
		cmd, test, ok := strings.Cut(name, "/")
		if !ok || cmd == "" || test == "" || strings.Contains(test, "/") {
			t.Errorf("invalid known failure %q", name)
			continue
		}
		if knownProbeHostOrDeferred(name, reason) {
			t.Logf("not probed %s: %s", name, reason)
			continue
		}
		groups[cmd] = append(groups[cmd], test)
	}
	commands := make([]string, 0, len(groups))
	for cmd := range groups {
		commands = append(commands, cmd)
	}
	sort.Strings(commands)
	for _, cmd := range commands {
		cases := groups[cmd]
		sort.Strings(cases)
		escaped := make([]string, len(cases))
		for i, name := range cases {
			escaped[i] = regexp.QuoteMeta(name)
		}
		// One command per process avoids a cross-product of command and case names,
		// which could otherwise run cases outside the exclusion list.
		pattern := "^TestToyboxSuite$/^" + regexp.QuoteMeta(cmd) + "$/^(" + strings.Join(escaped, "|") + ")$"
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		child := exec.CommandContext(ctx, os.Args[0], "-test.v", "-test.run", pattern)
		for _, env := range os.Environ() {
			if !strings.HasPrefix(env, "MISA_KNOWN_FAILURES=") && !strings.HasPrefix(env, "MISA_TEST_REPORT=") {
				child.Env = append(child.Env, env)
			}
		}
		child.Env = append(child.Env, "MISA_KNOWN_FAILURES=1")
		output, err := child.CombinedOutput()
		ctxErr := ctx.Err()
		cancel()
		if ctxErr != nil {
			t.Errorf("%s probe: %v", cmd, ctxErr)
			continue
		}
		if err != nil {
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
				t.Errorf("%s probe failed: %v\n%s", cmd, err, output)
				continue
			}
		}
		results := parseKnownProbe(string(output), "TestToyboxSuite")
		if strings.Contains(string(output), "panic:") {
			t.Errorf("%s probe panicked:\n%s", cmd, output)
			continue
		}
		for _, name := range cases {
			key := cmd + "/" + name
			switch results[key] {
			case "PASS":
				t.Errorf("stale known failure now passes: %s (%s)", key, knownFailures()[key])
			case "FAIL": // Expected failure.
			case "SKIP":
				t.Logf("not checked (adapter skipped) %s: %s\n%s", key, knownFailures()[key], output)
			default:
				t.Errorf("known failure not observed in probe: %s\n%s", key, output)
			}
		}
	}
}

func knownProbeHostOrDeferred(name, reason string) bool {
	category := classifyKnown(name, reason).Category
	return category == "host_dependent" || category == "deferred"
}
