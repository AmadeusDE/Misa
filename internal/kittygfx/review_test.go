package kittygfx

// Independent security review tests (PR 5a). These exercise the wire-format
// package from an untrusted-PTY-client point of view: every input here is
// attacker controlled. They are deliberately separate from the package's own
// tests so a failure points at a review finding, not at a changed fixture.

import (
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

// decodeNoPanic calls Decode and converts a panic into an error, because a
// panic in the terminal's reader goroutine kills the whole Misa process.
func decodeNoPanic(cmd Command, encoded []byte, lim Limits) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("panic: " + panicString(r))
		}
	}()
	_, err = Decode(cmd, encoded, lim)
	return err
}

func panicString(r any) string {
	if s, ok := r.(string); ok {
		return s
	}
	if e, ok := r.(error); ok {
		return e.Error()
	}
	return "non-string panic"
}

// TestReviewDecodeDoesNotPanicOnOverlongBase64 feeds payloads whose decoded
// length is larger than s*v*bytesPerPixel but whose *encoded* length is exactly
// EncodedLen(expected). encoding/base64.Decode writes three bytes per quantum
// and panics when the destination is shorter, so Decode must derive the
// destination size from the payload, not from the declared image size.
//
// Such a payload only exists when the pixel count is not a multiple of three
// (f=32 with w*h%3!=0): EncodedLen rounds up to a whole quantum, so a full
// quantum of unpadded base64 can carry up to two bytes more than the image.
func TestReviewDecodeDoesNotPanicOnOverlongBase64(t *testing.T) {
	tests := []struct {
		name    string
		width   int
		height  int
		format  int
		payload string
	}{
		// f=32, 1x1: 4 pixel bytes, EncodedLen 8, up to 6 decoded bytes.
		{"rgba 5 bytes", 1, 1, 32, "AAAAAAA="},
		{"rgba 6 bytes", 1, 1, 32, "AAAAAAAA"},
		// f=32, 1x2: 8 pixel bytes, EncodedLen 12, up to 9 decoded bytes.
		{"rgba 9 bytes", 1, 2, 32, strings.Repeat("A", 12)},
		// f=32, 2x2: 16 pixel bytes, EncodedLen 24, up to 18 decoded bytes.
		{"rgba 18 bytes", 2, 2, 32, strings.Repeat("A", 24)},
		{"rgba 17 bytes", 2, 2, 32, strings.Repeat("A", 23) + "="},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := tt.width * tt.height
			if tt.format == 32 {
				size *= 4
			} else {
				size *= 3
			}
			if want := base64EncodedLen(size); len(tt.payload) != want {
				t.Fatalf("test bug: payload %d bytes, want EncodedLen(%d)=%d",
					len(tt.payload), size, want)
			}
			if got, err := decodedLength([]byte(tt.payload)); err != nil || got <= int64(size) {
				t.Fatalf("test bug: payload decodes to %d (%v), want more than %d", got, err, size)
			}
			cmd := Command{Action: ActionTransmit, Transmission: 'd',
				Format: tt.format, Width: tt.width, Height: tt.height, ID: 1}
			err := decodeNoPanic(cmd, []byte(tt.payload), DefaultLimits())
			if err == nil {
				t.Fatalf("Decode accepted a payload that decodes to more than %dx%d pixels",
					tt.width, tt.height)
			}
			if strings.HasPrefix(err.Error(), "panic: ") {
				t.Fatalf("Decode panicked on attacker-controlled input: %v", err)
			}
		})
	}
}

// base64EncodedLen mirrors encoding/base64's EncodedLen for a raw byte count.
func base64EncodedLen(bytes int) int { return (bytes + 2) / 3 * 4 }

