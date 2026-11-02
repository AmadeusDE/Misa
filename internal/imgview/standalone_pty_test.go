//go:build unix && !android

package imgview

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func TestStandalonePTYProbeAndRestore(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer slave.Close()
	if err = pty.Setsize(slave, &pty.Winsize{Cols: 40, Rows: 12, X: 400, Y: 240}); err != nil {
		t.Fatal(err)
	}
	v, err := New(nil, false, "auto")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- runStandalone(v, slave, slave) }()
	chunks := make(chan []byte, 64)
	go func() {
		buf := make([]byte, 8192)
		for {
			n, e := master.Read(buf)
			if n > 0 {
				chunks <- append([]byte(nil), buf[:n]...)
			}
			if e != nil {
				return
			}
		}
	}()
	var all []byte
	deadline := time.After(5 * time.Second)
	for !bytes.Contains(all, []byte("\x1b[c")) {
		select {
		case b := <-chunks:
			all = append(all, b...)
		case <-deadline:
			t.Fatal("probe not sent")
		}
	}
	// A typed key during probing must survive the terminal replies.
	if _, err = io.WriteString(master, "q\x1b[?1;2c"); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("typed q lost during probe")
	}
	for !bytes.Contains(all, []byte(standaloneLeave)) {
		select {
		case b := <-chunks:
			all = append(all, b...)
		case <-time.After(time.Second):
			t.Fatalf("terminal modes not balanced: %q", all)
		}
	}
	if !bytes.Contains(all, []byte(standaloneEnter)) {
		t.Fatalf("terminal modes not balanced: %q", all)
	}
	if !term.IsTerminal(int(slave.Fd())) {
		t.Fatal("slave not a terminal")
	}
}

// TestStandaloneSuspendChild is invoked as a subprocess by TestStandaloneSuspendPTY.
func TestStandaloneSuspendChild(t *testing.T) {
	if os.Getenv("MISA_IMGVIEW_TEST_CHILD") != "1" {
		return
	}
	v, err := New(nil, false, "halfblock")
	if err != nil {
		t.Fatal(err)
	}
	if err = Run(v); err != nil {
		t.Fatal(err)
	}
}

func TestStandaloneSuspendPTY(t *testing.T) {
	for _, embedded := range []bool{false, true} {
		t.Run(fmt.Sprint(embedded), func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestStandaloneSuspendChild$")
			cmd.Env = append(os.Environ(), "MISA_IMGVIEW_TEST_CHILD=1", fmt.Sprintf("MISA_IMGVIEW_EMBEDDED=%d", map[bool]int{false: 0, true: 1}[embedded]))
			master, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 100, Rows: 20, X: 1000, Y: 400})
			if err != nil {
				t.Fatal(err)
			}
			defer master.Close()
			defer func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() }()
			chunks := make(chan []byte, 64)
			go func() {
				b := make([]byte, 8192)
				for {
					n, e := master.Read(b)
					if n > 0 {
						chunks <- append([]byte(nil), b[:n]...)
					}
					if e != nil {
						return
					}
				}
			}()
			var data []byte
			waitText := func(text string) {
				t.Helper()
				deadline := time.After(5 * time.Second)
				for !bytes.Contains(data, []byte(text)) {
					select {
					case b := <-chunks:
						data = append(data, b...)
					case <-deadline:
						t.Fatalf("missing %q in %q", text, data)
					}
				}
			}
			waitText(standaloneEnter)
			data = nil
			if _, err = master.Write([]byte{26}); err != nil {
				t.Fatal(err)
			}
			if embedded {
				waitText("suspend unavailable when launched from Misa; q returns")
			} else {
				waitText(standaloneLeave)
				deadline := time.Now().Add(5 * time.Second)
				for {
					var status unix.WaitStatus
					pid, e := unix.Wait4(cmd.Process.Pid, &status, unix.WUNTRACED|unix.WNOHANG, nil)
					if e != nil {
						t.Fatal(e)
					}
					if pid != 0 {
						if !status.Stopped() {
							t.Fatalf("status %v", status)
						}
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("child did not stop")
					}
					time.Sleep(10 * time.Millisecond)
				}
				data = nil
				if err = cmd.Process.Signal(syscall.SIGCONT); err != nil {
					t.Fatal(err)
				}
				waitText(standaloneEnter)
			}
			if _, err = master.Write([]byte("q")); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case err = <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("child did not quit")
			}
		})
	}
}

func TestStandaloneProbeWaitsForDA(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer slave.Close()
	if err = pty.Setsize(slave, &pty.Winsize{Cols: 80, Rows: 12}); err != nil {
		t.Fatal(err)
	}
	v, err := New(nil, false, "auto")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- runStandalone(v, slave, slave) }()
	chunks := make(chan []byte, 16)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, e := master.Read(buf)
			if n > 0 {
				chunks <- append([]byte(nil), buf[:n]...)
			}
			if e != nil {
				return
			}
		}
	}()
	var data []byte
	for !bytes.Contains(data, []byte("\x1b[c")) {
		select {
		case b := <-chunks:
			data = append(data, b...)
		case <-time.After(5 * time.Second):
			t.Fatal("probe missing")
		}
	}
	match := regexp.MustCompile(`i=([0-9]+)`).FindSubmatch(data)
	if len(match) != 2 {
		t.Fatalf("probe ID missing: %q", data)
	}
	if _, err = fmt.Fprintf(master, "q\x1b_Gi=%s;OK\x1b\\", match[1]); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		t.Fatalf("exited before DA reply: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if _, err = io.WriteString(master, "\x1b[?1;2c"); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("did not exit after DA reply")
	}
}
