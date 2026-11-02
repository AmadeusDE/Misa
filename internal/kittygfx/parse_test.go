package kittygfx

import (
	"errors"
	"strings"
	"testing"
)

func TestParseTransmit(t *testing.T) {
	body := "a=t,t=d,f=24,s=2,v=2,i=7,I=8,p=3,c=4,r=5,x=1,y=2,w=2,h=2,X=6,Y=7,C=1,z=-2,q=1,m=1,U=0;AAAAAA=="
	cmd, err := Parse([]byte(body), DefaultLimits())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := Command{
		Action: ActionTransmit, Transmission: 'd', Format: 24, Width: 2, Height: 2,
		ID: 7, Number: 8, PlacementID: 3, Columns: 4, Rows: 5,
		SourceX: 1, SourceY: 2, SourceWidth: 2, SourceHeight: 2,
		OffsetX: 6, OffsetY: 7, CursorMovement: 1, Z: -2, Quiet: 1, More: true,
	}
	if !sameFields(cmd, want) {
		t.Fatalf("Parse = %+v, want %+v", cmd, want)
	}
	if string(cmd.Payload) != "AAAAAA==" {
		t.Fatalf("payload = %q", cmd.Payload)
	}
	for _, key := range []byte{'a', 't', 'f', 's', 'v', 'i', 'I', 'p', 'c', 'r', 'x', 'y', 'w', 'h', 'X', 'Y', 'C', 'z', 'q', 'm', 'U'} {
		if !cmd.Has(key) {
			t.Errorf("Has(%q) = false, want true", key)
		}
	}
	for _, key := range []byte{'o', 'd'} {
		if cmd.Has(key) {
			t.Errorf("Has(%q) = true, want false", key)
		}
	}
}

func TestParseDefaults(t *testing.T) {
	cmd, err := Parse([]byte("i=3;AAAA"), DefaultLimits())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cmd.Action != ActionTransmit {
		t.Errorf("Action = %q, want %q", cmd.Action, ActionTransmit)
	}
	if cmd.Format != 32 {
		t.Errorf("Format = %d, want the protocol default 32", cmd.Format)
	}
	if cmd.Transmission != 0 || cmd.Quiet != 0 || cmd.More || cmd.Width != 0 {
		t.Errorf("unexpected defaults: %+v", cmd)
	}
	if string(cmd.Payload) != "AAAA" {
		t.Errorf("payload = %q", cmd.Payload)
	}
	if cmd.Has('a') || cmd.Has('f') || cmd.Has('q') {
		t.Error("Has reports keys that were absent")
	}
}

func TestParseTransmitAndPlace(t *testing.T) {
	cmd, err := Parse([]byte("a=T,f=32,s=1,v=1,i=1,p=2;AAAA"), DefaultLimits())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cmd.Action != ActionTransmitPlace || cmd.Format != 32 || cmd.ID != 1 || cmd.PlacementID != 2 {
		t.Fatalf("Parse = %+v", cmd)
	}
}

func TestParsePlace(t *testing.T) {
	cmd, err := Parse([]byte("a=p,i=3,p=1,c=2,r=2,X=1,Y=2"), DefaultLimits())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cmd.Action != ActionPlace || cmd.ID != 3 || cmd.PlacementID != 1 || cmd.Columns != 2 || cmd.Rows != 2 || cmd.OffsetX != 1 || cmd.OffsetY != 2 {
		t.Fatalf("Parse = %+v", cmd)
	}
	if cmd.Format != 0 || cmd.Width != 0 || len(cmd.Payload) != 0 {
		t.Fatalf("place picked up transmit fields: %+v", cmd)
	}
}

func TestParseDelete(t *testing.T) {
	for _, spec := range []byte{'a', 'A', 'i', 'I', 'n', 'N'} {
		cmd, err := Parse([]byte("a=d,d="+string(spec)), DefaultLimits())
		if err != nil {
			t.Fatalf("d=%c: %v", spec, err)
		}
		if cmd.Action != ActionDelete || cmd.Delete != spec || !cmd.Has('d') {
			t.Fatalf("d=%c: Parse = %+v", spec, cmd)
		}
	}
}

