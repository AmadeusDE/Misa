package term

import "errors"

// Stopped reports whether the program is suspended, waiting for Continue.
func (t *Terminal) Stopped() bool { return t.stopped.Load() }

// Continue resumes a stopped program, as fg and bg do. Platforms without job
// control report an error.
func (t *Terminal) Continue() error {
	c, ok := t.proc.(continuer)
	if !ok {
		return errors.New("no process to continue")
	}
	t.stopped.Store(false)
	return c.Continue()
}
