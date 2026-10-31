//go:build !(linux || darwin || freebsd || dragonfly || openbsd || netbsd || solaris)

// There is no fcntl on this port, so an iflag=/oflag= given with a redirected
// standard stream fails honestly instead of being ignored.
package builtins

import "os"

// ddFcntlSetFlags reports that this port cannot change an open descriptor.
func ddFcntlSetFlags(*os.File, int) error { return errPlatformFlag }

// ddClearStreamFlag reports that this port cannot change an open file's flags.
func ddClearStreamFlag(*os.File, int) (func(), bool) { return nil, false }
