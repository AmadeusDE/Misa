//go:build linux

// Linux/Android procfs backend. The stat field layout, the "last ')' ends the
// comm field" parsing rule and the TTY device-number naming follow toybox
// toys/posix/ps.c (0BSD, commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb); see
// docs/CREDITS.md. Real ids come from status rather than the process
// directory's owner so a value is never invented.

package proc

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func (r *Reader) checkSupported() error { return nil }

// readProcess reads one process. It reports ok=false when the process is gone
// (no stat file) or the stat file is unparsable; missing privileges on the
// optional files only leave those fields empty.
func (r *Reader) readProcess(pid int, opts Options) (Process, bool) {
	dir := r.Dir(pid)
	stat, err := os.ReadFile(filepath.Join(dir, "stat"))
	if err != nil {
		return Process{}, false
	}
	p, ok := parseStat(pid, stat)
	if !ok {
		return Process{}, false
	}
	r.readIDs(&p, dir)
	p.TTYName = TTYName(p.TTY)
	if opts.Argv {
		p.Argv = readArgv(filepath.Join(dir, "cmdline"))
	}
	if opts.Exe {
		if exe, err := os.Readlink(filepath.Join(dir, "exe")); err == nil {
			p.Exe = strings.TrimSuffix(exe, " (deleted)")
		}
	}
	return p, true
}

// parseStat parses /proc/PID/stat. The comm field is wrapped in parentheses and
// may itself contain ')' and spaces, so the fields resume after the last ')'.
func parseStat(pid int, data []byte) (Process, bool) {
	s := string(data)
	open := strings.IndexByte(s, '(')
	close := strings.LastIndexByte(s, ')')
	if open < 0 || close <= open || close+1 >= len(s) {
		return Process{}, false
	}
	p := Process{PID: pid, Comm: s[open+1 : close]}
	fields := strings.Fields(s[close+1:])
	if len(fields) < 22 || len(fields[0]) == 0 {
		return Process{}, false
	}
	p.State = fields[0][0]
	p.PPID = atoi(fields[1])
	p.PGID = atoi(fields[2])
	p.SID = atoi(fields[3])
	p.TTY = atoi(fields[4])
	p.Tpgid = atoi(fields[5])
	p.UTicks = atou(fields[11])
	p.STicks = atou(fields[12])
	p.Priority = int32(atoi(fields[15]))
	p.Nice = int32(atoi(fields[16]))
	p.Threads = atoi(fields[17])
	p.StartTicks = atou(fields[19])
	p.VSZ = atou(fields[20])
	p.RSS = atou(fields[21])
	return p, true
}

// readIDs fills the ids. The real ids need /proc/PID/status; without it the
// owner of the process directory supplies the effective ids and IDsExact stays
// false so callers know the real ids are unknown.
func (r *Reader) readIDs(p *Process, dir string) {
	if data, err := os.ReadFile(filepath.Join(dir, "status")); err == nil {
		uid, gid, ok := parseStatusIDs(string(data))
		if ok {
			p.RUID, p.UID = uid[0], uid[1]
			p.RGID, p.GID = gid[0], gid[1]
			p.IDsExact = true
			return
		}
	}
	if fi, err := os.Stat(dir); err == nil {
		if st, ok := fi.Sys().(*syscall.Stat_t); ok {
			p.UID, p.GID = st.Uid, st.Gid
			p.RUID, p.RGID = st.Uid, st.Gid
		}
	}
}

// parseStatusIDs reads the "Uid:" and "Gid:" lines: real, effective, saved, fs.
func parseStatusIDs(status string) (uid, gid [2]uint32, ok bool) {
	var gotUID, gotGID bool
	for _, line := range strings.Split(status, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		switch fields[0] {
		case "Uid:":
			uid[0], uid[1], gotUID = atou32(fields[1]), atou32(fields[2]), true
		case "Gid:":
			gid[0], gid[1], gotGID = atou32(fields[1]), atou32(fields[2]), true
		}
	}
	return uid, gid, gotUID && gotGID
}

// readArgv splits a cmdline file on NUL bytes. A kernel thread has an empty
// cmdline, which yields a nil slice rather than an empty argument.
func readArgv(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	argv := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
	if len(argv) == 1 && argv[0] == "" {
		return nil
	}
	return argv
}

// TTYName turns stat's tty device number into a name. The kernel encodes the
// number as (major<<8)|minor for small values and in the extended form above
// that, so the major and minor need unpacking.
func TTYName(dev int) string {
	if dev <= 0 {
		return "?"
	}
	major := (dev >> 8) & 0xfff
	minor := (dev & 0xff) | ((dev >> 12) & 0xfff00)
	switch {
	case major >= 136 && major <= 143:
		return "pts/" + strconv.Itoa((major-136)<<8|minor)
	case major == 4 && minor < 64:
		return "tty" + strconv.Itoa(minor)
	case major == 4:
		return "ttyS" + strconv.Itoa(minor-64)
	case major == 5 && minor == 1:
		return "console"
	case major == 5 && minor == 2:
		return "ptmx"
	}
	return strconv.Itoa(major) + ":" + strconv.Itoa(minor)
}

// The remaining readers each read one procfs file from the reader's root, so a
// fixture tree exercises them exactly like the process snapshot. A platform
// without a backend returns ErrUnsupported instead (see proc_other.go).

// Uptime reads the time since boot from <root>/uptime.
func (r *Reader) Uptime() (time.Duration, error) {
	data, err := os.ReadFile(filepath.Join(r.root, "uptime"))
	if err != nil {
		return 0, err
	}
	return ParseUptime(data)
}

// Meminfo reads <root>/meminfo.
func (r *Reader) Meminfo() (Meminfo, error) {
	data, err := os.ReadFile(filepath.Join(r.root, "meminfo"))
	if err != nil {
		return Meminfo{}, err
	}
	return ParseMeminfo(data)
}

// Loadavg reads <root>/loadavg.
func (r *Reader) Loadavg() (Loadavg, error) {
	data, err := os.ReadFile(filepath.Join(r.root, "loadavg"))
	if err != nil {
		return Loadavg{}, err
	}
	return ParseLoadavg(data)
}

// Stat reads <root>/stat.
func (r *Reader) Stat() (Stat, error) {
	data, err := os.ReadFile(filepath.Join(r.root, "stat"))
	if err != nil {
		return Stat{}, err
	}
	return ParseStat(data)
}

// Vmstat reads <root>/vmstat.
func (r *Reader) Vmstat() (Vmstat, error) {
	data, err := os.ReadFile(filepath.Join(r.root, "vmstat"))
	if err != nil {
		return Vmstat{}, err
	}
	return ParseVmstat(data)
}
