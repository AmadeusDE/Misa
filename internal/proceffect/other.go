//go:build !unix

package proceffect

import (
	"context"
	"errors"

	"github.com/AmadeusDE/Misa/internal/commandexec"
)

var errUnsupported = errors.New("process effects unsupported")

func run(context.Context, Request, Invocation) int             { return 125 }
func umask() (int, error)                                      { return 0, errUnsupported }
func niceness() (int, error)                                   { return 0, errUnsupported }
func handle([]string, func(context.Context, []string) int) int { return 125 }
func execTarget([]string) int                                  { return 125 }

func sharesGroup() bool { return false }
func start(context.Context, Request, Invocation, commandexec.Options) (*commandexec.Process, error) {
	return nil, errUnsupported
}
