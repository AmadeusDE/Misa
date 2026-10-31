package toybox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoveryReportsUnparseable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cat.test")
	if err := os.WriteFile(path, []byte("testcmd \"broken\"\ntesting \"also broken\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	suite, err := parseTestFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.unparsed) != 2 {
		t.Fatalf("got %d unparsed cases, want 2", len(suite.unparsed))
	}
	for _, c := range suite.unparsed {
		if c.line == 0 || strings.TrimSpace(c.reason) == "" {
			t.Fatalf("missing source or reason: %+v", c)
		}
	}
}

func TestKnownFailureEntriesHaveReasonsAndCases(t *testing.T) {
	found := map[string]bool{}
	for _, line := range strings.Split(knownFailuresText, "\n") {
		name, reason, _ := strings.Cut(line, "#")
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("%s: missing reason", name)
		}
		if found[name] {
			t.Errorf("%s: duplicate entry", name)
		}
		found[name] = false
	}
	entries, err := filepath.Glob("*.test")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range entries {
		suite, err := parseTestFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range suite.cases {
			key := suite.cmdName + "/" + strings.ReplaceAll(c.name, " ", "_")
			if _, ok := found[key]; ok {
				found[key] = true
			}
		}
	}
	for name, ok := range found {
		if !ok {
			t.Errorf("stale known failure: %s", name)
		}
	}
}

func TestKnownReasonClassification(t *testing.T) {
	for _, tc := range []struct{ name, reason, category string }{
		{"chmod/u-x_dir_file", "adapter: loop isn't parsed", "deferred"},
		{"du/-s", "du: block counts depend on the filesystem", "host_dependent"},
		{"env/why_is_this_allowed", "env: empty variable name is rejected", "known_failure"},
	} {
		got := classifyKnown(tc.name, tc.reason)
		if got.Category != tc.category {
			t.Errorf("%s: got %s want %s", tc.name, got.Category, tc.category)
		}
	}
}
