// Package testaccount records individual upstream adapter cases.
package testaccount

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	Unparseable   = "unparseable"
	KnownFailure  = "known_failure"
	Deferred      = "deferred"
	HostDependent = "host_dependent"
	UnsafeSetup   = "unsafe_setup"
)

type Reason struct {
	Category string `json:"category"`
	Detail   string `json:"detail"`
	Task     string `json:"task,omitempty"`
	Native   string `json:"native,omitempty"`
}

// Case records discovery and outcome. A parsed case may fail before execution
// (for example, during fixture setup); successful cases must execute. Failure
// contains optional diagnostic detail and is valid only for failed cases.
type Case struct {
	Suite    string  `json:"suite"`
	Utility  string  `json:"utility"`
	ID       string  `json:"id"`
	File     string  `json:"file,omitempty"`
	Line     int     `json:"line,omitempty"`
	Parsed   bool    `json:"parsed"`
	Executed bool    `json:"executed"`
	Status   string  `json:"status"`
	Failure  string  `json:"failure,omitempty"`
	Reason   *Reason `json:"reason,omitempty"`
}

func (c Case) Validate() error {
	if c.Suite == "" || c.Utility == "" || c.ID == "" || c.Line < 0 {
		return fmt.Errorf("case requires suite, utility and id, and a nonnegative line: %+v", c)
	}
	if c.Executed && !c.Parsed {
		return fmt.Errorf("executed case %s was not parsed", c.ID)
	}
	if c.Failure != "" && c.Status != "failed" {
		return fmt.Errorf("failure detail on non-failed case %s", c.ID)
	}
	switch c.Status {
	case "passed":
		if !c.Executed || c.Reason != nil {
			return fmt.Errorf("passed case %s must be executed and have no skip reason", c.ID)
		}
	case "failed":
		if !c.Parsed || c.Reason != nil {
			return fmt.Errorf("failed case %s must be parsed and have no skip reason", c.ID)
		}
	case "skipped":
		if c.Reason == nil || strings.TrimSpace(c.Reason.Detail) == "" {
			return fmt.Errorf("skipped case %s requires a reason", c.ID)
		}
		switch c.Reason.Category {
		case Unparseable, KnownFailure, Deferred, HostDependent, UnsafeSetup:
		default:
			return fmt.Errorf("unknown skip category %q", c.Reason.Category)
		}
		if c.Reason.Category == Deferred && strings.TrimSpace(c.Reason.Task) == "" {
			return fmt.Errorf("deferred case %s requires a task", c.ID)
		}
	default:
		return fmt.Errorf("invalid status %q", c.Status)
	}
	return nil
}

type Collector struct {
	suite string
	mu    sync.Mutex
	cases []Case
}

func New(suite string) *Collector { return &Collector{suite: suite} }
func (c *Collector) Add(v Case) error {
	if v.Suite == "" {
		v.Suite = c.suite
	}
	if v.Suite != c.suite {
		return fmt.Errorf("case suite %q differs from collector %q", v.Suite, c.suite)
	}
	if err := v.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cases = append(c.cases, v)
	return nil
}
func (c *Collector) Cases() []Case {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Case{}, c.cases...)
}

// WriteEnv atomically writes this suite's records; it does nothing unless reporting is enabled.
func (c *Collector) WriteEnv() error {
	dir := os.Getenv("MISA_TEST_REPORT")
	if dir == "" {
		return nil
	}
	if c.suite == "" || c.suite == "." || c.suite == ".." || strings.ContainsAny(c.suite, `/\`) {
		return fmt.Errorf("invalid suite name %q", c.suite)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c.Cases(), "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".report-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, c.suite+".json"))
}
