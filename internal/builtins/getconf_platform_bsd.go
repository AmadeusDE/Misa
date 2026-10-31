//go:build freebsd || openbsd

package builtins

import "runtime"

// getconfPlatform is the BSD table: the limits <sys/syslimits.h> documents on
// both (ARG_MAX 256 KiB, PATH_MAX 1024, NAME_MAX 255, LINK_MAX 32767, PIPE_BUF
// 512, OPEN_MAX 64, SYMLOOP_MAX 32) plus POSIX's 2048-byte LINE_MAX minimum,
// which neither lowers. FreeBSD ticks at 128 Hz and allows 1023 supplementary
// groups; OpenBSD ticks at 100 Hz with NGROUPS_MAX 16. The values this build
// cannot state without a C library (LOGIN_NAME_MAX, TTY_NAME_MAX, TZNAME_MAX,
// MAX_CANON, MAX_INPUT, SYMLINK_MAX) are -1, i.e. "undefined".
// See docs/plans/getconf.md.
var getconfPlatform = func() getconfLimits {
	p := getconfLimits{
		argMax:       262144,
		clkTck:       100,
		hostNameMax:  255,
		lineMax:      2048,
		iovMax:       1024,
		loginNameMax: -1,
		ngroupsMax:   16,
		openMax:      64,
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
		confstrPath:  "/usr/bin:/bin",
	}
	if runtime.GOOS == "freebsd" {
		p.clkTck = 128
		p.ngroupsMax = 1023
	}
	return p
}()

// platformCacheInfo: there is no cache geometry source here without cgo.
func platformCacheInfo() map[string]string { return nil }
