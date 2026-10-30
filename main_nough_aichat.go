//go:build nough

package main

import (
	"fmt"
	"os"
)

func runAIChat([]string) int {
	fmt.Fprintln(os.Stderr, "aichat: built without AI support (-tags nough)")
	return 1
}
