package testaccount

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

type Counts struct {
	Discovered int `json:"discovered"`
	Parsed     int `json:"parsed"`
	Executed   int `json:"executed"`
	Passed     int `json:"passed"`
	Failed     int `json:"failed"`
	Skipped    int `json:"skipped"`
}
type Utility struct {
	Name   string `json:"name"`
	Counts Counts `json:"counts"`
}
type Suite struct {
	Name      string    `json:"name"`
	Counts    Counts    `json:"counts"`
	Utilities []Utility `json:"utilities"`
}
type Report struct {
	Total  Counts  `json:"total"`
	Suites []Suite `json:"suites"`
	Cases  []Case  `json:"cases"`
}

func (n *Counts) add(c Case) {
	n.Discovered++
	if c.Parsed {
		n.Parsed++
	}
	if c.Executed {
		n.Executed++
	}
	switch c.Status {
	case "passed":
		n.Passed++
	case "failed":
		n.Failed++
	case "skipped":
		n.Skipped++
	}
}
func Aggregate(cases []Case) (Report, error) {
	r := Report{Cases: append([]Case{}, cases...), Suites: []Suite{}}
	sort.Slice(r.Cases, func(i, j int) bool {
		a, b := r.Cases[i], r.Cases[j]
		if a.Suite != b.Suite {
			return a.Suite < b.Suite
		}
		if a.Utility != b.Utility {
			return a.Utility < b.Utility
		}
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.ID < b.ID
	})
	seen := make(map[[3]string]bool)
	for _, c := range r.Cases {
		if err := c.Validate(); err != nil {
			return Report{}, err
		}
		key := [3]string{c.Suite, c.Utility, c.ID}
		if seen[key] {
			return Report{}, fmt.Errorf("duplicate case identity %q/%q/%q", c.Suite, c.Utility, c.ID)
		}
		seen[key] = true
		r.Total.add(c)
		if len(r.Suites) == 0 || r.Suites[len(r.Suites)-1].Name != c.Suite {
			r.Suites = append(r.Suites, Suite{Name: c.Suite, Utilities: []Utility{}})
		}
		s := &r.Suites[len(r.Suites)-1]
		s.Counts.add(c)
		if len(s.Utilities) == 0 || s.Utilities[len(s.Utilities)-1].Name != c.Utility {
			s.Utilities = append(s.Utilities, Utility{Name: c.Utility})
		}
		s.Utilities[len(s.Utilities)-1].Counts.add(c)
	}
	return r, nil
}
func WriteMarkdown(w io.Writer, r Report) error {
	var b strings.Builder
	fmt.Fprint(&b, "# Upstream adapter coverage\n\nNative, unit and diffutils tests are outside this report's scope. Counts describe adapter records, not Go test results.\n\n| Suite | Utility | Discovered | Parsed | Executed | Passed | Failed | Skipped |\n|---|---|---:|---:|---:|---:|---:|---:|\n")
	row := func(s, u string, c Counts) {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %d | %d | %d |\n", escape(s), escape(u), c.Discovered, c.Parsed, c.Executed, c.Passed, c.Failed, c.Skipped)
	}
	row("Total", "", r.Total)
	for _, s := range r.Suites {
		row(s.Name, "Total", s.Counts)
		for _, u := range s.Utilities {
			row(s.Name, u.Name, u.Counts)
		}
	}
	fmt.Fprint(&b, "\n## Skipped cases\n\n| Suite | Utility | Case | Category | Detail | Task | Native |\n|---|---|---|---|---|---|---|\n")
	for _, c := range r.Cases {
		if c.Status == "skipped" {
			v := c.Reason
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s |\n", escape(c.Suite), escape(c.Utility), escape(c.ID), escape(v.Category), escape(v.Detail), escape(v.Task), escape(v.Native))
		}
	}
	fmt.Fprint(&b, "\n## Failed cases\n\n| Suite | Utility | Case | Executed | Detail |\n|---|---|---|---|---|\n")
	for _, c := range r.Cases {
		if c.Status == "failed" {
			fmt.Fprintf(&b, "| %s | %s | %s | %t | %s |\n", escape(c.Suite), escape(c.Utility), escape(c.ID), c.Executed, escape(c.Failure))
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}
func escape(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "|", "\\|"), "\n", " ") }
