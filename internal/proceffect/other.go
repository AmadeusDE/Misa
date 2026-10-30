//go:build !unix

package proceffect

import (
	"context"
	"errors"
)

var errUnsupported = errors.New("process effects unsupported")

func run(context.Context, Request, Invocation) int { return 125 }
func umask() (int, error)                          { return 0, errUnsupported }
func niceness() (int, error)                       { return 0, errUnsupported }
func handle([]string, func([]string) int) int      { return 125 }
func execTarget([]string) int                      { return 125 }
