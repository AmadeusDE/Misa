package awk

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Runner runs one shell command line to completion and returns its exit
// status. stdin, stdout and stderr are the command's streams. An error
// means the command could not be run at all; the status is then ignored
// unless it is non-zero. A cancelled ctx must make Runner kill the command
// and return.
type Runner func(ctx context.Context, cmd string, stdin io.Reader, stdout, stderr io.Writer) (int, error)

// lockedWriter serialises writes from the interpreter and from commands.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// lockedReader serialises reads of the standard input that commands and the
// main input may share. A real file is passed through untouched so a command
// still inherits the descriptor.
type lockedReader struct {
	mu sync.Mutex
	r  io.Reader
}

func (l *lockedReader) Read(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r.Read(p)
}

func shareable(r io.Reader) io.Reader {
	if _, ok := r.(*os.File); ok {
		return r
	}
	return &lockedReader{r: r}
}

type streamKind uint8

const (
	sOutFile streamKind = iota
	sOutPipe
	sInFile
	sInPipe
)

// stream is one open file or command, shared by every use of its name until
// it is closed.
type stream struct {
	name  string
	kind  streamKind
	seq   int
	w     *bufio.Writer // outputs
	file  io.Closer
	pipeW *io.PipeWriter
	rd    *reader // inputs
	pipeR *io.PipeReader
	done  chan int // exit status of the command of a pipe
	std   bool     // shared with the process: flushed on close, never closed
	eager bool     // flushed after every print (stderr)
}

type inKey struct {
	name string
	cmd  bool
}

type streams struct {
	outs map[string]*stream
	ins  map[inKey]*stream
	seq  int
}

func (in *interp) sharedStdin() *reader {
	if in.stdinR == nil {
		in.stdinR = newReader(flushReader{in.cfg.Stdin, in.out}, nil)
	}
	return in.stdinR
}

func (in *interp) path(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(in.cfg.Dir, name)
}

func (in *interp) register(st *stream) *stream {
	in.st.seq++
	st.seq = in.st.seq
	if st.kind == sOutFile || st.kind == sOutPipe {
		in.st.outs[st.name] = st
	} else {
		in.st.ins[inKey{st.name, st.kind == sInPipe}] = st
	}
	return st
}

// run starts cmd and reports its status on done when it finishes.
func (in *interp) startCmd(cmd string, stdin io.Reader, stdout io.Writer, onExit func()) chan int {
	if in.cfg.Run == nil {
		runtimeErrorf("cannot run %q: command execution is not available", cmd)
	}
	done := make(chan int, 1)
	go func() {
		st, err := in.cfg.Run(in.ctx, cmd, stdin, stdout, in.errw)
		if err != nil {
			fmt.Fprintf(in.errw, "awk: %s: %v\n", cmd, err)
			if st == 0 {
				st = -1
			}
		}
		onExit()
		done <- st
	}()
	return done
}

