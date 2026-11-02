package term

import (
	"bufio"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/kittygfx"
)

// readAPCReply reads one complete graphics reply.
func readAPCReply(t *testing.T, br *bufio.Reader) string {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		for {
			c, err := br.ReadByte()
			if err != nil {
				break
			}
			b.WriteByte(c)
			if strings.HasSuffix(b.String(), "\x1b\\") {
				break
			}
		}
		done <- b.String()
	}()
	select {
	case s := <-done:
		return s
	case <-time.After(2 * time.Second):
		t.Fatal("no graphics reply")
		return ""
	}
}

func wantReplyCode(t *testing.T, reply, code string) {
	t.Helper()
	if !strings.HasPrefix(reply, "\x1b_G") || !strings.HasSuffix(reply, "\x1b\\") {
		t.Fatalf("reply %q is not a graphics APC", reply)
	}
	if i := strings.IndexByte(reply, ';'); i < 0 || !strings.HasPrefix(reply[i+1:], code) {
		t.Fatalf("reply %q, want code %s", reply, code)
	}
}

func TestGraphicsReplies(t *testing.T) {
	tm := gfxTerm(t, 10, 20)
	br := pipeReplies(t, tm)
	tm.ProcessOutput(gfxAPC("a=t,f=32,i=1,s=1,v=1;" + gfxPixels(1, 1, 4)))
	wantReplyCode(t, readAPCReply(t, br), "OK")
	tm.ProcessOutput(gfxAPC("a=p,i=1,c=1,r=1"))
	wantReplyCode(t, readAPCReply(t, br), "OK")
	tm.ProcessOutput(gfxAPC("a=q,i=1,s=1,v=1;" + gfxPixels(1, 1, 4)))
	wantReplyCode(t, readAPCReply(t, br), "OK")
	tm.ProcessOutput(gfxAPC("a=d,d=a"))
	wantReplyCode(t, readAPCReply(t, br), "OK")
}

func TestGraphicsQuietLevels(t *testing.T) {
	t.Run("q=1 hides success", func(t *testing.T) {
		tm := gfxTerm(t, 10, 20)
		r, _ := newSilentPipe(t, tm)
		tm.ProcessOutput(gfxAPC("a=t,f=32,i=1,s=1,v=1,q=1;" + gfxPixels(1, 1, 4)))
		expectSilence(t, r)
	})
	t.Run("q=1 shows errors", func(t *testing.T) {
		tm := gfxTerm(t, 10, 20)
		br := pipeReplies(t, tm)
		tm.ProcessOutput(gfxAPC("a=p,i=9,q=1,c=1,r=1"))
		wantReplyCode(t, readAPCReply(t, br), "ENOENT")
	})
	t.Run("q=2 hides errors", func(t *testing.T) {
		tm := gfxTerm(t, 10, 20)
		r, _ := newSilentPipe(t, tm)
		tm.ProcessOutput(gfxAPC("a=p,i=9,q=2,c=1,r=1"))
		expectSilence(t, r)
	})
}

func TestGraphicsMalformedBodies(t *testing.T) {
	cases := []struct {
		name string
		body string
		code string
	}{
		{"png format", "a=t,f=100,i=1,s=2,v=2;AAAA", "EINVAL"},
		{"compression", "a=t,o=z,i=1,s=2,v=2;AAAA", "EINVAL"},
		{"file transport", "a=t,t=f,i=1,s=2,v=2;AAAA", "EINVAL"},
		{"native animation", "a=a,i=1", "EINVAL"},
		{"composition", "a=c,i=1", "EINVAL"},
		{"not base64", "a=t,f=32,i=1,s=2,v=2;!!!!", "EINVAL"},
		{"short payload", "a=t,f=32,i=1,s=2,v=2;AAAA", "EINVAL"},
		{"no payload", "a=t,f=32,i=1,s=2,v=2;", "EINVAL"},
		{"no key value", "bogus", "EINVAL"},
		{"unknown key", "a=t,i=1,k=9", "EINVAL"},
		{"delete without spec", "a=d", "EINVAL"},
		{"delete of nothing", "a=d,d=i,i=4", "OK"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tm := gfxTerm(t, 10, 20)
			br := pipeReplies(t, tm)
			tm.ProcessOutput(gfxAPC(c.body))
			wantReplyCode(t, readAPCReply(t, br), c.code)
			if _, got := gfxSnapshot(t, tm); len(got) != 0 {
				t.Fatalf("malformed command placed %+v", got)
			}
		})
	}
}

