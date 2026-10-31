package builtins

// windowsLimits is the filesystem builtins' support record for Windows. It
// lives in an untagged file so every platform's tests can check it against
// the registry. Unix (Android, BSD, macOS included) has no limits.
var windowsLimits = []Limit{
	{"chown", "", Unsupported, "Windows files have no numeric owner or group to change"},
	{"mkfifo", "", Unsupported, "Windows has no FIFO nodes in the file system"},
	{"stat", "-f", Unsupported, "statfs fields (type, id, name length, inode counts) are not available"},
	{"stat", "format", Partial, "owner, group, blocks, IO block and change time (%u %g %U %G %b %o %z %Z) print ?; %f is built from the Go mode"},
	{"chmod", "", Partial, "only the owner write bit is honored, as the read-only attribute; other bits, setuid, setgid and sticky are not stored; -v reports the effective mode"},
	{"cp", "--preserve=ownership", Partial, "ownership is ignored; -p and -a still keep mode (read-only attribute) and times"},
	{"cp", "fifo", Partial, "named pipes are not recreated"},
	{"mv", "fifo", Partial, "named pipes are not recreated across devices"},
	{"du", "", Partial, "allocated sizes are not available, so sizes are apparent sizes"},
	{"ln", "-s", Partial, "creating a symbolic link needs Developer Mode or the symlink privilege; the OS error is reported"},
	{"mkdir", "-m", Partial, "only the read-only attribute of the final directory can be set; -p creates parents without a descriptor walk"},
	{"rm", "-i", Partial, "write protection is the read-only attribute; there is no search bit"},
	{"touch", "", Partial, "change time cannot be set; access and modification times keep 100 ns precision"},
	{"kill", "", Partial, "Windows has no signals: only 0 (is the process alive), TERM and KILL work, and both call TerminateProcess, so TERM is not graceful; kill -l lists only those; process groups (negative pids) are rejected"},
	{"timeout", "", Partial, "at the deadline the command is ended with TerminateProcess, so TERM is not graceful; its child processes are not ended until job objects land; --foreground has no extra effect"},
	{"timeout", "-s", Partial, "only 0, TERM and KILL; any other signal exits 125 with the reason"},
	{"nice", "", Partial, "no nice values: the adjustment picks a priority class for the new process (15 and up Idle, 1 to 14 Below normal, 0 Normal, -1 to -9 Above normal, -10 and below High) and Misa's own priority never changes; a Misa builtin runs in the shell process and keeps its priority, with a warning"},
	{"pgrep", "", Unsupported, "listing other processes needs the Windows process snapshot API, planned with the process monitor (I04)"},
	{"pkill", "", Unsupported, "listing other processes needs the Windows process snapshot API, planned with the process monitor (I04)"},
	{"sync", "no operands", Unsupported, "flushing every volume needs administrator rights on Windows; name the files to flush"},
	{"sync", "--file-system", Unsupported, "Windows cannot flush the file system of a file without administrator rights"},
	{"sync", "--data", Partial, "FlushFileBuffers writes data and metadata alike"},
	{"which", "", Partial, "a name is found only with its extension: PATHEXT is not expanded"},
}
