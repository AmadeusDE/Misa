//go:build darwin

package builtins

// getconfPlatform is the Darwin table: the limits macOS's <sys/syslimits.h>
// documents (ARG_MAX 1 MiB, PATH_MAX 1024, NAME_MAX 255, LINK_MAX 32767,
// PIPE_BUF 512, NGROUPS_MAX 16, OPEN_MAX 256, SYMLOOP_MAX 32) plus POSIX's
// 2048-byte LINE_MAX minimum, which macOS does not lower. The values this build
// cannot state without a C library (LOGIN_NAME_MAX, TTY_NAME_MAX, TZNAME_MAX,
// MAX_CANON, MAX_INPUT, SYMLINK_MAX) are -1, i.e. "undefined".
// See docs/plans/getconf.md.
var getconfPlatform = getconfLimits{
	argMax:       1048576,
	clkTck:       100,
	hostNameMax:  255,
	lineMax:      2048,
	iovMax:       1024,
	loginNameMax: -1,
	ngroupsMax:   16,
	openMax:      256,
	pathMax:      1024,
	nameMax:      255,
	linkMax:      32767,
	pipeBuf:      512,
	maxCanon:     -1,
	maxInput:     -1,
	symlinkMax:   -1,
	symloopMax:   32,
	ttyNameMax:   -1,
	tzNameMax:    -1,
	vdisable:     0,
	fileSizeBits: 64,
	confstrPath:  "/usr/bin:/bin:/usr/sbin:/sbin",
}

// platformCacheInfo: there is no cache geometry source here without cgo.
func platformCacheInfo() map[string]string { return nil }
