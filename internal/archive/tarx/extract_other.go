//go:build !linux

// Extraction behavior is informed by Toybox tar.c (0BSD); see NOTICE.
package tarx

import (
	"archive/tar"
	"fmt"
	"os"
)

func makeSpecial(p *os.Root, n string, h *tar.Header) error {
	return fmt.Errorf("special files unsupported on this platform")
}

func (x *extractor) specialMetadata(p *os.Root, n string, h *tar.Header) error {
	return fmt.Errorf("special metadata unsupported on this platform")
}
