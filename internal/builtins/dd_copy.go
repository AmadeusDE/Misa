// dd's copy loop and its record counters.
//
// The loop is modelled on toybox dd (toys/posix/dd.c, 0BSD): one buffer of
// ibs= bytes plus at most obs=-1 bytes of pending output, short reads counted
// as partial records, and conv=sync padding every short read to a full block.
package builtins

import (
	"fmt"
	"io"
)

// ddStats is the summary dd prints on standard error.
type ddStats struct {
	inFull, inPart   uint64
	outFull, outPart uint64
	bytes            uint64
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
	ibs, obs   int
	unbuffered bool
	sync       bool
	noerror    bool
	fullblock  bool
	remaining  int64 // bytes left to read, or -1
	inName     string
	outName    string
	stderr     io.Writer
	stats      ddStats
}

// run copies until the input ends, count= is satisfied or a read or write
// fails. Memory stays at ibs+obs bytes however large the input is.
func (c *ddCopier) run() error {
	inBuf := make([]byte, c.ibs)
	pending := make([]byte, 0, c.ibs+c.obs)
	stalls := 0

	for c.remaining != 0 {
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
				c.stats.inFull++
			} else {
				c.stats.inPart++
			}
			if c.remaining > 0 {
				c.remaining -= int64(n)
			}
			if c.sync && n < want {
				clear(inBuf[n:want])
				n = want
			}
			if c.unbuffered {
				if err := c.writeRecord(inBuf[:n]); err != nil {
					return err
				}
			} else {
				pending = append(pending, inBuf[:n]...)
				for len(pending) >= c.obs {
					if err := c.writeRecord(pending[:c.obs]); err != nil {
						return err
					}
					copy(pending, pending[c.obs:])
					pending = pending[:len(pending)-c.obs]
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			if !c.readError(err) {
				return &ddIOError{op: "read", name: c.inName, err: err}
			}
		}
	}
	if len(pending) > 0 {
		return c.writeRecord(pending)
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

// writeRecord writes one output block and counts it.
func (c *ddCopier) writeRecord(record []byte) error {
	if len(record) == c.obs {
		c.stats.outFull++
	} else {
		c.stats.outPart++
	}
	for len(record) > 0 {
		written, err := c.out.Write(record)
		if written > 0 {
			c.stats.bytes += uint64(written)
			record = record[written:]
		}
		if err != nil {
			return &ddIOError{op: "write", name: c.outName, err: err}
		}
		if written == 0 {
			return &ddIOError{op: "write", name: c.outName, err: io.ErrShortWrite}
		}
	}
	return nil
}

// readError handles a read failure with conv=noerror: report it, step over the
// bad block and, with conv=sync, write a zero block in its place. It reports
// whether the copy should continue.
func (c *ddCopier) readError(err error) bool {
	if !c.noerror {
		return false
	}
	fmt.Fprintf(c.stderr, "dd: '%s': read error: %s\n", c.inName, fmtOsErr(err))
	if seeker, ok := c.in.(io.Seeker); ok {
		seeker.Seek(int64(c.ibs), io.SeekCurrent)
	}
	if c.sync {
		c.writeRecord(make([]byte, c.ibs))
	}
	return true
}
