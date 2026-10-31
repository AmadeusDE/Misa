package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/AmadeusDE/Misa/tests/testaccount"
)

func TestRunSavesPartialAndReturnsFailure(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "report")
	run := func(pkg, reportDir string) error {
		if pkg == "./tests/toybox/" {
			c := testaccount.New("toybox")
			if err := c.Add(testaccount.Case{Utility: "cat", ID: "x", Parsed: true, Executed: true, Status: "failed"}); err != nil {
				t.Fatal(err)
			}
			t.Setenv("MISA_TEST_REPORT", reportDir)
			if err := c.WriteEnv(); err != nil {
				t.Fatal(err)
			}
			return errors.New("failed")
		}
		return nil
	}
	if err := generate(out, run); err == nil {
		t.Fatal("missing failure")
	}
	data, err := os.ReadFile(out + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var r testaccount.Report
	if err := json.Unmarshal(data, &r); err != nil || r.Total.Failed != 1 {
		t.Fatalf("%s %v", data, err)
	}
	if _, err := os.Stat(out + ".md"); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateBothSuites(t *testing.T) {
	out := filepath.Join(t.TempDir(), "nested", "coverage")
	calls := 0
	run := func(pkg, dir string) error {
		calls++
		suite := "toybox"
		if pkg == "./tests/uutils/" {
			suite = "uutils"
		}
		c := testaccount.New(suite)
		if err := c.Add(testaccount.Case{Utility: "echo", ID: "case", Status: "skipped", Reason: &testaccount.Reason{Category: testaccount.Deferred, Detail: "not supported", Task: "F07"}}); err != nil {
			t.Fatal(err)
		}
		t.Setenv("MISA_TEST_REPORT", dir)
		return c.WriteEnv()
	}
	if err := generate(out, run); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var r testaccount.Report
	if err = json.Unmarshal(data, &r); err != nil || calls != 2 || len(r.Suites) != 2 || r.Total.Discovered != 2 || r.Total.Skipped != 2 {
		t.Fatalf("calls=%d report=%+v err=%v", calls, r, err)
	}
}

func TestGenerateRetainsValidRecordsWithDuplicates(t *testing.T) {
	out := filepath.Join(t.TempDir(), "report")
	run := func(pkg, dir string) error {
		suite := "toybox"
		if pkg == "./tests/uutils/" {
			suite = "uutils"
		}
		c := testaccount.New(suite)
		v := testaccount.Case{Utility: "cat", ID: "one", Parsed: true, Executed: true, Status: "passed"}
		if err := c.Add(v); err != nil {
			t.Fatal(err)
		}
		if suite == "toybox" {
			if err := c.Add(v); err != nil {
				t.Fatal(err)
			}
		}
		t.Setenv("MISA_TEST_REPORT", dir)
		return c.WriteEnv()
	}
	if err := generate(out, run); err == nil {
		t.Fatal("expected duplicate error")
	}
	data, err := os.ReadFile(out + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var r testaccount.Report
	if err := json.Unmarshal(data, &r); err != nil || r.Total.Discovered != 2 {
		t.Fatalf("%s %v", data, err)
	}
}
