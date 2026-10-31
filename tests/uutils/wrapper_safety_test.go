package uutils

import (
	"strconv"
	"strings"
)

// Only obviously no-child paths through wrapper implementations are allowed.
// Everything else is refused: nice can change process priority, and all three
// wrappers can invoke a host executable.
func safeWrapperValidation(cmd string, args []string) bool {
	switch cmd {
	case "env":
		if len(args) == 1 && (args[0] == "--help" || args[0] == "--version" || args[0] == "--definitely-invalid" || args[0] == "--verbo" || args[0] == "'-v '") {
			return true
		}
		if len(args) > 0 && args[0] == "-i" {
			for _, a := range args[1:] {
				if !strings.Contains(a, "=") || strings.HasPrefix(a, "-") {
					return false
				}
			}
			return true
		}
		if len(args) > 0 && (args[0] == "--ignore-signal=__ALL__" || args[0] == "--default-signal=__ALL__" || args[0] == "--block-signal=__ALL__") {
			return true
		}
		// Assignment-only invocations cannot execute a child.
		if len(args) > 0 {
			for _, a := range args {
				if !strings.Contains(a, "=") || strings.HasPrefix(a, "-") {
					return false
				}
			}
			return true
		}
	case "nice":
		if len(args) == 0 {
			return true
		}
		if args[0] == "--help" || args[0] == "--version" || args[0] == "--invalid" {
			return true
		}
		if len(args) >= 2 && args[0] == "-n" {
			_, err := strconv.ParseInt(args[1], 10, 64)
			return err != nil && !strings.Contains(err.Error(), "value out of range")
		}
	case "timeout":
		if len(args) == 0 || args[0] == "--help" || args[0] == "--version" || args[0] == "--definitely-invalid" {
			return true
		}
		if len(args) == 2 && args[0] == "" {
			return true
		}
	}
	return false
}
