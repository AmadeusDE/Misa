//go:build !linux

package proc

import "time"

// Non-Linux platforms have no procfs snapshot backend yet: BSD and macOS
// process data would have to come from sysctl, which this package does not
// implement. Callers get ErrUnsupported rather than fabricated fields.

func (r *Reader) checkSupported() error { return ErrUnsupported }

func (r *Reader) readProcess(pid int, opts Options) (Process, bool) { return Process{}, false }

// Uptime, Meminfo, Loadavg, Stat and Vmstat have no backend outside procfs
// either.
func (r *Reader) Uptime() (time.Duration, error) { return 0, ErrUnsupported }

func (r *Reader) Meminfo() (Meminfo, error) { return Meminfo{}, ErrUnsupported }

func (r *Reader) Loadavg() (Loadavg, error) { return Loadavg{}, ErrUnsupported }

func (r *Reader) Stat() (Stat, error) { return Stat{}, ErrUnsupported }

func (r *Reader) Vmstat() (Vmstat, error) { return Vmstat{}, ErrUnsupported }
