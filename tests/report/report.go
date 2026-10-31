// Command report runs the upstream adapters and writes structured coverage reports.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AmadeusDE/Misa/tests/testaccount"
)

type runner func(pkg, reportDir string) error

func runSuite(pkg, dir string) error {
	cmd := exec.Command("go", "test", pkg, "-count=1", "-timeout=180s")
	cmd.Env = append(os.Environ(), "MISA_TEST_REPORT="+dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
func generate(out string, run runner) error {
	dir, err := os.MkdirTemp("", "misa-test-report-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	var problems []error
	for _, pkg := range []string{"./tests/toybox/", "./tests/uutils/"} {
		if err := run(pkg, dir); err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", pkg, err))
		}
	}
	var cases []testaccount.Case
	seen := make(map[[3]string]bool)
	for _, name := range []string{"toybox", "uutils"} {
		data, err := os.ReadFile(filepath.Join(dir, name+".json"))
		if err != nil {
			problems = append(problems, fmt.Errorf("%s records: %w", name, err))
			continue
		}
		var records []testaccount.Case
		if err := json.Unmarshal(data, &records); err != nil {
			problems = append(problems, fmt.Errorf("%s records: %w", name, err))
			continue
		}
		for _, c := range records {
			if c.Suite != name {
				problems = append(problems, fmt.Errorf("%s records contain suite %q", name, c.Suite))
				continue
			}
			if err := c.Validate(); err != nil {
				problems = append(problems, err)
				continue
			}
			key := [3]string{c.Suite, c.Utility, c.ID}
			if seen[key] {
				problems = append(problems, fmt.Errorf("duplicate case identity %q/%q/%q", c.Suite, c.Utility, c.ID))
				continue
			}
			seen[key] = true
			cases = append(cases, c)
		}
	}
	report, err := testaccount.Aggregate(cases)
	if err != nil {
		return errors.Join(append(problems, err)...)
	}
	if err = os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return errors.Join(append(problems, err)...)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err == nil {
		err = os.WriteFile(out+".json", append(data, '\n'), 0644)
	}
	if err != nil {
		problems = append(problems, err)
	}
	f, err := os.Create(out + ".md")
	if err == nil {
		err = testaccount.WriteMarkdown(f, report)
		err = errors.Join(err, f.Close())
	}
	if err != nil {
		problems = append(problems, err)
	}
	return errors.Join(problems...)
}
func main() {
	out := flag.String("out", "docs/plans/upstream-test-report", "output filename prefix")
	flag.Parse()
	if err := generate(*out, runSuite); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
