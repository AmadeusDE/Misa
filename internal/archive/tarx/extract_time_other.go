//go:build !unix

// Extraction behavior is informed by Toybox tar.c (0BSD); see NOTICE.
package tarx

import (
	"archive/tar"
	"fmt"
	"os"
	"time"
)

func fileTimes(f *os.File, at, mt time.Time) error {
	return fmt.Errorf("file timestamp restoration unsupported on this platform")
}

func symlinkTimes(p *os.Root, name string, h *tar.Header) error {
	return fmt.Errorf("symlink timestamp restoration unsupported on this platform")
}
