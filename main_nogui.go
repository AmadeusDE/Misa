//go:build nogui

package main

import (
	"fmt"
	"os"
)

func runGUI(string, []string, string) int {
	fmt.Fprintln(os.Stderr, "misa: built without the Wayland window (-tags nogui)")
	return 1
}
