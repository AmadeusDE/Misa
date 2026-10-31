package term

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

const nativeHelperEnv = "MISA_TERM_NATIVE_HELPER"
const nativeTimeout = 20 * time.Second

// The child writes short base64 fragments, so wrapping and ConPTY's VT control
// sequences do not make assumptions about the raw terminal stream.
func nativeEmit(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	s := base64.StdEncoding.EncodeToString(b)
	fmt.Fprintln(os.Stdout, "MISA-BEGIN")
	for len(s) > 0 {
		n := min(32, len(s))
		fmt.Fprintf(os.Stdout, "MISA-%s\r\n", s[:n])
		s = s[n:]
	}
	fmt.Fprintln(os.Stdout, "MISA-END")
}

type nativePayload struct {
	Args       []string
	Env        []string
	Cwd        string
	Unicode    string
	Input      string
	Rows, Cols int
	Error      string
}

func nativeSize(p *nativePayload) {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(os.Stdout.Fd()), &info); err != nil {
		p.Error = err.Error()
		return
	}
	p.Cols = int(info.Window.Right - info.Window.Left + 1)
	p.Rows = int(info.Window.Bottom - info.Window.Top + 1)
}

func TestNativeTransportHelper(t *testing.T) {
	mode := os.Getenv(nativeHelperEnv)
	if mode == "" {
		return
	}
	if err := windows.SetConsoleOutputCP(65001); err != nil {
		os.Exit(91)
	}
	switch mode {
	case "inspect":
		cwd, err := os.Getwd()
		if err != nil {
			os.Exit(92)
		}
		var env []string
		for _, e := range os.Environ() {
			if strings.HasPrefix(strings.ToLower(e), "misa_case_test=") {
				env = append(env, e)
			}
		}
		nativeEmit(nativePayload{Args: os.Args[3:], Env: env, Cwd: cwd, Unicode: os.Getenv("MISA_UNICODE")})
		os.Exit(23)
	case "large":
		for i := range 10000 {
			fmt.Fprintf(os.Stdout, "SEQ%05dZ %s\r\n", i, strings.Repeat("x", 48))
		}
		nativeEmit(nativePayload{Input: "large complete"})
		os.Exit(0)
	case "interactive":
		p := nativePayload{}
		nativeSize(&p)
		nativeEmit(p)
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			os.Exit(93)
		}
		p.Input = strings.TrimRight(line, "\r\n")
		nativeSize(&p)
		nativeEmit(p)
		os.Exit(0)
	default:
		os.Exit(94)
	}
}

type nativeCapture struct {
	mu     sync.Mutex
	b      bytes.Buffer
	notify chan struct{}
	done   chan struct{}
	err    error
}

func (c *nativeCapture) read(r io.Reader) {
	buf := make([]byte, 32768)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			c.mu.Lock()
			c.b.Write(buf[:n])
			c.mu.Unlock()
			select {
			case c.notify <- struct{}{}:
			default:
			}
		}
		if err != nil {
			c.mu.Lock()
			c.err = err
			c.mu.Unlock()
			close(c.done)
			return
		}
	}
}
func (c *nativeCapture) text() string     { c.mu.Lock(); defer c.mu.Unlock(); return c.b.String() }
func (c *nativeCapture) readError() error { c.mu.Lock(); defer c.mu.Unlock(); return c.err }

var nativeFragment = regexp.MustCompile(`MISA-([A-Za-z0-9+/=]{1,32})`)

func nativeMessages(s string) []nativePayload {
	var out []nativePayload
	for {
		start := strings.Index(s, "MISA-BEGIN")
		if start < 0 {
			break
		}
		s = s[start+len("MISA-BEGIN"):]
		end := strings.Index(s, "MISA-END")
		if end < 0 {
			break
		}
		var encoded strings.Builder
		for _, m := range nativeFragment.FindAllStringSubmatch(s[:end], -1) {
			encoded.WriteString(m[1])
		}
		raw, err := base64.StdEncoding.DecodeString(encoded.String())
		var p nativePayload
		if err == nil && json.Unmarshal(raw, &p) == nil {
			out = append(out, p)
		}
		s = s[end+len("MISA-END"):]
	}
	return out
}
func nativeWaitMessages(t *testing.T, c *nativeCapture, count int) []nativePayload {
	t.Helper()
	timer := time.NewTimer(nativeTimeout)
	defer timer.Stop()
	for {
		p := nativeMessages(c.text())
		if len(p) >= count {
			return p
		}
		select {
		case <-c.notify:
		case <-c.done:
			output := c.text()
			p := nativeMessages(output)
			if len(p) >= count {
				return p
			}
			if len(output) > 4096 {
				output = "..." + output[len(output)-4096:]
			}
			t.Fatalf("output ended waiting for %d payload(s), received %d: %v; output tail: %q", count, len(p), c.readError(), output)
		case <-timer.C:
			t.Fatalf("timed out waiting for %d payload(s), received %d", count, len(p))
		}
	}
}
func nativeStart(t *testing.T, mode, dir string, args ...string) (conn, proc, *nativeCapture) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return nativeStartExe(t, mode, dir, exe, nil, args...)
}

func nativeStartExe(t *testing.T, mode, dir, exe string, extraEnv []string, args ...string) (conn, proc, *nativeCapture) {
	t.Helper()
	argv := append([]string{exe, "-test.run=^TestNativeTransportHelper$", "--"}, args...)
	env := append(os.Environ(), nativeHelperEnv+"="+mode, "MISA_CASE_TEST=first", "misa_case_test=last", "MISA_UNICODE=日本語 🐈")
	env = append(env, extraEnv...)
	c, p, err := startProcess(startSpec{Argv: argv, Dir: dir, Env: env, Rows: 24, Cols: 80})
	if errors.Is(err, ErrUnsupported) {
		t.Skipf("ConPTY unsupported: %v", err)
	}
	if err != nil {
		t.Fatalf("startProcess: %v", err)
	}
	t.Cleanup(func() {
		done := make(chan struct{})
		go func() { p.Kill(); _ = c.Close(); close(done) }()
		select {
		case <-done:
		case <-time.After(nativeTimeout):
			t.Error("transport cleanup timed out")
		}
	})
	cap := &nativeCapture{notify: make(chan struct{}, 1), done: make(chan struct{})}
	go cap.read(c)
	return c, p, cap
}
func nativeStatus(t *testing.T, p proc, want int) {
	t.Helper()
	select {
	case got, ok := <-p.Status():
		if !ok || got != want {
			t.Errorf("status = %d (open %v), want %d", got, ok, want)
		}
	case <-time.After(nativeTimeout):
		t.Fatal("timed out waiting for process status")
	}
}
