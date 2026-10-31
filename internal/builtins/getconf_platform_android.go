//go:build android

package builtins

// getconfPlatform is the Linux table (Android is Linux) with Android's own
// command directory: there is no /usr/bin on Android, so confstr(3)'s PATH is
// /system/bin. See docs/plans/getconf.md.
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
	confstrPath:  "/system/bin",
}