// output returns the stream a print redirects to.
func (in *interp) output(redir tokKind, name string) *stream {
	if st, ok := in.st.outs[name]; ok {
		return st
	}
	switch {
	case name == "/dev/stdout" || name == "-":
		return in.register(&stream{name: name, kind: sOutFile, w: in.out, std: true})
	case name == "/dev/stderr":
		return in.register(&stream{name: name, kind: sOutFile, w: bufio.NewWriter(in.errw), std: true, eager: true})
	case strings.HasPrefix(name, "/inet/"):
		runtimeErrorf("%s: network files are not supported", name)
	}
	if redir == tPipe {
		in.flushAll()
		pr, pw := io.Pipe()
		done := in.startCmd(name, pr, in.outw, func() { pr.Close() })
		return in.register(&stream{name: name, kind: sOutPipe, w: bufio.NewWriterSize(pw, 4096), pipeW: pw, done: done})
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if redir == tAppend {
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}
	f, err := os.OpenFile(in.path(name), flags, 0o666)
	if err != nil {
		runtimeErrorf("can't redirect to %s: %v", name, unwrapPath(err))
	}
	return in.register(&stream{name: name, kind: sOutFile, w: bufio.NewWriterSize(f, 32<<10), file: f})
}

func unwrapPath(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}

// input returns the stream getline reads, or nil when it cannot be opened.
func (in *interp) input(name string, cmd bool) *stream {
	if st, ok := in.st.ins[inKey{name, cmd}]; ok {
		return st
	}
	if cmd {
		in.flushAll()
		pr, pw := io.Pipe()
		done := in.startCmd(name, in.cfg.Stdin, pw, func() { pw.Close() })
		return in.register(&stream{name: name, kind: sInPipe, rd: newReader(pr, nil), pipeR: pr, done: done})
	}
	if name == "-" || name == "/dev/stdin" {
		return in.register(&stream{name: name, kind: sInFile, rd: in.sharedStdin(), std: true})
	}
	f, err := os.Open(in.path(name))
	if err != nil {
		return nil
	}
	return in.register(&stream{name: name, kind: sInFile, rd: newReader(f, f)})
}

// closeStream releases st and returns the status close() reports.
func (in *interp) closeStream(st *stream) int {
	status := 0
	switch st.kind {
	case sOutFile:
		if err := st.w.Flush(); err != nil {
			status = -1
		}
		if st.file != nil && st.file.Close() != nil {
			status = -1
		}
		delete(in.st.outs, st.name)
	case sOutPipe:
		if err := st.w.Flush(); err != nil && err != io.ErrClosedPipe {
			status = -1
		}
		st.pipeW.Close()
		status = <-st.done
		delete(in.st.outs, st.name)
	case sInFile:
		if !st.std {
			st.rd.close()
		}
		delete(in.st.ins, inKey{st.name, false})
	case sInPipe:
		st.pipeR.Close()
		status = <-st.done
		delete(in.st.ins, inKey{st.name, true})
	}
	return status
}

// closeNamed implements close(name): it closes every stream opened under
// the name and returns the status, or -1 when none was open.
func (in *interp) closeNamed(name string) int {
	status, found := -1, false
	in.flushAll()
	if st, ok := in.st.outs[name]; ok {
		status, found = in.closeStream(st), true
	}
	for _, cmd := range []bool{false, true} {
		if st, ok := in.st.ins[inKey{name, cmd}]; ok {
			status, found = in.closeStream(st), true
		}
	}
	if !found {
		return -1
	}
	in.checkDone()
	return status
}

func (in *interp) flushAll() {
	in.out.Flush()
	for _, st := range in.st.outs {
		if st.w != in.out {
			st.w.Flush()
		}
	}
}

// fflush implements fflush(): no name flushes everything.
func (in *interp) fflush(args []expr) Value {
	if len(args) == 0 {
		in.flushAll()
		return num(0)
	}
	name := in.eval(args[0]).text(in.convfmt)
	st, ok := in.st.outs[name]
	switch {
	case ok:
		st.w.Flush()
	case name == "/dev/stdout" || name == "/dev/stderr" || name == "-":
		in.out.Flush()
	default:
		return num(-1)
	}
	return num(0)
}

// shutdown flushes standard output and closes every stream in the order it
// was opened, waiting for commands to finish.
func (in *interp) shutdown() {
	in.out.Flush()
	var open []*stream
	for _, st := range in.st.outs {
		open = append(open, st)
	}
	for _, st := range in.st.ins {
		open = append(open, st)
	}
	sort.Slice(open, func(i, j int) bool { return open[i].seq < open[j].seq })
	for _, st := range open {
		in.closeStream(st)
	}
	in.out.Flush()
}

// system implements system(cmd): flush, run, return the exit status.
func (in *interp) system(cmd string) Value {
	in.flushAll()
	if in.cfg.Run == nil {
		runtimeErrorf("system: command execution is not available")
	}
	st, err := in.cfg.Run(in.ctx, cmd, in.cfg.Stdin, in.outw, in.errw)
	in.checkDone()
	if err != nil {
		in.warnf("%s: %v", cmd, err)
		if st == 0 {
			st = -1
		}
	}
	return num(float64(st))
}

func (in *interp) checkDone() {
	if in.cfg.Done == nil {
		return
	}
	select {
	case <-in.cfg.Done:
		panic(canceled{})
	default:
	}
}

// getline implements all forms; it returns 1, 0 at end of input or -1 on error.
func (in *interp) getline(g *getlineExpr) Value {
	switch {
	case g.src == nil:
		s, ok := in.readMain()
		if !ok {
			return num(0)
		}
		in.deliver(g.lv, s)
		return num(1)
	case g.cmd:
		st := in.input(in.eval(g.src).text(in.convfmt), true)
		return in.getlineFrom(st, g, true)
	}
	name := in.eval(g.src).text(in.convfmt)
	return in.getlineFrom(in.input(name, false), g, false)
}

func (in *interp) getlineFrom(st *stream, g *getlineExpr, countNR bool) Value {
	if st == nil {
		return num(-1)
	}
	s, ok := in.readRecord(st.rd)
	if !ok {
		in.checkDone()
		if st.rd.err != nil {
			return num(-1)
		}
		return num(0)
	}
	if countNR {
		in.vars[vNR] = num(in.vars[vNR].float() + 1)
	}
	in.deliver(g.lv, s)
	return num(1)
}

// deliver stores a record read by getline in its variable, or in $0.
func (in *interp) deliver(lv expr, s string) {
	if lv == nil {
		in.setRecord(s)
		return
	}
	in.store(in.resolve(lv), input(s))
}
