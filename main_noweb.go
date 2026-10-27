//go:build noweb

package main

import (
	"fmt"
	"os"
)

func runWeb([]string) int {
	fmt.Fprintln(os.Stderr, "misa: built without the web UI (-tags noweb)")
	return 1
}

func runWebAdmin([]string) int { return runWeb(nil) }
