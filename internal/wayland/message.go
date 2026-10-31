//go:build unix

// Package wayland is a minimal pure-Go client for the Wayland wire protocol:
// a socket, object ids, and the argument encoding. Interfaces are plain
// opcodes at the call sites, as in the protocol XML.
package wayland

import (
	"encoding/binary"
	"errors"
)

// Fixed is wl_fixed_t, a signed 24.8 fixed-point number.
type Fixed int32

func (f Fixed) Float() float64 { return float64(f) / 256 }

// FD is a file descriptor argument, passed out of band.
type FD int

var errShort = errors.New("wayland: truncated message")

const headerSize = 8

// encode builds one request. Arguments are uint32 (uint, object, new_id),
// int32 (int), Fixed, string, []byte (array) or FD; FDs are returned
// separately for the control message.
func encode(obj uint32, op uint16, args []any) ([]byte, []int) {
	b := make([]byte, headerSize, 64)
	var fds []int
	for _, a := range args {
		switch v := a.(type) {
		case uint32:
			b = binary.LittleEndian.AppendUint32(b, v)
		case int32:
			b = binary.LittleEndian.AppendUint32(b, uint32(v))
		case Fixed:
			b = binary.LittleEndian.AppendUint32(b, uint32(v))
		case string:
			b = appendBytes(b, append([]byte(v), 0))
		case []byte:
			b = appendBytes(b, v)
		case FD:
			fds = append(fds, int(v))
		default:
			panic("wayland: unsupported argument type")
		}
	}
	binary.LittleEndian.PutUint32(b, obj)
	binary.LittleEndian.PutUint32(b[4:], uint32(len(b))<<16|uint32(op))
	return b, fds
}

func appendBytes(b, v []byte) []byte {
	b = binary.LittleEndian.AppendUint32(b, uint32(len(v)))
	b = append(b, v...)
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	return b
}

// Event is one message from the compositor. Its getters consume arguments in
// order; a malformed message yields zero values and sets Err.
type Event struct {
	Obj  uint32
	Op   uint16
	data []byte
	fds  func() (int, bool)
	Err  error
}

// splitMessage returns the first complete message in buf and its length, or
// ok false when more bytes are needed.
func splitMessage(buf []byte) (ev Event, n int, ok bool) {
	if len(buf) < headerSize {
		return Event{}, 0, false
	}
	word := binary.LittleEndian.Uint32(buf[4:])
	size := int(word >> 16)
	if size < headerSize {
		size = headerSize
	}
	if len(buf) < size {
		return Event{}, 0, false
	}
	ev = Event{
		Obj:  binary.LittleEndian.Uint32(buf),
		Op:   uint16(word),
		data: append([]byte(nil), buf[headerSize:size]...),
	}
	return ev, size, true
}

func (e *Event) word() uint32 {
	if len(e.data) < 4 {
		e.Err = errShort
		return 0
	}
	v := binary.LittleEndian.Uint32(e.data)
	e.data = e.data[4:]
	return v
}

func (e *Event) Uint() uint32   { return e.word() }
func (e *Event) Int() int32     { return int32(e.word()) }
func (e *Event) Fixed() Fixed   { return Fixed(e.word()) }
func (e *Event) Object() uint32 { return e.word() }

func (e *Event) Array() []byte {
	n := int(e.word())
	padded := (n + 3) &^ 3
	if n < 0 || padded > len(e.data) {
		e.Err = errShort
		return nil
	}
	v := e.data[:n]
	e.data = e.data[padded:]
	return v
}

func (e *Event) Text() string {
	b := e.Array()
	if len(b) > 0 && b[len(b)-1] == 0 {
		b = b[:len(b)-1]
	}
	return string(b)
}

// FD takes the next received file descriptor, or -1 if none arrived.
func (e *Event) FD() int {
	if e.fds == nil {
		return -1
	}
	if fd, ok := e.fds(); ok {
		return fd
	}
	e.Err = errors.New("wayland: missing file descriptor")
	return -1
}
