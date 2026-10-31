//go:build unix

package shcli

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/AmadeusDE/Misa/internal/shell/session"
	"github.com/AmadeusDE/Misa/internal/shell/sessionbuiltins"
	"github.com/AmadeusDE/Misa/internal/shell/worker"
	"golang.org/x/sys/unix"
)

func runWorker(ctx context.Context, name string, opts Options) int {
	fail := func(err error) int {
		if opts.Stderr != nil {
			fmt.Fprintf(opts.Stderr, "%s: worker: %v\n", name, err)
		}
		return 1
	}
	typ, err := unix.GetsockoptInt(3, unix.SOL_SOCKET, unix.SO_TYPE)
	if err != nil {
		return fail(err)
	}
	if typ != unix.SOCK_STREAM {
		return fail(fmt.Errorf("fd 3 is not a stream socket"))
	}
	if _, err = unix.Getpeername(3); err != nil {
		return fail(err)
	}
	unix.CloseOnExec(3)
	f := os.NewFile(3, "worker control")
	if f == nil {
		return fail(fmt.Errorf("invalid fd 3"))
	}
	control, err := net.FileConn(f)
	_ = f.Close()
	if err != nil {
		return fail(err)
	}
	s := session.Options{Dir: opts.Dir, Env: opts.Env, Stdin: opts.Stdin, Stdout: opts.Stdout, Stderr: opts.Stderr}
	if err = worker.Serve(ctx, control, worker.Options{Session: *sessionbuiltins.Configure(&s)}); err != nil {
		return fail(err)
	}
	return 0
}