func TestGraphicsDisabledSwallowsAndStaysSilent(t *testing.T) {
	tm := New(4, 20)
	r, _ := newSilentPipe(t, tm)
	stream := append([]byte("ab"), gfxAPC("a=T,f=32,i=1,s=2,v=2;"+gfxPixels(2, 2, 4))...)
	stream = append(stream, "cd"...)
	tm.ProcessOutput(stream)
	expectSilence(t, r)
	var s Snapshot
	tm.Snapshot(&s)
	if len(s.Graphics) != 0 {
		t.Fatalf("a disabled terminal stored %+v", s.Graphics)
	}
	if row := s.Cells[:4]; row[0].Ch != 'a' || row[1].Ch != 'b' || row[2].Ch != 'c' || row[3].Ch != 'd' {
		t.Fatalf("cells = %q%q%q%q, want abcd: the payload must not become text", row[0].Ch, row[1].Ch, row[2].Ch, row[3].Ch)
	}
	tm.SetGraphicsEnabled(true)
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=2,v=2;" + gfxPixels(2, 2, 4)))
	if _, got := gfxSnapshot(t, tm); len(got) != 1 {
		t.Fatalf("placements after enabling = %+v, want one", got)
	}
	tm.SetGraphicsEnabled(false)
	if _, got := gfxSnapshot(t, tm); len(got) != 0 {
		t.Fatalf("disabling kept %+v", got)
	}
}

func TestGraphicsPayloadNeverExecutesEscapes(t *testing.T) {
	tm := gfxTerm(t, 4, 20)
	tm.ProcessOutput([]byte("\x1b_Ga=t,f=32,i=1,s=1,v=1;\x1b[31mred\x1b\\"))
	tm.ProcessOutput([]byte("x"))
	var s Snapshot
	tm.Snapshot(&s)
	if s.Cells[0].Ch != 'x' || s.Cells[0].HasFG {
		t.Fatalf("cell = %+v, want a plain x: an escape inside an APC is not a command", s.Cells[0])
	}
	if len(s.Graphics) != 0 {
		t.Fatalf("aborted transfer stored %+v", s.Graphics)
	}
}

func TestGraphicsUnterminatedAPCThenReset(t *testing.T) {
	tm := gfxTerm(t, 4, 20)
	tm.ProcessOutput([]byte("\x1b_Ga=t,f=32,i=1,s=2,v=2;AAAAAA=="))
	if tm.gfx.transfer != nil {
		t.Fatal("an unterminated transmit opened a transfer")
	}
	tm.ProcessOutput([]byte("\x1bc\x1b\\"))
	tm.ProcessOutput([]byte("ok"))
	var s Snapshot
	tm.Snapshot(&s)
	if s.Cells[0].Ch != 'o' || s.Cells[1].Ch != 'k' {
		t.Fatalf("screen starts %q%q, want ok after the reset", s.Cells[0].Ch, s.Cells[1].Ch)
	}
	if len(s.Graphics) != 0 {
		t.Fatalf("reset left %+v", s.Graphics)
	}
}

func TestGraphicsEscapeInPayloadResetsAndSwallows(t *testing.T) {
	tm := gfxTerm(t, 4, 20)
	tm.ProcessOutput([]byte("\x1b_Ga=t,i=1;junk\x1bc"))
	var s Snapshot
	tm.Snapshot(&s)
	for i, c := range s.Cells[:8] {
		if c.Ch != ' ' {
			t.Fatalf("cell %d = %q, want blank: the payload never reaches the screen", i, c.Ch)
		}
	}
	if len(s.Graphics) != 0 {
		t.Fatalf("aborted transfer stored %+v", s.Graphics)
	}
	tm.ProcessOutput([]byte("\x1b_Ga=t,i=1;\x1b[31mred\x1b\\"))
	tm.Snapshot(&s)
	if s.Cells[0].Ch != ' ' || s.Cells[0].HasFG {
		t.Fatalf("an escape inside a payload was executed: %+v", s.Cells[0])
	}
}

