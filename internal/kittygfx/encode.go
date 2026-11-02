package kittygfx

import (
	"fmt"
	"strconv"
)

const (
	apcStart = "\x1b_G"
	st       = "\x1b\\"
)

// Encode emits exactly one graphics APC from an already base64 encoded payload.
// Only non-zero fields are written, in the order a t f i I p s v c r x y w h
// X Y C z q m U d, followed by ";" and the payload when it is not empty.
func Encode(cmd Command, payload []byte) ([]byte, error) {
	if err := cmd.encodable(); err != nil {
		return nil, err
	}
	if !validBase64(payload) {
		return nil, fmt.Errorf("%w: payload is not base64", ErrMalformed)
	}
	out := make([]byte, 0, len(apcStart)+len(payload)+64)
	out = append(out, apcStart...)
	first := true
	add := func(key byte, value string) {
		if value == "" {
			return
		}
		if !first {
			out = append(out, ',')
		}
		first = false
		out = append(out, key, '=')
		out = append(out, value...)
	}
	add('a', byteValue(cmd.Action))
	add('t', byteValue(cmd.Transmission))
	add('f', intValue(cmd.Format, false))
	add('i', uintValue(cmd.ID))
	add('I', uintValue(cmd.Number))
	add('p', uintValue(cmd.PlacementID))
	for _, field := range [...]struct {
		key     byte
		value   int
		present bool
	}{
		{'s', cmd.Width, false}, {'v', cmd.Height, false},
		{'c', cmd.Columns, false}, {'r', cmd.Rows, false},
		{'x', cmd.SourceX, false}, {'y', cmd.SourceY, false},
		{'w', cmd.SourceWidth, false}, {'h', cmd.SourceHeight, false},
		{'X', cmd.OffsetX, false}, {'Y', cmd.OffsetY, false},
		{'C', cmd.CursorMovement, cmd.Has('C')}, {'z', cmd.Z, false},
		{'q', cmd.Quiet, cmd.Has('q')}, {'m', boolValue(cmd.More), cmd.Has('m')},
		{'U', cmd.Unicode, false},
	} {
		add(field.key, intValue(field.value, field.present))
	}
	add('d', byteValue(cmd.Delete))
	if len(payload) > 0 {
		out = append(out, ';')
		out = append(out, payload...)
	}
	return append(out, st...), nil
}

func (c Command) encodable() error {
	switch c.Action {
	case ActionTransmit, ActionTransmitPlace, ActionPlace, ActionDelete, ActionQuery:
	case 0:
		return fmt.Errorf("%w: missing action", ErrMalformed)
	default:
		return fmt.Errorf("%w: action %q", ErrUnsupported, c.Action)
	}
	if c.Unicode != 0 {
		return fmt.Errorf("%w: unicode placeholders", ErrUnsupported)
	}
	if c.Format != 0 && c.Format != 24 && c.Format != 32 {
		return fmt.Errorf("%w: format %d", ErrUnsupported, c.Format)
	}
	if c.Transmission != 0 {
		if c.Transmission != 'd' {
			return fmt.Errorf("%w: transmission medium %q", ErrUnsupported, c.Transmission)
		}
		if c.Action != ActionTransmit && c.Action != ActionTransmitPlace && c.Action != ActionQuery {
			return fmt.Errorf("%w: transmission medium without a transfer", ErrMalformed)
		}
	}
	for _, v := range [...]int{c.Width, c.Height, c.Columns, c.Rows,
		c.SourceX, c.SourceY, c.SourceWidth, c.SourceHeight, c.OffsetX, c.OffsetY} {
		if v < 0 {
			return fmt.Errorf("%w: negative size or offset", ErrMalformed)
		}
	}
	if c.Quiet < 0 || c.Quiet > 2 {
		return fmt.Errorf("%w: q=%d", ErrMalformed, c.Quiet)
	}
	if c.CursorMovement < 0 || c.CursorMovement > 1 {
		return fmt.Errorf("%w: C=%d", ErrMalformed, c.CursorMovement)
	}
	if c.Delete != 0 {
		if c.Action != ActionDelete {
			return fmt.Errorf("%w: delete specifier without a=d", ErrMalformed)
		}
		switch c.Delete {
		case 'a', 'A', 'i', 'I', 'n', 'N':
		default:
			return fmt.Errorf("%w: delete specifier %q", ErrUnsupported, c.Delete)
		}
	}
	return nil
}

func byteValue(v byte) string {
	if v == 0 {
		return ""
	}
	return string(v)
}

func intValue(v int, present bool) string {
	if v == 0 && !present {
		return ""
	}
	return strconv.Itoa(v)
}

func uintValue(v uint32) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatUint(uint64(v), 10)
}

func boolValue(v bool) int {
	if v {
		return 1
	}
	return 0
}
