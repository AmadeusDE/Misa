//go:build !unix

// Extraction behavior is informed by Toybox tar.c (0BSD); see NOTICE.
package tarx

import (
	"fmt"
	"os"
)

func linkRoots(src *os.Root, old string, dst *os.Root, name string) error {
	return fmt.Errorf("hard links unsupported on this platform")
}
