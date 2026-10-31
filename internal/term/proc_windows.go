package term

import (
	"golang.org/x/sys/windows"
)

type windowsProc struct {
	id      int
	process windows.Handle
	conn    *windowsConn
	status  chan int
}

func (p *windowsProc) ID() int            { return p.id }
func (p *windowsProc) Status() <-chan int { return p.status }
func (p *windowsProc) Kill()              { p.conn.kill() }
func (p *windowsProc) wait() {
	code := uint32(1)
	if result, err := windows.WaitForSingleObject(p.process, windows.INFINITE); err == nil && result == windows.WAIT_OBJECT_0 {
		if err := windows.GetExitCodeProcess(p.process, &code); err != nil {
			code = 1
		}
	}
	windows.CloseHandle(p.process)
	// Closing the pseudoconsole allows its output pipe to reach EOF; the
	// reader must continue draining while ClosePseudoConsole runs.
	// TODO(P03): coordinate leader exit with live console descendants.
	go p.conn.closeConsole()
	p.status <- int(code)
}
