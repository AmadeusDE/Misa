//go:build unix

package procmon

import "syscall"

var unixSignals = []signalEntry{
	{0, "Cancel", 0},
	{1, "SIGHUP", syscall.SIGHUP},
	{2, "SIGINT", syscall.SIGINT},
	{3, "SIGQUIT", syscall.SIGQUIT},
	{4, "SIGILL", syscall.SIGILL},
	{5, "SIGTRAP", syscall.SIGTRAP},
	{6, "SIGABRT", syscall.SIGABRT},
	{6, "SIGIOT", syscall.SIGABRT},
	{7, "SIGBUS", syscall.SIGBUS},
	{8, "SIGFPE", syscall.SIGFPE},
	{9, "SIGKILL", syscall.SIGKILL},
	{10, "SIGUSR1", syscall.SIGUSR1},
	{11, "SIGSEGV", syscall.SIGSEGV},
	{12, "SIGUSR2", syscall.SIGUSR2},
	{13, "SIGPIPE", syscall.SIGPIPE},
	{14, "SIGALRM", syscall.SIGALRM},
	{15, "SIGTERM", syscall.SIGTERM},
	{16, "SIGSTKFLT", syscall.Signal(16)},
	{17, "SIGCHLD", syscall.SIGCHLD},
	{18, "SIGCONT", syscall.SIGCONT},
	{19, "SIGSTOP", syscall.SIGSTOP},
	{20, "SIGTSTP", syscall.SIGTSTP},
	{21, "SIGTTIN", syscall.SIGTTIN},
	{22, "SIGTTOU", syscall.SIGTTOU},
	{23, "SIGURG", syscall.SIGURG},
	{24, "SIGXCPU", syscall.SIGXCPU},
	{25, "SIGXFSZ", syscall.SIGXFSZ},
	{26, "SIGVTALRM", syscall.Signal(26)},
	{27, "SIGPROF", syscall.SIGPROF},
	{28, "SIGWINCH", syscall.SIGWINCH},
	{29, "SIGIO", syscall.SIGIO},
	{30, "SIGPWR", syscall.Signal(30)},
	{31, "SIGSYS", syscall.SIGSYS},
}

var platformActions = Actions{
	Signals: unixSignals,
	send:    func(pid int, sig syscall.Signal) error { return syscall.Kill(pid, sig) },
	renice:  func(pid, nice int) error { return syscall.Setpriority(syscall.PRIO_PROCESS, pid, nice) },
}
