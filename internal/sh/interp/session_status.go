package interp

// SetLastStatus sets the status expanded by $? in the next command. It is for
// session hosts that report a status without executing an interpreter command,
// such as cancellation. Like Run, it must not be called concurrently.
func (r *Runner) SetLastStatus(status uint8) {
	r.lastExit = exitStatus{code: status}
}
