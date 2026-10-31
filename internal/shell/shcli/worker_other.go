//go:build !unix

package shcli

import (
	"context"
	"fmt"
)

func runWorker(_ context.Context, name string, opts Options) int {
	if opts.Stderr != nil {
		fmt.Fprintf(opts.Stderr, "%s: worker: unsupported platform\n", name)
	}
	return 1
}