func TestParseQuery(t *testing.T) {
	cmd, err := Parse([]byte("a=q,i=1,s=1,v=1,f=24;AAAA"), DefaultLimits())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cmd.Action != ActionQuery || cmd.ID != 1 || cmd.Width != 1 || cmd.Height != 1 || cmd.Format != 24 {
		t.Fatalf("Parse = %+v", cmd)
	}
}

func TestParseRejects(t *testing.T) {
	tests := []struct {
		name string
		body string
		want error
	}{
		{"empty", "", ErrMalformed},
		{"no value", "a", ErrMalformed},
		{"empty value", "a=", ErrMalformed},
		{"trailing comma", "a=t,", ErrMalformed},
		{"non letter key", "1=2", ErrMalformed},
		{"unknown key", "a=t,S=1", ErrUnsupported},
		{"action frame", "a=f", ErrUnsupported},
		{"action animation", "a=a", ErrUnsupported},
		{"action compose", "a=c", ErrUnsupported},
		{"action unknown", "a=x", ErrUnsupported},
		{"action too long", "a=tt", ErrMalformed},
		{"transmission file", "a=t,t=f", ErrUnsupported},
		{"transmission tempfile", "a=t,t=t", ErrUnsupported},
		{"transmission shared memory", "a=t,t=s", ErrUnsupported},
		{"transmission on place", "a=p,t=d", ErrMalformed},
		{"format png", "a=t,f=100", ErrUnsupported},
		{"format zero", "a=t,f=0", ErrMalformed},
		{"format odd", "a=t,f=8", ErrMalformed},
		{"compression", "a=t,o=z", ErrUnsupported},
		{"unicode placeholders", "a=t,U=1", ErrUnsupported},
		{"negative unicode", "a=t,U=-1", ErrUnsupported},
		{"negative width", "a=t,s=-1", ErrMalformed},
		{"bad number", "a=t,s=x", ErrMalformed},
		{"id overflow", "a=t,i=4294967296", ErrMalformed},
		{"quiet range", "a=t,q=3", ErrMalformed},
		{"more range", "a=t,m=2", ErrMalformed},
		{"cursor range", "a=t,C=2", ErrMalformed},
		{"delete without action", "d=a", ErrMalformed},
		{"delete on transmit", "a=t,d=a", ErrMalformed},
		{"delete specifier", "a=d,d=z", ErrUnsupported},
		{"delete placement by id", "a=d,d=p", ErrUnsupported},
		{"delete placement by id upper", "a=d,d=P", ErrUnsupported},
		{"width limit", "a=t,s=16385,v=1", ErrTooLarge},
		{"height limit", "a=t,s=1,v=16385", ErrTooLarge},
		{"payload not base64", "a=t;AA*A", ErrMalformed},
		{"payload padding", "a=t;A===", ErrMalformed},
		{"payload after padding", "a=t;AA==A", ErrMalformed},
		{"payload too large", "a=t;" + strings.Repeat("A", 8193), ErrTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.body), DefaultLimits())
			if !errors.Is(err, tt.want) {
				t.Fatalf("Parse(%q) = %v, want %v", tt.body, err, tt.want)
			}
		})
	}
}

func TestParseErrorKeepsQuietAndID(t *testing.T) {
	tests := []struct {
		body         string
		quiet        int
		id, number   uint32
		placementID  uint32
		wantErrCheck error
	}{
		{"S=1,q=2,i=9;AAAA", 2, 9, 0, 0, ErrUnsupported},
		{"q=2,i=9,S=1", 2, 9, 0, 0, ErrUnsupported},
		{"s=x,q=1,i=4,I=5", 1, 4, 5, 0, ErrMalformed},
		{"a=x,q=2,i=7,p=3", 2, 7, 0, 3, ErrUnsupported},
		{"a=t,i=6,q=1,s=99999,v=1", 1, 6, 0, 0, ErrTooLarge},
		{"a=t;AA*A,i=5,q=2", 0, 0, 0, 0, ErrMalformed},
	}
	for _, tt := range tests {
		cmd, err := Parse([]byte(tt.body), DefaultLimits())
		if !errors.Is(err, tt.wantErrCheck) {
			t.Fatalf("Parse(%q) = %v, want %v", tt.body, err, tt.wantErrCheck)
		}
		if cmd.Quiet != tt.quiet || cmd.ID != tt.id || cmd.Number != tt.number || cmd.PlacementID != tt.placementID {
			t.Fatalf("Parse(%q) kept %+v, want quiet=%d id=%d number=%d placement=%d",
				tt.body, cmd, tt.quiet, tt.id, tt.number, tt.placementID)
		}
	}
}

