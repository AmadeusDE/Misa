package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AmadeusDE/Misa/testapi"
)

func main() {
	name := filepath.Base(os.Args[0])
	if name == "multicall" {
		if len(os.Args) < 2 {
			fmt.Fprintf(os.Stderr, "usage: multicall <command> [args...]\n")
			os.Exit(1)
		}
		name = os.Args[1]
		os.Args = os.Args[1:]
	}
	cmd, ok := testapi.Lookup(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "%s: unknown command\n", name)
		os.Exit(127)
	}
	d, _ := os.Getwd()
	if d == "" {
		d = "/"
	}
	ctx := testapi.NewContext(d, os.Args[1:], name, 80, 24, os.Stdout, os.Stderr, os.Stdin)
	os.Exit(cmd.Run(ctx))
}