// TestReviewDecodeRejectsShortDecodedPayload covers the other direction: a
// payload of exactly the expected encoded length that decodes to fewer bytes.
// encoding/base64 skips newlines and accepts a two-character final quantum, so
// a payload can pass the length check and still fill fewer destination bytes.
// A raster must never be published with an unfilled tail (it would show a
// spurious zero pixel) and the decoder must not trust the length alone.
func TestReviewDecodeRejectsShortDecodedPayload(t *testing.T) {
	tests := []struct {
		name    string
		width   int
		height  int
		format  int
		payload string
	}{
		// f=24, 2x1: 6 pixel bytes, EncodedLen 8. Trailing newlines are skipped
		// by encoding/base64, leaving a six character quantum group.
		{"rgb newlines", 2, 1, 24, "AAAAAA\n\n"},
		// f=32, 1x1: 4 pixel bytes, EncodedLen 8.
		{"rgba newlines", 1, 1, 32, "AAAA\n\n\n\n"},
		// Not a whole quantum: base64 pads or fails, never a short fill.
		{"not a quantum", 2, 1, 24, "AAAAA"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := tt.width * tt.height
			if tt.format == 32 {
				size *= 4
			} else {
				size *= 3
			}
			cmd := Command{Action: ActionTransmit, Transmission: 'd',
				Format: tt.format, Width: tt.width, Height: tt.height, ID: 1}
			raster, err := Decode(cmd, []byte(tt.payload), DefaultLimits())
			if err == nil {
				t.Fatalf("Decode accepted a %d byte payload for %d pixel bytes: %+v",
					len(tt.payload), size, raster)
			}
			if strings.HasPrefix(err.Error(), "panic: ") {
				t.Fatalf("Decode panicked: %v", err)
			}
		})
	}
}

// TestReviewDecodeRejectsHugeDeclaredImageBeforeAllocating makes sure a client
// cannot make the decoder reserve the per-image budget with a four byte
// payload: the declared size must be checked against the payload before the
// pixel buffer is allocated.
func TestReviewDecodeRejectsHugeDeclaredImageBeforeAllocating(t *testing.T) {
	cmd := Command{Action: ActionTransmit, Transmission: 'd', Format: 32,
		Width: 4096, Height: 4096, ID: 1}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	start := time.Now()
	err := decodeNoPanic(cmd, []byte("AAAA"), DefaultLimits())
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)
	if err == nil {
		t.Fatal("Decode accepted a 4096x4096 image with a four byte payload")
	}
	if grew := after.TotalAlloc - before.TotalAlloc; grew > 4<<20 {
		t.Errorf("Decode allocated %d bytes before rejecting the payload", grew)
	}
	if elapsed > time.Second {
		t.Errorf("Decode took %v to reject a short payload", elapsed)
	}
}

// TestReviewParseHugeControlSectionIsBounded documents that Parse itself puts
// no bound on the number of control keys; it must at least terminate quickly
// and report an error instead of accumulating state.
func TestReviewParseHugeControlSectionIsBounded(t *testing.T) {
	body := []byte(strings.Repeat("a=t,", 1<<18)) // 1 MiB of keys, no payload
	start := time.Now()
	if _, err := Parse(body, DefaultLimits()); err == nil {
		t.Fatal("Parse accepted a megabyte of control keys")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Parse of a 1 MiB control section took %v", elapsed)
	}
}

// TestReviewReplyCarriesPlacementID: a placement reply has to echo p, because
// a=p with p absent gets an id assigned by the terminal and the client learns
// it only from the reply.
func TestReviewReplyCarriesPlacementID(t *testing.T) {
	got := string(Reply(Command{ID: 4, PlacementID: 9}, "OK", ""))
	if !strings.Contains(got, "p=9") {
		t.Fatalf("placement reply %q does not carry p=9", got)
	}
	if got := string(Reply(Command{ID: 4, PlacementID: 9, Quiet: 1}, "OK", "")); got != "" {
		t.Fatalf("q=1 still replied %q to a success", got)
	}
	if got := string(Reply(Command{ID: 4, PlacementID: 9, Quiet: 2}, "EINVAL", "x")); got != "" {
		t.Fatalf("q=2 replied %q to an error", got)
	}
}

