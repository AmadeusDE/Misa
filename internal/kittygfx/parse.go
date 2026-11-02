package kittygfx

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

// Actions of a graphics command.
const (
	ActionTransmit      = 't'
	ActionTransmitPlace = 'T'
	ActionPlace         = 'p'
	ActionDelete        = 'd'
	ActionQuery         = 'q'
)

var (
	ErrMalformed   = errors.New("kittygfx: malformed command")
	ErrUnsupported = errors.New("kittygfx: unsupported command")
	ErrTooLarge    = errors.New("kittygfx: limit exceeded")
)

// Command is one graphics APC body: no leading 'G', no ST terminator.
// Parse fills in the protocol defaults a=t and, for a transfer or query, f=32.
type Command struct {
	Action         byte   // 'a'
	Transmission   byte   // 't'
	Delete         byte   // 'd'
	Format         int    // 'f'
	Width          int    // 's'
	Height         int    // 'v'
	ID             uint32 // 'i'
	Number         uint32 // 'I'
	PlacementID    uint32 // 'p'
	Columns        int    // 'c'
	Rows           int    // 'r'
	SourceX        int    // 'x'
	SourceY        int    // 'y'
	SourceWidth    int    // 'w'
	SourceHeight   int    // 'h'
	OffsetX        int    // 'X'
	OffsetY        int    // 'Y'
	CursorMovement int    // 'C'
	Z              int    // 'z'
	Quiet          int    // 'q'
	More           bool   // 'm'
	Unicode        int    // 'U'
	Payload        []byte // base64 payload as received, still encoded

	present uint64
}

// Has reports whether the single-letter key was present in the parsed APC, so
// m=0 and C=0 stay distinguishable from an absent key.
func (c Command) Has(key byte) bool { return c.present&(1<<(key&63)) != 0 }

// Parse decodes one graphics APC body. On error it still returns the fields
// decoded so far — at least i, I, p and q — so the caller can honour the quiet
// level and address the error reply.
func Parse(body []byte, lim Limits) (Command, error) {
	lim = lim.Normalize()
	var cmd Command
	var firstErr error
	control, payload := body, []byte(nil)
	if i := bytes.IndexByte(body, ';'); i >= 0 {
		control, payload = body[:i], body[i+1:]
	}
	switch {
	case len(payload) > lim.MaxPayload:
		firstErr = fmt.Errorf("%w: payload of %d bytes exceeds %d", ErrTooLarge, len(payload), lim.MaxPayload)
	case !validBase64(payload):
		firstErr = fmt.Errorf("%w: payload is not base64", ErrMalformed)
	default:
		cmd.Payload = append([]byte(nil), payload...)
	}
	for {
		item := control
		if i := bytes.IndexByte(control, ','); i >= 0 {
			item, control = control[:i], control[i+1:]
		} else {
			control = nil
		}
		if err := cmd.set(item, lim); err != nil && firstErr == nil {
			firstErr = err
		}
		if control == nil {
			break
		}
	}
	if firstErr != nil {
		return cmd, firstErr
	}
	return cmd, cmd.applyDefaults()
}

func (c *Command) set(item []byte, lim Limits) error {
	if len(item) < 2 || item[1] != '=' {
		return fmt.Errorf("%w: %q is not key=value", ErrMalformed, item)
	}
	key, value := item[0], item[2:]
	if !isLetter(key) {
		return fmt.Errorf("%w: %q is not a key", ErrMalformed, key)
	}
	if len(value) == 0 {
		return fmt.Errorf("%w: key %q has an empty value", ErrMalformed, key)
	}
	if c.present&(1<<(key&63)) != 0 {
		return fmt.Errorf("%w: duplicate key %q", ErrMalformed, key)
	}
	if err := c.assign(key, value, lim); err != nil {
		return err
	}
	c.present |= 1 << (key & 63)
	return nil
}

