//go:build unix

package commandexec

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

type slowWriter struct{ bytes.Buffer }

func (w *slowWriter) Write(b []byte) (int, error) {
	time.Sleep(20 * time.Millisecond)
	return w.Buffer.Write(b)
}

// A slow consumer must not lose output buffered in the pipe, even with a
// drain bound, and zero WaitDelay waits for descendant output.
func TestWaitDelayKeepsOutput(t *testing.T) {
	script := "i=0; while [ $i -lt 20 ]; do echo line$i; i=$((i+1)); done"
	var out slowWriter
	p, err := Start(context.Background(), "/bin/sh", Invocation{Args: []string{"-c", script}, Stdout: &out}, Options{WaitDelay: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if code := p.Wait(); code != 0 || strings.Count(out.String(), "\n") != 20 {
		t.Fatalf("bounded drain truncated: %d %q", code, out.String())
	}
	var late bytes.Buffer
	p, err = Start(context.Background(), "/bin/sh", Invocation{Args: []string{"-c", "(sleep 0.3; echo late) & exit 0"}, Stdout: &late}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if code := p.Wait(); code != 0 || late.String() != "late\n" {
		t.Fatalf("zero WaitDelay should wait for EOF: %d %q", code, late.String())
	}
}
