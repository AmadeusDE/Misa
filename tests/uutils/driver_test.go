package uutils

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/AmadeusDE/Misa/tests/testaccount"
)

// ---------------------------------------------------------------------
// Top-level driver
// ---------------------------------------------------------------------

type runRecord struct {
	executed bool
	reason   *testaccount.Reason
}

func TestUutilsSuite(t *testing.T) {
	account := testaccount.New("uutils")
	defer func() {
		if err := account.WriteEnv(); err != nil {
			t.Error(err)
		}
	}()
	root := "testdata/tests"
	if _, err := os.Stat(root); err != nil {
		root = "tests/uutils/testdata/tests"
		if _, err := os.Stat(root); err != nil {
			t.Fatalf("uutils testdata not found: %v", err)
		}
	}
	byUtilDir := filepath.Join(root, "by-util")
	fixturesRoot := filepath.Join(root, "fixtures")
	if !fixtureRootSafe(fixturesRoot) {
		t.Fatalf("uutils fixtures root missing or unsafe: %s", fixturesRoot)
	}

	entries, err := os.ReadDir(byUtilDir)
	if err != nil {
		t.Fatalf("by-util dir not found: %v", err)
	}

	var files []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "test_") && strings.HasSuffix(e.Name(), ".rs") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	var totalParsed, totalCommands int

	for _, fname := range files {
		cmdName := strings.TrimSuffix(strings.TrimPrefix(fname, "test_"), ".rs")
		path := filepath.Join(byUtilDir, fname)
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Error(readErr)
			continue
		}
		_, available := builtins.Lookup(cmdName)
		var sut suite
		if available {
			sut, err = parseTestFile(path, cmdName)
			if err != nil {
				t.Error(err)
				continue
			}
		}
		for _, gap := range functionGaps(stripComments(string(raw)), sut.cases) {
			reason := &testaccount.Reason{Category: testaccount.Unparseable, Detail: "no assertion chain in this Rust function is represented by the adapter", Task: "F07", Native: nativeForGap(cmdName, gap)}
			if !available {
				reason = &testaccount.Reason{Category: testaccount.Deferred, Detail: "utility is not registered", Task: "F07"}
			}
			rec := testaccount.Case{Utility: cmdName, ID: fmt.Sprintf("function:%s_L%d", gap.name, gap.line), File: path, Line: gap.line, Status: "skipped", Reason: reason}
			if err := account.Add(rec); err != nil {
				t.Error(err)
			}
		}
		if !available {
			continue
		}
		if len(sut.cases) == 0 {
			continue
		}

		totalParsed += len(sut.cases)
		totalCommands++
		fixtureDir := filepath.Join(fixturesRoot, cmdName)

		t.Run(cmdName, func(t *testing.T) {
			for _, tc := range sut.cases {
				tc := tc
				name := tc.name
				if name == "" {
					name = fmt.Sprintf("line%d", tc.line)
				}
				t.Run(fmt.Sprintf("%s_L%d", name, tc.line), func(t *testing.T) {
					rec := &runRecord{}
					defer func() {
						status, reason, failure := resultStatus(t.Failed(), t.Skipped(), rec)
						c := testaccount.Case{Utility: cmdName, ID: fmt.Sprintf("%s_L%d", name, tc.line), File: path, Line: tc.line, Parsed: true, Executed: rec.executed, Status: status, Reason: reason, Failure: failure}
						if err := account.Add(c); err != nil {
							t.Error(err)
						}
					}()
					runTestCase(t, cmdName, tc, fixtureDir, rec)
				})
			}
		})
	}

	t.Logf("uutils tests: %d parsed across %d commands", totalParsed, totalCommands)
}

// A test can fail during fixture creation before the builtin was invoked.
func resultStatus(failed, skipped bool, rec *runRecord) (string, *testaccount.Reason, string) {
	if failed {
		if !rec.executed {
			return "failed", nil, "fixture/setup failed before command invocation"
		}
		return "failed", nil, ""
	}
	if skipped {
		if rec.reason != nil {
			return "skipped", rec.reason, ""
		}
		return "skipped", &testaccount.Reason{Category: testaccount.HostDependent, Detail: "adapter setup or environment unavailable"}, ""
	}
	return "passed", nil, ""
}
