//go:build !noweb

package main

import (
	"os"

	"github.com/AmadeusDE/Misa/internal/mweb"
)

func runWeb(args []string) int { return mweb.Run(args, os.Stderr) }

func runWebAdmin(args []string) int { return mweb.Admin(args, os.Stdout, os.Stdin) }
