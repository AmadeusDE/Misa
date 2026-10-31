// dd's conv= data transformations: the character-set and case tables,
// conv=swab, and the conv=block/unblock record blocking with cbs=.
//
// The blocking rules and the ordering of the transformations follow uutils
// coreutils dd (src/uu/dd/src/blocks.rs and parseargs.rs, MIT) at the commit
// pinned in docs/CREDITS.md: conv=ascii converts before blocking (it implies
// conv=unblock), conv=ebcdic/ibm convert after it, and each conv=block record
// that does not fit in cbs= bytes is counted as truncated.
package builtins

import "bytes"

// ddSwab swaps every pair of bytes in buf, leaving a final odd byte in place.
// It is applied to each input block, as toybox and uutils do for conv=swab.
func ddSwab(buf []byte) {
	for i := 1; i < len(buf); i += 2 {
		buf[i-1], buf[i] = buf[i], buf[i-1]
	}
}

// ddAllZero reports whether buf is a run of zero bytes, the blocks conv=sparse
// turns into holes in the output file.
func ddAllZero(buf []byte) bool {
	for _, b := range buf {
		if b != 0 {
			return false
		}
	}
	return true
}

// ddConvert copies buf through a character table.
func ddConvert(buf []byte, table *[256]byte) []byte {
	out := make([]byte, len(buf))
	for i, b := range buf {
		out[i] = table[b]
	}
	return out
}

// ddCaseTable builds the table for conv=ucase/conv=lcase on its own. Only the
// ASCII letters fold, like the C locale the references assume.
func ddCaseTable(upper bool) *[256]byte {
	table := new([256]byte)
	for i := range table {
		table[i] = byte(i)
	}
	from, to := byte('a'), byte('A')
	if !upper {
		from, to = 'A', 'a'
	}
	for i := 0; i < 26; i++ {
		table[from+byte(i)] = to + byte(i)
	}
	return table
}

var (
	ddUpperTable = ddCaseTable(true)
	ddLowerTable = ddCaseTable(false)
)

// ddUnblock turns cbs= sized records back into newline terminated lines: the
// trailing spaces of each record are dropped and a newline is added.
func ddUnblock(buf []byte, cbs int) []byte {
	out := make([]byte, 0, len(buf)+len(buf)/cbs+1)
	for len(buf) > 0 {
		chunk := buf[:min(cbs, len(buf))]
		buf = buf[len(chunk):]
		for len(chunk) > 0 && chunk[len(chunk)-1] == ' ' {
			chunk = chunk[:len(chunk)-1]
		}
		out = append(out, chunk...)
		out = append(out, '\n')
	}
	return out
}

// ddTransform applies conv= to the copied blocks. A nil *ddTransform passes
// its input through unchanged.
type ddTransform struct {
	table        *[256]byte // character conversion, or nil
	block        bool       // conv=block: pad records to cbs=
	unblock      bool       // conv=unblock: cut records into lines
	convertFirst bool       // conv=ascii converts before blocking
	cbs          int

	recordLen int    // bytes of the current block record so far
	truncated bool   // the current record was already counted as truncated
	count     uint64 // records truncated over the whole copy
	out       []byte // blocked output not yet handed over
}

// newDDTransform builds the transformation conv= asked for, or nil when the
// copy is a plain byte copy.
func newDDTransform(cfg *ddConfig) *ddTransform {
	c := &cfg.conv
	if c.table == nil && !c.block && !c.unblock {
		return nil
	}
	t := &ddTransform{
		table:        c.table,
		block:        c.block,
		unblock:      c.unblock,
		convertFirst: c.ascii,
		cbs:          cfg.cbs,
	}
	// conv=ascii converts from EBCDIC before the blocking step so that the
	// EBCDIC space is turned into an ASCII space first; the other character
	// sets come after it, as the references order them.
	return t
}

// apply transforms one input block and hands the result to emit.
func (t *ddTransform) apply(buf []byte, emit func([]byte) error) error {
	if t == nil {
		return emit(buf)
	}
	table := t.table
	if table != nil && (t.convertFirst || (!t.block && !t.unblock)) {
		buf = ddConvert(buf, table)
		table = nil
	}
	if t.unblock {
		out := ddUnblock(buf, t.cbs)
		if table != nil {
			out = ddConvert(out, table)
		}
		return emit(out)
	}
	if t.block {
		return t.blockBytes(buf, table, emit)
	}
	return emit(buf)
}

// finish pads and writes the last conv=block record, which the input may have
// ended in the middle of.
func (t *ddTransform) finish(emit func([]byte) error) error {
	if t == nil || !t.block {
		return nil
	}
	if t.recordLen > 0 {
		t.endRecord(t.table)
	}
	return t.flush(emit)
}

// blockBytes splits buf on newlines and pads every record to cbs= bytes, as
// conv=block does. table converts the record and the padding afterwards when
// the conversion runs after the blocking step (conv=ebcdic/ibm).
func (t *ddTransform) blockBytes(buf []byte, table *[256]byte, emit func([]byte) error) error {
	for {
		record := buf
		end := bytes.IndexByte(buf, '\n')
		if end >= 0 {
			record, buf = buf[:end], buf[end+1:]
		}
		n := min(len(record), t.cbs-t.recordLen)
		if table == nil {
			t.out = append(t.out, record[:n]...)
		} else {
			t.out = append(t.out, ddConvert(record[:n], table)...)
		}
		t.recordLen += n
		if n < len(record) && !t.truncated {
			t.count++
			t.truncated = true
		}
		if end < 0 {
			break
		}
		t.endRecord(table)
	}
	return t.flush(emit)
}

// endRecord pads the current record to cbs= bytes and starts a new one.
func (t *ddTransform) endRecord(table *[256]byte) {
	pad := byte(' ')
	if table != nil {
		pad = table[' ']
	}
	for i := t.cbs - t.recordLen; i > 0; i-- {
		t.out = append(t.out, pad)
	}
	t.recordLen, t.truncated = 0, false
}

// flush hands the bytes blocked so far to emit.
func (t *ddTransform) flush(emit func([]byte) error) error {
	if len(t.out) == 0 {
		return nil
	}
	if err := emit(t.out); err != nil {
		return err
	}
	t.out = t.out[:0]
	return nil
}
