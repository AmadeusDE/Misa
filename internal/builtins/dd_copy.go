// dd's copy loop and its record counters.
//
// The loop is modelled on toybox dd (toys/posix/dd.c, 0BSD): one buffer of
// ibs= bytes plus at most obs=-1 bytes of pending output, short reads counted
// as partial records, and conv=sync padding every short read to a full block.
// conv=sparse follows uutils coreutils dd (src/uu/dd/src/dd.rs, MIT): an
// all-zero output block becomes a hole in a regular output file instead of a
// write.
package builtins

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
)

// ddStats is the summary dd prints on standard error.
type ddStats struct {
	inFull, inPart   uint64
	outFull, outPart uint64
	bytes            uint64
	truncated        uint64 // conv=block records cut to cbs=
}

// ddIOError names the side of the copy that failed.
type ddIOError struct {
	op   string // "read" or "write"
	name string
	err  error
}

func (e *ddIOError) Error() string { return e.op + " error: " + e.err.Error() }

func (e *ddIOError) Unwrap() error { return e.err }

type ddCopier struct {
	in         io.Reader
	out        io.Writer
	outFile    *os.File // the output file when dd opened a real file
	ibs, obs   int
	unbuffered bool
	sync       bool
	syncPad    byte
	noerror    bool
	fullblock  bool
	sparse     bool
	swab       bool
	aligned    bool // iflag=/oflag=direct: read into an aligned buffer
	directOut  bool // oflag=direct: unaligned writes are refused, not failed
	directBit  int
	transform  *ddTransform
	// stop, when set, is asked between blocks whether the copy should end
	// early. It is the hook cooperative cancellation plugs into.
	stop func() bool

	remaining int64 // bytes left to read, or -1
	inName    string
	outName   string
	stderr    io.Writer

	pending []byte
	mu      sync.Mutex // guards stats for the progress line
	stats   ddStats
}

// snapshot copies the counters for the progress line without tearing them.
func (c *ddCopier) snapshot() ddStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}

// finalStats is snapshot plus the conv=block records that did not fit.
func (c *ddCopier) finalStats() ddStats {
	stats := c.snapshot()
	if c.transform != nil {
		stats.truncated = c.transform.count
	}
	return stats
}