// TestReviewEncodeCannotInjectKeys: every Command field that reaches the wire
// is either an integer or a checked byte, so no field may smuggle a second
// command, a semicolon or a control byte into the APC.
func TestReviewEncodeCannotInjectKeys(t *testing.T) {
	for _, cmd := range []Command{
		{Action: ';'},
		{Action: ActionDelete, Delete: ';'},
		{Action: ActionDelete, Delete: ','},
		{Action: ActionTransmit, Transmission: ';'},
		{Action: ActionTransmit, Format: -1},
		{Action: ActionTransmit, Quiet: -1},
		{Action: ActionTransmit, CursorMovement: 2},
		{Action: ActionTransmit, Unicode: 1},
	} {
		wire, err := Encode(cmd, nil)
		if err == nil {
			t.Fatalf("Encode(%+v) = %q, want an error", cmd, wire)
		}
		if strings.ContainsAny(string(wire), "\x1b\x07") && len(wire) > 0 {
			t.Fatalf("Encode(%+v) emitted a control byte: %q", cmd, wire)
		}
	}
	wire, err := Encode(Command{Action: ActionTransmit, ID: 1, Width: 1, Height: 1}, []byte("AAAA"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(wire), ";"); n != 1 {
		t.Fatalf("Encode emitted %d semicolons: %q", n, wire)
	}
}

// TestReviewParseRejectsBareContinuationPayload documents the wire form of a
// chunk continuation: Parse only accepts "<keys>;<payload>", so a chunk sent
// as a bare payload (no semicolon) is malformed. Kitty clients that omit the
// semicolon therefore get an error reply rather than a stored image; the
// terminal has to decide whether to accept that form when a transfer is open.
func TestReviewParseRejectsBareContinuationPayload(t *testing.T) {
	if _, err := Parse([]byte("AAAA"), DefaultLimits()); !errors.Is(err, ErrMalformed) {
		t.Fatalf("Parse of a bare payload = %v, want ErrMalformed", err)
	}
	if _, err := Parse([]byte(";AAAA"), DefaultLimits()); !errors.Is(err, ErrMalformed) {
		t.Fatalf("Parse of an empty key list = %v, want ErrMalformed", err)
	}
	// The form Misa accepts: keys (here only m) and a semicolon.
	if _, err := Parse([]byte("m=1;AAAA"), DefaultLimits()); err != nil {
		t.Fatalf("Parse of m=1;payload = %v", err)
	}
}

// TestReviewParseRejectsZeroIDForTransmit: an image that no later command can
// address is useless, and Reply cannot echo i=0, so a transmit without a usable
// id has to be rejected by whoever stores it. Parse reports presence, which is
// what the store needs to tell "i=0" from "no i".
func TestReviewParseRejectsZeroIDForTransmit(t *testing.T) {
	cmd, err := Parse([]byte("a=t,f=24,s=1,v=1,i=0;AAAA"), DefaultLimits())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cmd.ID != 0 || !cmd.Has('i') {
		t.Fatalf("i=0: ID=%d Has(i)=%v, want 0 and true", cmd.ID, cmd.Has('i'))
	}
	cmd, err = Parse([]byte("a=t,f=24,s=1,v=1;AAAA"), DefaultLimits())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cmd.Has('i') || cmd.Has('I') {
		t.Fatalf("no id given but Has(i)=%v Has(I)=%v", cmd.Has('i'), cmd.Has('I'))
	}
}

// TestReviewDecodeNeverAcceptsMismatchedDimensions: the pixel count has to
// match s*v exactly for both formats, so a client cannot declare a small image
// and send more data (or the reverse) and get a raster whose Bounds disagree
// with its payload.
func TestReviewDecodeNeverAcceptsMismatchedDimensions(t *testing.T) {
	for _, format := range []int{24, 32} {
		bpp := 3
		if format == 32 {
			bpp = 4
		}
		// 2x1 image, payload for 1x1 and for 3x1.
		for _, pixels := range []int{1, 3} {
			raw := make([]byte, pixels*bpp)
			cmd := Command{Action: ActionTransmit, Transmission: 'd', Format: format,
				Width: 2, Height: 1, ID: 1}
			if err := decodeNoPanic(cmd, []byte(encodePixels(raw)), DefaultLimits()); err == nil {
				t.Fatalf("f=%d: Decode accepted %d pixels for a 2x1 image", format, pixels)
			}
		}
	}
}
