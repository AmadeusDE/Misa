package builtins

// utimeNow and utimeOmit are UTIME_NOW and UTIME_OMIT from <sys/stat.h>;
// x/sys/unix doesn't export them for darwin.
const (
	utimeNow  = -1
	utimeOmit = -2
)
