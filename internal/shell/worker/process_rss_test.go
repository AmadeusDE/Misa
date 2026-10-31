//go:build linux && !android

package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// BenchmarkProcessRSS reports resident KiB for an idle child after handshake.
// It is a measurement, not an assertion; runtime and platform affect it.
func BenchmarkProcessRSS(b *testing.B) {
	exe, err := os.Executable()
	if err != nil {
		b.Skip(err)
	}
	pidfile := filepath.Join(b.TempDir(), "pid")
	var total uint64
	for b.Loop() {
		c, err := StartProcess(context.Background(), ProcessOptions{Executable: exe, Args: []string{"-test.run=^TestProcessHelper$"}, Env: []string{"MISA_WORKER_HELPER=1", "MISA_WORKER_PIDFILE=" + pidfile}})
		if errors.Is(err, os.ErrPermission) {
			b.Skip(err)
		}
		if err != nil {
			b.Fatal(err)
		}
		data, err := os.ReadFile(pidfile)
		if err != nil {
			b.Fatal(err)
		}
		pid, err := strconv.Atoi(string(data))
		if err != nil {
			b.Fatal(err)
		}
		status, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
		if err != nil {
			b.Fatal(err)
		}
		found := false
		for _, line := range strings.Split(string(status), "\n") {
			var rss uint64
			if _, err := fmt.Sscanf(line, "VmRSS: %d kB", &rss); err == nil {
				total += rss
				found = true
				break
			}
		}
		if !found {
			b.Fatal("VmRSS missing")
		}
		if err = c.Shutdown(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(total)/float64(b.N), "KiB-child")
}
