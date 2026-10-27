//go:build !unix || android

package gui

import (
	"fmt"
	"os"
)

// Run reports that this platform has no window backend. Android compiles
// the GUI core (the Wayland files are unix && !android) and fails cleanly,
// including when WAYLAND_DISPLAY is set; the ANativeWindow backend arrives
// with phase A3. The same stub covers Windows until W3.
func Run(dir string, argv []string, title string) int {
	fmt.Fprintln(os.Stderr, "misa: gui: no window frontend on this platform")
	return 1
}
