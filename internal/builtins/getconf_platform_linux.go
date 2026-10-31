//go:build linux && !android

package builtins

// getconfPlatform is the Linux table. Every value is a limit the kernel
// documents, not a probe of the running system: include/uapi/linux/limits.h
// gives ARG_MAX, LINK_MAX, MAX_CANON, MAX_INPUT, NAME_MAX, PATH_MAX and
// PIPE_BUF; USER_HZ is 100 (CLK_TCK); utsname's nodename holds 65 bytes, so
// HOST_NAME_MAX is 64; the kernel walks at most 40 symlinks (MAXSYMLINKS);
// UIO_MAXIOV is 1024; LINE_MAX is POSIX's 2048-byte minimum, which Linux does
// not lower. See docs/plans/getconf.md.
var getconfPlatform = getconfLimits{
	argMax:       131072,
	clkTck:       100,
	hostNameMax:  64,
	lineMax:      2048,
	iovMax:       1024,
	loginNameMax: 256,
	ngroupsMax:   65536,
	openMax:      1024,
	pathMax:      4096,
	nameMax:      255,
	linkMax:      127,
	pipeBuf:      4096,
	maxCanon:     255,
	maxInput:     255,
	symlinkMax:   -1,
	symloopMax:   40,
	ttyNameMax:   32,
	tzNameMax:    6,
	vdisable:     0,
	fileSizeBits: 64,
	confstrPath:  "/usr/bin:/bin",
}