func (c *Command) assign(key byte, value []byte, lim Limits) error {
	switch key {
	case 'a':
		if len(value) != 1 {
			return fmt.Errorf("%w: action %q", ErrMalformed, value)
		}
		switch value[0] {
		case ActionTransmit, ActionTransmitPlace, ActionPlace, ActionDelete, ActionQuery:
			c.Action = value[0]
			return nil
		}
		return fmt.Errorf("%w: action %q", ErrUnsupported, value)
	case 't':
		if len(value) != 1 || value[0] != 'd' {
			return fmt.Errorf("%w: transmission medium %q", ErrUnsupported, value)
		}
		c.Transmission = 'd'
		return nil
	case 'd':
		if len(value) != 1 {
			return fmt.Errorf("%w: delete specifier %q", ErrMalformed, value)
		}
		switch value[0] {
		case 'a', 'A', 'i', 'I', 'n', 'N':
			c.Delete = value[0]
			return nil
		}
		return fmt.Errorf("%w: delete specifier %q", ErrUnsupported, value)
	case 'f':
		n, err := parseRange(value, 0, 1000)
		if err != nil {
			return err
		}
		switch n {
		case 24, 32:
			c.Format = n
			return nil
		case 100:
			return fmt.Errorf("%w: PNG payloads", ErrUnsupported)
		}
		return fmt.Errorf("%w: format %d", ErrMalformed, n)
	case 'o':
		return fmt.Errorf("%w: compression", ErrUnsupported)
	case 'i', 'I', 'p':
		n, err := strconv.ParseUint(string(value), 10, 32)
		if err != nil {
			return fmt.Errorf("%w: key %q value %q", ErrMalformed, key, value)
		}
		switch key {
		case 'i':
			c.ID = uint32(n)
		case 'I':
			c.Number = uint32(n)
		default:
			c.PlacementID = uint32(n)
		}
		return nil
	case 'm', 'q', 'C':
		n, err := parseRange(value, 0, 2)
		if err != nil {
			return err
		}
		switch key {
		case 'm':
			if n > 1 {
				return fmt.Errorf("%w: m=%d", ErrMalformed, n)
			}
			c.More = n == 1
		case 'q':
			c.Quiet = n
		default:
			if n > 1 {
				return fmt.Errorf("%w: C=%d", ErrMalformed, n)
			}
			c.CursorMovement = n
		}
		return nil
	case 'U':
		n, err := parseInt(value)
		if err != nil {
			return err
		}
		if n != 0 {
			return fmt.Errorf("%w: unicode placeholders", ErrUnsupported)
		}
		c.Unicode = 0
		return nil
	case 'z':
		n, err := parseInt(value)
		if err != nil {
			return err
		}
		c.Z = n
		return nil
	}
	field := c.intField(key)
	if field == nil {
		return fmt.Errorf("%w: key %q", ErrUnsupported, key)
	}
	n, err := parseInt(value)
	if err != nil {
		return err
	}
	if n < 0 {
		return fmt.Errorf("%w: key %q value %d", ErrMalformed, key, n)
	}
	*field = n
	if key == 's' && n > lim.MaxWidth {
		return fmt.Errorf("%w: s=%d exceeds %d", ErrTooLarge, n, lim.MaxWidth)
	}
	if key == 'v' && n > lim.MaxHeight {
		return fmt.Errorf("%w: v=%d exceeds %d", ErrTooLarge, n, lim.MaxHeight)
	}
	return nil
}

func (c *Command) intField(key byte) *int {
	switch key {
	case 's':
		return &c.Width
	case 'v':
		return &c.Height
	case 'c':
		return &c.Columns
	case 'r':
		return &c.Rows
	case 'x':
		return &c.SourceX
	case 'y':
		return &c.SourceY
	case 'w':
		return &c.SourceWidth
	case 'h':
		return &c.SourceHeight
	case 'X':
		return &c.OffsetX
	case 'Y':
		return &c.OffsetY
	}
	return nil
}

func (c *Command) applyDefaults() error {
	if c.Action == 0 {
		c.Action = ActionTransmit
	}
	transfer := c.Action == ActionTransmit || c.Action == ActionTransmitPlace || c.Action == ActionQuery
	if transfer && c.Format == 0 {
		c.Format = 32
	}
	if c.Delete != 0 && c.Action != ActionDelete {
		return fmt.Errorf("%w: delete specifier without a=d", ErrMalformed)
	}
	if c.Transmission != 0 && !transfer {
		return fmt.Errorf("%w: transmission medium without a transfer", ErrMalformed)
	}
	return nil
}

func parseInt(value []byte) (int, error) {
	n, err := strconv.Atoi(string(value))
	if err != nil {
		return 0, fmt.Errorf("%w: %q is not a number", ErrMalformed, value)
	}
	return n, nil
}

func parseRange(value []byte, lo, hi int) (int, error) {
	n, err := parseInt(value)
	if err != nil {
		return 0, err
	}
	if n < lo || n > hi {
		return 0, fmt.Errorf("%w: %q is out of range", ErrMalformed, value)
	}
	return n, nil
}

func isLetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }

func validBase64(payload []byte) bool {
	padding := 0
	for _, b := range payload {
		switch {
		case b >= 'A' && b <= 'Z', b >= 'a' && b <= 'z', b >= '0' && b <= '9', b == '+', b == '/':
			if padding > 0 {
				return false
			}
		case b == '=':
			padding++
			if padding > 2 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
