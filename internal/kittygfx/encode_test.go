package kittygfx

import (
	"bytes"
	"errors"
	"testing"
)

func sameFields(a, b Command) bool {
	return a.Action == b.Action &&
		a.Transmission == b.Transmission &&
		a.Delete == b.Delete &&
		a.Format == b.Format &&
		a.Width == b.Width &&
		a.Height == b.Height &&
		a.ID == b.ID &&
		a.Number == b.Number &&
		a.PlacementID == b.PlacementID &&
		a.Columns == b.Columns &&
		a.Rows == b.Rows &&
		a.SourceX == b.SourceX &&
		a.SourceY == b.SourceY &&
		a.SourceWidth == b.SourceWidth &&
		a.SourceHeight == b.SourceHeight &&
		a.OffsetX == b.OffsetX &&
		a.OffsetY == b.OffsetY &&
		a.CursorMovement == b.CursorMovement &&
		a.Z == b.Z &&
		a.Quiet == b.Quiet &&
		a.More == b.More &&
		a.Unicode == b.Unicode
}

func sameCommand(a, b Command) bool {
	return sameFields(a, b) && bytes.Equal(a.Payload, b.Payload)
}

func TestEncodeTransmit(t *testing.T) {
	got, err := Encode(Command{Action: ActionTransmit, Format: 24, Width: 2, Height: 2, ID: 7}, []byte("AAAA"))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	want := "\x1b_Ga=t,f=24,i=7,s=2,v=2;AAAA\x1b\\"
	if string(got) != want {
		t.Fatalf("Encode = %q, want %q", got, want)
	}
}

func TestEncodeKeyOrder(t *testing.T) {
	cmd := Command{
		Action: ActionTransmit, Transmission: 'd', Format: 24, ID: 1, Number: 2, PlacementID: 3,
		Width: 4, Height: 5, Columns: 6, Rows: 7, SourceX: 8, SourceY: 9, SourceWidth: 10,
		SourceHeight: 11, OffsetX: 12, OffsetY: 13, CursorMovement: 1, Z: -1, Quiet: 1, More: true,
	}
	got, err := Encode(cmd, []byte("AAAA"))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	want := "\x1b_Ga=t,t=d,f=24,i=1,I=2,p=3,s=4,v=5,c=6,r=7,x=8,y=9,w=10,h=11,X=12,Y=13,C=1,z=-1,q=1,m=1;AAAA\x1b\\"
	if string(got) != want {
		t.Fatalf("Encode = %q, want %q", got, want)
	}
}

