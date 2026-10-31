//go:build !linux

package proc

// Non-Linux platforms have no procfs snapshot backend yet: BSD and macOS
// process data would have to come from sysctl, which this package does not
// implement. Callers get ErrUnsupported rather than fabricated fields.

func (r *Reader) checkSupported() error { return ErrUnsupported }

func (r *Reader) readProcess(pid int, opts Options) (Process, bool) { return Process{}, false }
