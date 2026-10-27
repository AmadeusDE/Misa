package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

var names = map[string]syscall.Errno{
	"EPERM":        syscall.EPERM,
	"ENOENT":       syscall.ENOENT,
	"ESRCH":        syscall.ESRCH,
	"EINTR":        syscall.EINTR,
	"EIO":          syscall.EIO,
	"ENXIO":        syscall.ENXIO,
	"E2BIG":        syscall.E2BIG,
	"ENOEXEC":      syscall.ENOEXEC,
	"EBADF":        syscall.EBADF,
	"ECHILD":       syscall.ECHILD,
	"EDEADLK":      syscall.EDEADLK,
	"ENOMEM":       syscall.ENOMEM,
	"EACCES":       syscall.EACCES,
	"EFAULT":       syscall.EFAULT,
	"ENOTBLK":      syscall.ENOTBLK,
	"EBUSY":        syscall.EBUSY,
	"EEXIST":       syscall.EEXIST,
	"EXDEV":        syscall.EXDEV,
	"ENODEV":       syscall.ENODEV,
	"ENOTDIR":      syscall.ENOTDIR,
	"EISDIR":       syscall.EISDIR,
	"EINVAL":       syscall.EINVAL,
	"ENFILE":       syscall.ENFILE,
	"EMFILE":       syscall.EMFILE,
	"ENOTTY":       syscall.ENOTTY,
	"ETXTBSY":      syscall.ETXTBSY,
	"EFBIG":        syscall.EFBIG,
	"ENOSPC":       syscall.ENOSPC,
	"ESPIPE":       syscall.ESPIPE,
	"EROFS":        syscall.EROFS,
	"EMLINK":       syscall.EMLINK,
	"EPIPE":        syscall.EPIPE,
	"EDOM":         syscall.EDOM,
	"ERANGE":       syscall.ERANGE,
	"EAGAIN":       syscall.EAGAIN,
	"ENOSYS":       syscall.ENOSYS,
	"ENOTEMPTY":    syscall.ENOTEMPTY,
	"ELOOP":        syscall.ELOOP,
	"ENAMETOOLONG": syscall.ENAMETOOLONG,
	"EOVERFLOW":    syscall.EOVERFLOW,
	"EILSEQ":       syscall.EILSEQ,
	"ENOTSUP":      syscall.ENOTSUP,
	"EOPNOTSUPP":   syscall.EOPNOTSUPP,
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: strerror <errno-name|number>...\n")
		os.Exit(1)
	}
	for _, arg := range os.Args[1:] {
		upper := strings.ToUpper(arg)
		if e, ok := names[upper]; ok {
			fmt.Println(e.Error())
		} else if n, err := strconv.Atoi(arg); err == nil {
			fmt.Println(syscall.Errno(n).Error())
		} else {
			fmt.Fprintf(os.Stderr, "strerror: unknown errno: %s\n", arg)
			os.Exit(1)
		}
	}
}