func TestEncodeOmittedFields(t *testing.T) {
	tests := []struct {
		name string
		cmd  Command
		want string
	}{
		{"delete", Command{Action: ActionDelete, Delete: 'a'}, "\x1b_Ga=d,d=a\x1b\\"},
		{"query", Command{Action: ActionQuery, ID: 5, Format: 24, Width: 1, Height: 1}, "\x1b_Ga=q,f=24,i=5,s=1,v=1\x1b\\"},
		{"transmit and place", Command{Action: ActionTransmitPlace, Format: 32, Width: 1, Height: 1, ID: 1, PlacementID: 1}, "\x1b_Ga=T,f=32,i=1,p=1,s=1,v=1\x1b\\"},
		{"place", Command{Action: ActionPlace, ID: 1, PlacementID: 2, Columns: 3, Rows: 4}, "\x1b_Ga=p,i=1,p=2,c=3,r=4\x1b\\"},
		{"empty payload", Command{Action: ActionTransmit}, "\x1b_Ga=t\x1b\\"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Encode(tt.cmd, nil)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("Encode = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEncodePreservesKeyPresence(t *testing.T) {
	tests := []struct {
		body string
		want string
	}{
		{"a=t,m=0", "\x1b_Ga=t,f=32,m=0\x1b\\"},
		{"a=t,q=0", "\x1b_Ga=t,f=32,q=0\x1b\\"},
		{"a=t,C=0", "\x1b_Ga=t,f=32,C=0\x1b\\"},
		{"a=t,m=1,q=2,C=1", "\x1b_Ga=t,f=32,C=1,q=2,m=1\x1b\\"},
	}
	for _, tt := range tests {
		cmd, err := Parse([]byte(tt.body), DefaultLimits())
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.body, err)
		}
		wire, err := Encode(cmd, cmd.Payload)
		if err != nil {
			t.Fatalf("Encode(%q): %v", tt.body, err)
		}
		if string(wire) != tt.want {
			t.Fatalf("Encode(Parse(%q)) = %q, want %q", tt.body, wire, tt.want)
		}
		back, err := Parse(wire[3:len(wire)-2], DefaultLimits())
		if err != nil {
			t.Fatalf("Parse(%q): %v", wire, err)
		}
		if !sameCommand(cmd, back) || back.Has('m') != cmd.Has('m') || back.Has('q') != cmd.Has('q') || back.Has('C') != cmd.Has('C') {
			t.Fatalf("presence lost: %+v -> %+v", cmd, back)
		}
	}
}

func TestEncodeConstructedContinuation(t *testing.T) {
	wire, err := Encode(Command{Action: ActionTransmit, Format: 24, Width: 1, Height: 1, ID: 4, More: true}, []byte("AAAA"))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if want := "\x1b_Ga=t,f=24,i=4,s=1,v=1,m=1;AAAA\x1b\\"; string(wire) != want {
		t.Fatalf("Encode = %q, want %q", wire, want)
	}
	wire, err = Encode(Command{Action: ActionTransmit, Format: 24, Width: 1, Height: 1, ID: 4}, []byte("AAAA"))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if want := "\x1b_Ga=t,f=24,i=4,s=1,v=1;AAAA\x1b\\"; string(wire) != want {
		t.Fatalf("last chunk = %q, want %q", wire, want)
	}
}

func TestEncodeQueryTransmission(t *testing.T) {
	wire, err := Encode(Command{Action: ActionQuery, Transmission: 'd', Format: 32, Width: 1, Height: 1, ID: 1}, []byte("AAAA"))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if want := "\x1b_Ga=q,t=d,f=32,i=1,s=1,v=1;AAAA\x1b\\"; string(wire) != want {
		t.Fatalf("Encode = %q, want %q", wire, want)
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	cmds := []struct {
		cmd     Command
		payload []byte
	}{
		{Command{Action: ActionTransmit, Transmission: 'd', Format: 32, Width: 3, Height: 4, ID: 9}, []byte("AAAAAA==")},
		{Command{Action: ActionTransmitPlace, Format: 24, Width: 1, Height: 1, ID: 1, PlacementID: 2, Columns: 3, Rows: 4}, []byte("AAAA")},
		{Command{Action: ActionPlace, ID: 7, PlacementID: 8, SourceX: 1, SourceY: 2, SourceWidth: 3, SourceHeight: 4, OffsetX: 5, OffsetY: 6, CursorMovement: 1, Z: 2, Quiet: 1}, nil},
		{Command{Action: ActionDelete, Delete: 'N'}, nil},
		{Command{Action: ActionQuery, ID: 4, Format: 24, Width: 1, Height: 1}, []byte("AAAA")},
		{Command{Action: ActionTransmit, Format: 24, Width: 2, Height: 2, ID: 1, More: true, Quiet: 2}, []byte("AAAAAAAA")},
	}
	for _, tt := range cmds {
		wire, err := Encode(tt.cmd, tt.payload)
		if err != nil {
			t.Fatalf("Encode(%+v): %v", tt.cmd, err)
		}
		if len(wire) < 4 || string(wire[:3]) != "\x1b_G" || string(wire[len(wire)-2:]) != "\x1b\\" {
			t.Fatalf("not one APC: %q", wire)
		}
		back, err := Parse(wire[3:len(wire)-2], DefaultLimits())
		if err != nil {
			t.Fatalf("Parse(%q): %v", wire, err)
		}
		want := tt.cmd
		want.Payload = tt.payload
		if !sameCommand(back, want) {
			t.Fatalf("round trip = %+v, want %+v", back, want)
		}
	}
}

func TestEncodeRejects(t *testing.T) {
	tests := []struct {
		name    string
		cmd     Command
		payload []byte
		want    error
	}{
		{"missing action", Command{}, nil, ErrMalformed},
		{"unknown action", Command{Action: 'x'}, nil, ErrUnsupported},
		{"frame action", Command{Action: 'f'}, nil, ErrUnsupported},
		{"unicode", Command{Action: ActionTransmit, Unicode: 1}, nil, ErrUnsupported},
		{"format", Command{Action: ActionTransmit, Format: 16}, nil, ErrUnsupported},
		{"png format", Command{Action: ActionTransmit, Format: 100}, nil, ErrUnsupported},
		{"transmission", Command{Action: ActionTransmit, Transmission: 'f'}, nil, ErrUnsupported},
		{"delete specifier", Command{Action: ActionDelete, Delete: 'z'}, nil, ErrUnsupported},
		{"delete placement", Command{Action: ActionDelete, Delete: 'p'}, nil, ErrUnsupported},
		{"delete placement upper", Command{Action: ActionDelete, Delete: 'P'}, nil, ErrUnsupported},
		{"delete without action", Command{Action: ActionPlace, Delete: 'a'}, nil, ErrMalformed},
		{"quiet range", Command{Action: ActionTransmit, Quiet: 3}, nil, ErrMalformed},
		{"cursor range", Command{Action: ActionTransmit, CursorMovement: 2}, nil, ErrMalformed},
		{"payload", Command{Action: ActionTransmit}, []byte("AA*A"), ErrMalformed},
		{"negative width", Command{Action: ActionTransmit, Width: -1}, nil, ErrMalformed},
		{"negative height", Command{Action: ActionTransmit, Height: -1}, nil, ErrMalformed},
		{"negative columns", Command{Action: ActionPlace, Columns: -1}, nil, ErrMalformed},
		{"negative rows", Command{Action: ActionPlace, Rows: -1}, nil, ErrMalformed},
		{"negative source x", Command{Action: ActionPlace, SourceX: -1}, nil, ErrMalformed},
		{"negative source y", Command{Action: ActionPlace, SourceY: -1}, nil, ErrMalformed},
		{"negative source width", Command{Action: ActionPlace, SourceWidth: -1}, nil, ErrMalformed},
		{"negative source height", Command{Action: ActionPlace, SourceHeight: -1}, nil, ErrMalformed},
		{"negative offset x", Command{Action: ActionPlace, OffsetX: -1}, nil, ErrMalformed},
		{"negative offset y", Command{Action: ActionPlace, OffsetY: -1}, nil, ErrMalformed},
		{"transmission on place", Command{Action: ActionPlace, Transmission: 'd'}, nil, ErrMalformed},
		{"transmission on delete", Command{Action: ActionDelete, Delete: 'a', Transmission: 'd'}, nil, ErrMalformed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Encode(tt.cmd, tt.payload)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Encode(%+v) = %v, want %v", tt.cmd, err, tt.want)
			}
		})
	}
}
