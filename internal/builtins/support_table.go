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
}
