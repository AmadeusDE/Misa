package builtins

// utimeNow is UTIME_NOW from <sys/stat.h>; x/sys/unix doesn't export it for darwin.
const utimeNow = -1
