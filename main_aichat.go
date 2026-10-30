//go:build !nough

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/AmadeusDE/Misa/internal/aichat"
)

// runAIChat is `misa aichat [-m provider/model] [conversation-id]` on the whole terminal.
func runAIChat(args []string) int {
	a, err := aichat.ParseArgs(args)
	if err == nil {
		err = aichat.Run(aichat.Options{Model: a.Model, ID: a.ID})
	}
	if errors.Is(err, aichat.ErrHelp) {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "aichat:", err)
		return 1
	}
	return 0
}
