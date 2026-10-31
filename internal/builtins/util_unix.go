//go:build unix

package builtins

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

var signalNames = map[string]syscall.Signal{
	"HUP":   syscall.SIGHUP,
	"INT":   syscall.SIGINT,
	"QUIT":  syscall.SIGQUIT,
	"ILL":   syscall.SIGILL,
	"TRAP":  syscall.SIGTRAP,
	"ABRT":  syscall.SIGABRT,
	"BUS":   syscall.SIGBUS,
	"FPE":   syscall.SIGFPE,
	"KILL":  syscall.SIGKILL,
	"USR1":  syscall.SIGUSR1,
	"SEGV":  syscall.SIGSEGV,
	"USR2":  syscall.SIGUSR2,
	"PIPE":  syscall.SIGPIPE,
	"ALRM":  syscall.SIGALRM,
	"TERM":  syscall.SIGTERM,
	"CHLD":  syscall.SIGCHLD,
	"CONT":  syscall.SIGCONT,
	"STOP":  syscall.SIGSTOP,
	"TSTP":  syscall.SIGTSTP,
	"TTIN":  syscall.SIGTTIN,
	"TTOU":  syscall.SIGTTOU,
	"URG":   syscall.SIGURG,
	"XCPU":  syscall.SIGXCPU,
	"XFSZ":  syscall.SIGXFSZ,
	"PROF":  syscall.SIGPROF,
	"WINCH": syscall.SIGWINCH,
	"IO":    syscall.SIGIO,
	"PWR":   syscall.Signal(30),
	"SYS":   syscall.SIGSYS,
}

func signalByName(name string) (syscall.Signal, bool) {
	name = strings.TrimPrefix(strings.ToUpper(name), "SIG")
	if sig, ok := signalNames[name]; ok {
		return sig, true
	}
	n, err := strconv.Atoi(name)
	if err == nil && n > 0 && n < 65 {
		return syscall.Signal(n), true
	}
	return 0, false
}

type procInfo struct {
	pid     int
	comm    string
	cmdline string
	uid     uint32
	ppid    int
}

func scanProcs() ([]procInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	var procs []procInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		p := procInfo{pid: pid}

		if data, err := os.ReadFile(filepath.Join("/proc", e.Name(), "comm")); err == nil {
			p.comm = strings.TrimSpace(string(data))
		} else {
			continue
		}

		if data, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline")); err == nil {
			p.cmdline = strings.ReplaceAll(strings.TrimRight(string(data), "\x00"), "\x00", " ")
		}

		if info, err := os.Stat(filepath.Join("/proc", e.Name())); err == nil {
			if st, ok := info.Sys().(*syscall.Stat_t); ok {
				p.uid = st.Uid
			}
		}

		if data, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat")); err == nil {
			s := string(data)
			if closeP := strings.LastIndex(s, ")"); closeP > 0 && closeP+2 < len(s) {
				fields := strings.Fields(s[closeP+2:])
				if len(fields) >= 2 {
					p.ppid, _ = strconv.Atoi(fields[1])
				}
			}
		}

		procs = append(procs, p)
	}
	return procs, nil
}
