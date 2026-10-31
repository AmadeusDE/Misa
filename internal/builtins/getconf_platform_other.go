//go:build !linux && !darwin && !freebsd && !openbsd && !android

package builtins

// getconfPlatform is the fallback table for platforms Misa has no limit table
// for: every platform-dependent name reports "undefined" rather than a guess.
var getconfPlatform = getconfLimits{
	argMax:       -1,
	clkTck:       -1,
	hostNameMax:  -1,
	lineMax:      -1,
	iovMax:       -1,
	loginNameMax: -1,
	ngroupsMax:   -1,
	openMax:      -1,
	pathMax:      -1,
	nameMax:      -1,
	linkMax:      -1,
	pipeBuf:      -1,
	maxCanon:     -1,
	maxInput:     -1,
	symlinkMax:   -1,
	symloopMax:   -1,
	ttyNameMax:   -1,
	tzNameMax:    -1,
	vdisable:     -1,
	fileSizeBits: -1,
}

// platformCacheInfo: no cache geometry source on this platform.
func platformCacheInfo() map[string]string { return nil }

// platformOpenMax, platformChildMax: no rlimit source on this platform.
func platformOpenMax() (int64, bool)  { return 0, false }
func platformChildMax() (int64, bool) { return 0, false }