func TestGraphicsOversizedPayload(t *testing.T) {
	tm := gfxTerm(t, 4, 20)
	br := pipeReplies(t, tm)
	body := "a=t,f=32,i=1,s=1,v=1;" + strings.Repeat("A", 2*defaultGraphicsLimits.payloadBytes)
	tm.ProcessOutput(gfxAPC(body))
	wantReplyCode(t, readAPCReply(t, br), "ENOSPC")
	tm.ProcessOutput(gfxAPC("a=t,f=32,i=1,s=1,v=1;" + gfxPixels(1, 1, 4)))
	wantReplyCode(t, readAPCReply(t, br), "OK")
	if tm.gfx.image(screenMain, 1) == nil {
		t.Error("a valid transfer after the oversized one was rejected")
	}
}

func TestGraphicsInFlightLimitReclaimsTransfer(t *testing.T) {
	tm := gfxTerm(t, 4, 20)
	tm.SetGraphicsLimits(kittygfx.Limits{MaxInFlightBytes: 8})
	br := pipeReplies(t, tm)
	payload := gfxPixels(4, 4, 4)
	tm.ProcessOutput(gfxAPC("a=t,f=32,i=1,s=4,v=4,m=1;" + payload[:8]))
	tm.ProcessOutput(gfxAPC("m=1;" + payload[8:24]))
	wantReplyCode(t, readAPCReply(t, br), "ENOSPC")
	if tm.gfx.transfer != nil {
		t.Fatal("a transfer over the in-flight limit was kept")
	}
}

func TestGraphicsContinuationNeedsNoIdentity(t *testing.T) {
	tm := gfxTerm(t, 4, 20)
	br := pipeReplies(t, tm)
	payload := gfxPixels(2, 2, 4)
	tm.ProcessOutput(gfxAPC("a=t,f=32,i=3,s=2,v=2,m=1;" + payload[:12]))
	tm.ProcessOutput(gfxAPC("m=0;" + payload[12:]))
	wantReplyCode(t, readAPCReply(t, br), "OK")
	if tm.gfx.image(screenMain, 3) == nil {
		t.Fatal("a continuation chunk without i/I was rejected")
	}
}

func TestGraphicsTransferReclaimedByReset(t *testing.T) {
	tm := gfxTerm(t, 4, 20)
	tm.ProcessOutput(gfxAPC("a=t,f=32,i=1,s=2,v=2,m=1;AAAA"))
	if tm.gfx.transfer == nil {
		t.Fatal("no transfer in flight")
	}
	tm.ProcessOutput([]byte("\x1bc"))
	if tm.gfx.transfer != nil {
		t.Fatal("RIS kept the partial transfer")
	}
}

func TestGraphicsSameReadOrdering(t *testing.T) {
	tm := gfxTerm(t, 3, 20)
	stream := append([]byte("\x1b[3;1H"), gfxAPC("a=T,f=32,i=1,s=1,v=1;"+gfxPixels(1, 1, 4))...)
	stream = append(stream, "\n\n"...)
	tm.ProcessOutput(stream)
	_, got := gfxSnapshot(t, tm)
	if len(got) != 1 {
		t.Fatalf("placements = %+v, want one", got)
	}
	if got[0].CellRect.Min.Y != 0 {
		t.Errorf("placement row = %d, want 0: the scroll after the APC in the same read moved it", got[0].CellRect.Min.Y)
	}
}

func TestGraphicsQueryProbeNeedsNoStoredImage(t *testing.T) {
	tm := gfxTerm(t, 4, 20)
	br := pipeReplies(t, tm)
	tm.ProcessOutput(gfxAPC("a=q,i=31,s=1,v=1;" + gfxPixels(1, 1, 4)))
	wantReplyCode(t, readAPCReply(t, br), "OK")
	if len(tm.gfx.images) != 0 {
		t.Fatalf("a query stored images: %v", tm.gfx.images)
	}
	tm.ProcessOutput(gfxAPC("a=q"))
	wantReplyCode(t, readAPCReply(t, br), "OK")
}

