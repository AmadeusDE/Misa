// Package vm runs code tasks' throwaway machines: a base image prepared once from a cloud
// image with cloud-init, then one copy-on-write overlay per task under qemu, reached over
// SSH with keys only the daemon holds and sharing the task's directories at the same paths.
package vm

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/config"
)

type Manager struct {
	cfg  config.VM
	dir  string
	log  io.Writer
	slot chan struct{}

	mu      sync.Mutex
	running map[string]string // run id → what it shares
}

func New(c *config.Config, log io.Writer) *Manager {
	if log == nil {
		log = io.Discard
	}
	return &Manager{cfg: c.VM, dir: c.VMDir(), log: log, slot: make(chan struct{}, c.VM.MaxRunning), running: map[string]string{}}
}

func (m *Manager) basePath() string { return filepath.Join(m.dir, "base.qcow2") }
func (m *Manager) runDir() string   { return filepath.Join(m.dir, "run") }

// Base describes the prepared base image.
type Base struct {
	Image    string    `json:"image"`
	Packages []string  `json:"packages"`
	Setup    []string  `json:"setup"`
	At       time.Time `json:"prepared_at"`
}

type Status struct {
	Prepared bool
	Base     Base
	Running  []string
	Max      int
}

func (m *Manager) Status() Status {
	s := Status{Max: m.cfg.MaxRunning}
	if _, err := os.Stat(m.basePath()); err == nil {
		s.Prepared = true
		if raw, err := os.ReadFile(filepath.Join(m.dir, "base.json")); err == nil {
			_ = json.Unmarshal(raw, &s.Base)
		}
	}
	m.mu.Lock()
	for id, what := range m.running {
		s.Running = append(s.Running, id+": "+what)
	}
	m.mu.Unlock()
	return s
}

// Cleanup kills machines a previous daemon left running and removes their overlays.
func (m *Manager) Cleanup() {
	entries, _ := os.ReadDir(m.runDir())
	for _, e := range entries {
		dir := filepath.Join(m.runDir(), e.Name())
		if raw, err := os.ReadFile(filepath.Join(dir, "pid")); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil && ownsRun(pid, dir) {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
		if err := os.RemoveAll(dir); err == nil {
			fmt.Fprintf(m.log, "vm: removed leftover machine %s\n", e.Name())
		}
	}
}

// ownsRun reports whether pid is a qemu started on dir's overlay, so a reused pid is spared.
func ownsRun(pid int, dir string) bool {
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	return err == nil && strings.Contains(string(cmdline), "qemu") && strings.Contains(string(cmdline), dir)
}
