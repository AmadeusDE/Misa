//go:build unix && !linux

package wayland

import "os"

func anonymousFile() (*os.File, error) { return anonymousFileInDir() }
