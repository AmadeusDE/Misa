package testaccount

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidationAndAggregation(t *testing.T) {
	c := New("toybox")
	good := Case{Utility: "cat", ID: "one", File: "cat.test", Line: 1, Parsed: true, Executed: true, Status: "passed"}
	if err := c.Add(good); err != nil {
		t.Fatal(err)
	}
	skip := Case{Utility: "cat", ID: "two", Parsed: false, Status: "skipped", Reason: &Reason{Category: Unparseable, Detail: "syntax"}}
	if err := c.Add(skip); err != nil {
		t.Fatal(err)
	}
	bad := []Case{{Utility: "cat", ID: "x", Executed: true, Status: "passed"}, {Utility: "cat", ID: "x", Parsed: true, Status: "passed"}, {Utility: "cat", ID: "x", Status: "skipped"}, {Utility: "cat", ID: "x", Status: "skipped", Reason: &Reason{Category: "unknown", Detail: "x"}}, {Suite: "other", Utility: "cat", ID: "x", Status: "skipped", Reason: &Reason{Category: Deferred, Detail: "x"}}}
	for _, b := range bad {
		if err := c.Add(b); err == nil {
			t.Errorf("accepted %+v", b)
		}
	}
	r, err := Aggregate(c.Cases())
	if err != nil {
		t.Fatal(err)
	}
	if r.Total.Discovered != 2 || r.Total.Parsed != 1 || r.Total.Executed != 1 || r.Total.Passed != 1 || r.Total.Skipped != 1 {
		t.Fatalf("%+v", r.Total)
	}
	if r.Suites[0].Utilities[0].Counts.Skipped != 1 {
		t.Fatal(r)
	}
	var b strings.Builder
	if err := WriteMarkdown(&b, r); err != nil || !strings.Contains(b.String(), "unparseable") {
		t.Fatalf("%s %v", b.String(), err)
	}
}
func TestWriteEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MISA_TEST_REPORT", dir)
	c := New("toybox")
	if err := c.Add(Case{Utility: "cat", ID: "a", Parsed: true, Executed: true, Status: "passed"}); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteEnv(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "toybox.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []Case
	if err := json.Unmarshal(data, &cases); err != nil || len(cases) != 1 || cases[0].Suite != "toybox" {
		t.Fatalf("%s %v", data, err)
	}
}

func TestRejectDuplicateIdentity(t *testing.T) {
	c := Case{Suite: "toybox", Utility: "cat", ID: "same", Parsed: true, Executed: true, Status: "passed"}
	other := c
	other.File = "another.test"
	other.Line = 99
	if _, err := Aggregate([]Case{c, other}); err == nil {
		t.Fatal("duplicate identity accepted")
	}
	other.Utility = "echo"
	if _, err := Aggregate([]Case{c, other}); err != nil {
		t.Fatal(err)
	}
}

func TestDeferredRequiresTask(t *testing.T) {
	c := Case{Suite: "toybox", Utility: "cat", ID: "x", Status: "skipped", Reason: &Reason{Category: Deferred, Detail: "not implemented"}}
	if err := c.Validate(); err == nil {
		t.Fatal("deferred skip without task accepted")
	}
	c.Reason.Task = "F07"
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestStatusCounters(t *testing.T) {
	cases := []Case{
		{Suite: "s", Utility: "u", ID: "pass", Parsed: true, Executed: true, Status: "passed"},
		{Suite: "s", Utility: "u", ID: "fail", Parsed: true, Executed: true, Status: "failed"},
		{Suite: "s", Utility: "u", ID: "skip", Parsed: true, Status: "skipped", Reason: &Reason{Category: HostDependent, Detail: "requires host"}},
		{Suite: "s", Utility: "u", ID: "unparsed", Status: "skipped", Reason: &Reason{Category: Unparseable, Detail: "unknown syntax"}},
	}
	r, err := Aggregate(cases)
	if err != nil {
		t.Fatal(err)
	}
	want := Counts{Discovered: 4, Parsed: 3, Executed: 2, Passed: 1, Failed: 1, Skipped: 2}
	if r.Total != want || r.Suites[0].Counts != want || r.Suites[0].Utilities[0].Counts != want {
		t.Fatalf("counts: %+v", r)
	}
}

func TestSetupFailureAndFailureDetail(t *testing.T) {
	c := Case{Suite: "uutils", Utility: "cat", ID: "setup", Parsed: true, Status: "failed", Failure: "fixture setup failed"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	r, err := Aggregate([]Case{c})
	if err != nil {
		t.Fatal(err)
	}
	if r.Total.Failed != 1 || r.Total.Executed != 0 || r.Total.Parsed != 1 {
		t.Fatal(r.Total)
	}
	c.Parsed = false
	if err := c.Validate(); err == nil {
		t.Fatal("unparsed failure accepted")
	}
	c.Parsed = true
	c.Executed = true
	c.Status = "passed"
	if err := c.Validate(); err == nil {
		t.Fatal("failure detail on pass accepted")
	}
	c.Status = "skipped"
	c.Reason = &Reason{Category: HostDependent, Detail: "requires host"}
	if err := c.Validate(); err == nil {
		t.Fatal("failure detail on skip accepted")
	}
}

func TestFailureDetailRendered(t *testing.T) {
	r, err := Aggregate([]Case{{Suite: "uutils", Utility: "cat", ID: "setup", Parsed: true, Status: "failed", Failure: "setup failed"}})
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := WriteMarkdown(&b, r); err != nil || !strings.Contains(b.String(), "setup failed") {
		t.Fatalf("%s %v", b.String(), err)
	}
}
