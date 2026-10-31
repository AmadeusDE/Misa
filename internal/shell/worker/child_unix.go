//go:build unix

package worker

import (
	"io"
	"os"
	"os/exec"
)

// stdio pumps are independent of exec.Cmd.Wait: caller-owned I/O may block
// forever, but it must never prevent reaping or a bounded worker shutdown.
type processStreams struct {
	all   []*os.File
	child []*os.File
	input *os.File
	pumps []func()
}

func (s *processStreams) closeAll() {
	for _, f := range s.all {
		_ = f.Close()
	}
}
func (s *processStreams) closeInput() {
	if s.input != nil {
		_ = s.input.Close()
	}
}
func (s *processStreams) start() {
	for _, f := range s.child {
		_ = f.Close()
	}
	for _, pump := range s.pumps {
		go pump()
	}
}

func prepareStreams(cmd *exec.Cmd, opts ProcessOptions) (*processStreams, error) {
	s := new(processStreams)
	fail := func(err error) (*processStreams, error) { s.closeAll(); return nil, err }
	if opts.Stdin != nil {
		if f, ok := opts.Stdin.(*os.File); ok {
			cmd.Stdin = f
		} else {
			r, w, err := os.Pipe()
			if err != nil {
				return fail(err)
			}
			s.all = append(s.all, r, w)
			s.child = append(s.child, r)
			s.input = w
			cmd.Stdin = r
			s.pumps = append(s.pumps, func() { defer w.Close(); _, _ = io.Copy(w, opts.Stdin) })
		}
	}
	setup := func(dst io.Writer) (*os.File, error) {
		if dst == nil {
			return nil, nil
		}
		if f, ok := dst.(*os.File); ok {
			return f, nil
		}
		r, w, err := os.Pipe()
		if err != nil {
			return nil, err
		}
		s.all = append(s.all, r, w)
		s.child = append(s.child, w)
		s.pumps = append(s.pumps, func() { defer r.Close(); _, _ = io.Copy(dst, r) })
		return w, nil
	}
	out, err := setup(opts.Stdout)
	if err != nil {
		return fail(err)
	}
	if out != nil {
		cmd.Stdout = out
	}
	out, err = setup(opts.Stderr)
	if err != nil {
		return fail(err)
	}
	if out != nil {
		cmd.Stderr = out
	}
	return s, nil
}