func TestParseRejectsDuplicateKeys(t *testing.T) {
	for _, body := range []string{"a=t,a=p", "i=1,i=2", "q=1,q=2", "s=1,s=2", "a=t,f=24,f=32"} {
		if _, err := Parse([]byte(body), DefaultLimits()); !errors.Is(err, ErrMalformed) {
			t.Fatalf("Parse(%q) = %v, want ErrMalformed", body, err)
		}
	}
	cmd, err := Parse([]byte("q=1,q=2,i=3"), DefaultLimits())
	if !errors.Is(err, ErrMalformed) || cmd.Quiet != 1 || cmd.ID != 3 {
		t.Fatalf("Parse kept %+v, %v", cmd, err)
	}
}

func TestParseQueryTransfer(t *testing.T) {
	cmd, err := Parse([]byte("a=q,t=d,f=32,s=1,v=1,i=1;AAAA"), DefaultLimits())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cmd.Action != ActionQuery || cmd.Transmission != 'd' || cmd.Format != 32 || cmd.ID != 1 {
		t.Fatalf("Parse = %+v", cmd)
	}
}

func TestParseUnicodeZero(t *testing.T) {
	cmd, err := Parse([]byte("a=t,U=0"), DefaultLimits())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cmd.Unicode != 0 || !cmd.Has('U') {
		t.Fatalf("U=0: Unicode=%d Has(U)=%v", cmd.Unicode, cmd.Has('U'))
	}
}

func TestParseBadRangeValue(t *testing.T) {
	for _, body := range []string{"a=t,q=x", "a=t,m=", "a=t,f=2000", "a=t,C=-1"} {
		if _, err := Parse([]byte(body), DefaultLimits()); !errors.Is(err, ErrMalformed) {
			t.Fatalf("Parse(%q) = %v, want ErrMalformed", body, err)
		}
	}
}

func TestParsePayloadIsCopied(t *testing.T) {
	body := []byte("a=t;AAAA")
	cmd, err := Parse(body, DefaultLimits())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for i := range body {
		body[i] = 'x'
	}
	if string(cmd.Payload) != "AAAA" {
		t.Fatalf("payload aliases the input: %q", cmd.Payload)
	}
}

func TestParseMorePresence(t *testing.T) {
	cmd, err := Parse([]byte("a=t,m=0"), DefaultLimits())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cmd.More || !cmd.Has('m') {
		t.Fatalf("m=0: More=%v Has(m)=%v", cmd.More, cmd.Has('m'))
	}
	cmd, err = Parse([]byte("a=t,m=1"), DefaultLimits())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !cmd.More || !cmd.Has('m') {
		t.Fatalf("m=1: More=%v Has(m)=%v", cmd.More, cmd.Has('m'))
	}
}

func TestParseZIndex(t *testing.T) {
	cmd, err := Parse([]byte("a=t,z=-7"), DefaultLimits())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cmd.Z != -7 || !cmd.Has('z') {
		t.Fatalf("Z = %d, Has(z) = %v", cmd.Z, cmd.Has('z'))
	}
}

func TestParseCustomLimits(t *testing.T) {
	if _, err := Parse([]byte("a=t;AAAAA"), Limits{MaxPayload: 4}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("payload over custom limit: %v", err)
	}
	if _, err := Parse([]byte("a=t,s=4096,v=1"), Limits{}); err != nil {
		t.Fatalf("zero Limits should allow the default width: %v", err)
	}
	if _, err := Parse([]byte("a=t,s=100,v=1"), Limits{MaxWidth: 99}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("width over custom limit: %v", err)
	}
}