func TestGraphicsPlaceUnknownImage(t *testing.T) {
	tm := gfxTerm(t, 4, 20)
	br := pipeReplies(t, tm)
	tm.ProcessOutput(gfxAPC("a=p,i=99,c=1,r=1"))
	wantReplyCode(t, readAPCReply(t, br), "ENOENT")
	if _, got := gfxSnapshot(t, tm); len(got) != 0 {
		t.Fatalf("placements = %+v, want none", got)
	}
}

func TestGraphicsNumberOnlyIdentity(t *testing.T) {
	tm := gfxTerm(t, 4, 20)
	tm.SetCellPixels(5, 10)
	br := pipeReplies(t, tm)
	tm.ProcessOutput(gfxAPC("a=t,f=32,I=5,s=1,v=1;" + gfxPixels(1, 1, 4)))
	wantReplyCode(t, readAPCReply(t, br), "OK")
	tm.ProcessOutput(gfxAPC("a=p,I=5,c=1,r=1"))
	wantReplyCode(t, readAPCReply(t, br), "OK")
	if _, got := gfxSnapshot(t, tm); len(got) != 1 || got[0].ImageID == 0 {
		t.Fatalf("placements = %+v, want one with an allocated id", got)
	}
	tm.ProcessOutput(gfxAPC("a=d,d=N,I=5"))
	wantReplyCode(t, readAPCReply(t, br), "OK")
	if _, got := gfxSnapshot(t, tm); len(got) != 0 {
		t.Fatalf("placements after deleting the number = %+v", got)
	}
}

func TestGraphicsDeleteStaysOnItsScreen(t *testing.T) {
	tm := gfxTerm(t, 4, 20)
	tm.SetCellPixels(5, 10)
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=1,v=1;" + gfxPixels(1, 1, 4)))
	tm.ProcessOutput([]byte("\x1b[?1049h"))
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=2,s=1,v=1;" + gfxPixels(1, 1, 4)))
	tm.ProcessOutput(gfxAPC("a=d,d=A"))
	if _, got := gfxSnapshot(t, tm); len(got) != 0 {
		t.Fatalf("alt placements = %+v, want none", got)
	}
	if tm.gfx.image(screenMain, 1) == nil {
		t.Error("d=A on the alt screen dropped the main screen's image")
	}
	tm.ProcessOutput([]byte("\x1b[?1049l"))
	if _, got := gfxSnapshot(t, tm); len(got) != 1 {
		t.Fatalf("main placements = %+v, want the untouched one", got)
	}
}

func TestGraphicsLimitsGetter(t *testing.T) {
	tm := gfxTerm(t, 4, 20)
	if got := tm.GraphicsLimits(); got != kittygfx.DefaultLimits() {
		t.Fatalf("default limits = %+v, want %+v", got, kittygfx.DefaultLimits())
	}
	want := kittygfx.Limits{MaxImages: 3, MaxPayload: 64}
	tm.SetGraphicsLimits(want)
	if got, exp := tm.GraphicsLimits(), want.Normalize(); got != exp {
		t.Fatalf("limits = %+v, want %+v: unset fields take the defaults", got, exp)
	}
}

func TestGraphicsClosedStaysClosed(t *testing.T) {
	tm := gfxTerm(t, 4, 20)
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=1,v=1;" + gfxPixels(1, 1, 4)))
	if _, got := gfxSnapshot(t, tm); len(got) != 1 {
		t.Fatalf("placements = %+v, want one", got)
	}
	tm.Close()
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=2,s=1,v=1;" + gfxPixels(1, 1, 4)))
	tm.SetGraphicsEnabled(true)
	if tm.GraphicsEnabled() {
		t.Fatal("a closed terminal enabled graphics again")
	}
	if _, got := gfxSnapshot(t, tm); len(got) != 0 {
		t.Fatalf("closed terminal stored %+v", got)
	}
}
