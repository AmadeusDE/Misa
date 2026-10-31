//go:build unix

package builtins

import (
	"fmt"
	"io"
	"strconv"
	"syscall"
)

// platformSignalNote is unused on Unix, where every signal can be sent.
const platformSignalNote = ""

var sigNumberMap = map[int]string{
	0:  "EXIT",
	1:  "HUP",
	2:  "INT",
	3:  "QUIT",
	4:  "ILL",
	5:  "TRAP",
	6:  "ABRT",
	7:  "BUS",
	8:  "FPE",
	9:  "KILL",
	10: "USR1",
	11: "SEGV",
	12: "USR2",
	13: "PIPE",
	14: "ALRM",
	15: "TERM",
	16: "STKFLT",
	17: "CHLD",
	18: "CONT",
	19: "STOP",
	20: "TSTP",
	21: "TTIN",
	22: "TTOU",
	23: "URG",
	24: "XCPU",
	25: "XFSZ",
	26: "VTALRM",
	27: "PROF",
	28: "WINCH",
	29: "IO",
	30: "PWR",
	31: "SYS",
}

func signalNumberName(n int) (string, bool) {
	if name, ok := sigNumberMap[n]; ok {
		return name, true
	}
	if n >= 32 && n <= 64 {
		return strconv.Itoa(n), true
	}
	return "", false
}

func printSignalTable(w io.Writer) {
	for i := 0; i <= 31; i++ {
		if name, ok := sigNumberMap[i]; ok {
			fmt.Fprintf(w, "%2d %s\n", i, name)
		}
	}
	for i := 34; i <= 64; i++ {
		switch i {
		case 34:
			fmt.Fprintf(w, "%2d RTMIN\n", i)
		case 64:
			fmt.Fprintf(w, "%2d RTMAX\n", i)
		default:
			fmt.Fprintf(w, "%2d RTMIN+%d\n", i, i-34)
		}
	}
}

func listedSignals() []string {
	return []string{
		"EXIT", "HUP", "INT", "QUIT", "ILL", "TRAP", "ABRT", "BUS",
		"FPE", "KILL", "USR1", "SEGV", "USR2", "PIPE", "ALRM", "TERM",
		"STKFLT", "CHLD", "CONT", "STOP", "TSTP", "TTIN", "TTOU", "URG",
		"XCPU", "XFSZ", "VTALRM", "PROF", "WINCH", "IO", "PWR", "SYS",
		"RTMIN", "RTMAX",
	}
}

func signalSupported(syscall.Signal) bool { return true }

func sendSignal(pid int, sig syscall.Signal) error { return syscall.Kill(pid, sig) }

func killFailure(error) string { return "No such process" }
