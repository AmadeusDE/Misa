package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The permission gate reads "path" with an exact-case lookup that ignores
// everything else; a tool must never run on a different reading.
func TestToolsRefuseArgumentsTheGateWouldReadDifferently(t *testing.T) {
	cwd := t.TempDir()
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("TOP SECRET"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "ok"), []byte("fine"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{"path":"ok","PATH":"` + secret + `"}`,
		`{"path":"ok","path":"` + secret + `"}`,
		`{"path":"ok"}{"path":"` + secret + `"}`,
	} {
		out, err := readTool{}.Run(context.Background(), cwd, json.RawMessage(raw))
		if err == nil {
			t.Errorf("read %s ran: %q", raw, out)
		}
	}
}

func TestReadAndEditRefuseDevicesAndFIFOs(t *testing.T) {
	cwd := t.TempDir()
	fifo := filepath.Join(cwd, "fifo")
	if err := mkfifo(fifo); err != nil {
		t.Skip(err)
	}
	for _, p := range []string{"/dev/zero", fifo} {
		done := make(chan error, 2)
		go func() {
			_, err := readTool{}.Run(context.Background(), cwd, json.RawMessage(`{"path":"`+p+`"}`))
			done <- err
		}()
		go func() {
			_, err := editTool{}.Run(context.Background(), cwd, json.RawMessage(`{"path":"`+p+`","old_text":"a","new_text":"b"}`))
			done <- err
		}()
		for range 2 {
			select {
			case err := <-done:
				if err == nil {
					t.Errorf("%s: no error", p)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("%s: hung", p)
			}
		}
	}
}