// run copies until the input ends, count= is satisfied or a read or write
// fails. Memory stays at ibs+obs bytes however large the input is.
func (c *ddCopier) run() error {
	var inBuf []byte
	if c.aligned {
		inBuf = ddAlignBytes(c.ibs, 4096)
	} else {
		inBuf = make([]byte, c.ibs)
	}
	c.pending = make([]byte, 0, c.ibs+c.obs)
	stalls := 0

	for c.remaining != 0 {
		if c.stop != nil && c.stop() {
			return errDDInterrupted
		}
		want := c.ibs
		if c.remaining > 0 && int64(want) > c.remaining {
			want = int(c.remaining)
		}
		n, err := c.readBlock(inBuf[:want])
		if n == 0 && err == nil {
			// A reader that hands back nothing at a time; give up rather
			// than spin forever.
			if stalls++; stalls > 64 {
				return nil
			}
			continue
		}
		stalls = 0
		if n > 0 {
			if n == want {
				c.addStats(ddStats{inFull: 1})
			} else {
				c.addStats(ddStats{inPart: 1})
			}
			if c.remaining > 0 {
				c.remaining -= int64(n)
			}
			if c.sync && n < want {
				for i := n; i < want; i++ {
					inBuf[i] = c.syncPad
				}
				n = want
			}
			if c.swab {
				ddSwab(inBuf[:n])
			}
			if err := c.transform.apply(inBuf[:n], c.emit); err != nil {
				return err
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			if rerr := c.readError(err); rerr != nil {
				return rerr
			}
		}
	}
	if err := c.transform.finish(c.emit); err != nil {
		return err
	}
	if len(c.pending) > 0 {
		return c.writeRecord(c.pending)
	}
	return nil
}

// emit hands one piece of converted input to the output: straight out when the
// block sizes pass short reads along, otherwise gathered into obs= records.
func (c *ddCopier) emit(data []byte) error {
	if c.unbuffered {
		return c.writeRecord(data)
	}
	c.pending = append(c.pending, data...)
	for len(c.pending) >= c.obs {
		if err := c.writeRecord(c.pending[:c.obs]); err != nil {
			return err
		}
		c.pending = c.pending[c.obs:]
	}
	return nil
}

// readBlock reads one input block. With iflag=fullblock it keeps reading until
// the block is full or the input ends, so pipes fill whole records.
func (c *ddCopier) readBlock(buf []byte) (int, error) {
	if !c.fullblock {
		return c.in.Read(buf)
	}
	n, err := io.ReadFull(c.in, buf)
	if err == io.ErrUnexpectedEOF {
		err = io.EOF
	}
	return n, err
}

// writeRecord writes one output block and counts it. With conv=sparse an
// all-zero block in a regular output file becomes a hole instead.
func (c *ddCopier) writeRecord(record []byte) error {
	full := len(record) == c.obs
	if c.sparse && c.outFile != nil && ddAllZero(record) && ddSkipBlock(c.outFile, int64(len(record))) {
		c.addStats(ddRecordStats(full, len(record)))
		return nil
	}
	written := 0
	for len(record) > 0 {
		n, err := c.writeOnce(record)
		written += n
		record = record[n:]
		if err != nil {
			return &ddIOError{op: "write", name: c.outName, err: err}
		}
		if n == 0 {
			return &ddIOError{op: "write", name: c.outName, err: io.ErrShortWrite}
		}
	}
	c.addStats(ddRecordStats(full, written))
	return nil
}

// writeOnce writes as much of buf as the output takes. oflag=direct transfers
// must be aligned to the device block size, so a short final block can be
// refused with EINVAL: like the references, dd drops the flag for that write
// and puts it back afterwards instead of failing the copy.
func (c *ddCopier) writeOnce(buf []byte) (int, error) {
	n, err := c.out.Write(buf)
	if err == nil || !c.directOut || c.outFile == nil || !errors.Is(err, syscall.EINVAL) {
		return n, err
	}
	restore, ok := ddClearStreamFlag(c.outFile, c.directBit)
	if !ok {
		return n, err
	}
	n, err = c.out.Write(buf)
	restore()
	return n, err
}

// ddRecordStats counts one output record and the bytes it accounted for.
func ddRecordStats(full bool, bytes int) ddStats {
	if full {
		return ddStats{outFull: 1, bytes: uint64(bytes)}
	}
	return ddStats{outPart: 1, bytes: uint64(bytes)}
}

// addStats adds to the counters under the lock the progress line takes.
func (c *ddCopier) addStats(delta ddStats) {
	c.mu.Lock()
	c.stats.inFull += delta.inFull
	c.stats.inPart += delta.inPart
	c.stats.outFull += delta.outFull
	c.stats.outPart += delta.outPart
	c.stats.bytes += delta.bytes
	c.mu.Unlock()
}

// readError handles a read failure with conv=noerror: report it, step over the
// bad block and, with conv=sync, write a padded block in its place. It returns
// nil to continue the copy and the error that ends it otherwise.
func (c *ddCopier) readError(err error) error {
	if !c.noerror {
		return &ddIOError{op: "read", name: c.inName, err: err}
	}
	c.mu.Lock()
	fmt.Fprintf(c.stderr, "dd: '%s': read error: %s\n", c.inName, fmtOsErr(err))
	c.mu.Unlock()
	if seeker, ok := c.in.(io.Seeker); ok {
		seeker.Seek(int64(c.ibs), io.SeekCurrent)
	}
	if c.sync {
		pad := make([]byte, c.ibs)
		for i := range pad {
			pad[i] = c.syncPad
		}
		return c.emit(pad)
	}
	return nil
}
