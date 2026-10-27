package sshclient

import "io"

// escapeReader handles OpenSSH's escape character on the way to the remote:
// "~." at the start of a line disconnects and "~~" sends one "~".
type escapeReader struct {
	r           io.Reader
	disconnect  func()
	atLineStart bool
	tilde       bool
	buf         []byte
	pending     []byte
	err         error
}

func (e *escapeReader) Read(p []byte) (int, error) {
	for len(e.pending) == 0 {
		if e.err != nil {
			return 0, e.err
		}
		if cap(e.buf) < len(p) {
			e.buf = make([]byte, len(p))
		}
		n, err := e.r.Read(e.buf[:len(p)])
		e.pending = e.filter(e.buf[:n])
		if err != nil && e.err == nil {
			e.err = err
			if e.tilde {
				e.pending = append(e.pending, '~')
			}
		}
	}
	n := copy(p, e.pending)
	e.pending = e.pending[n:]
	return n, nil
}

func (e *escapeReader) filter(in []byte) []byte {
	var out []byte
	for _, b := range in {
		if e.tilde {
			e.tilde = false
			switch b {
			case '.':
				e.err = io.EOF
				e.disconnect()
				return out
			case '~':
				out = append(out, '~')
				e.atLineStart = false
				continue
			default:
				out = append(out, '~')
			}
		} else if e.atLineStart && b == '~' {
			e.tilde = true
			continue
		}
		out = append(out, b)
		e.atLineStart = b == '\r' || b == '\n'
	}
	return out
}
