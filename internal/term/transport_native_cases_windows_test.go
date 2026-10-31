package term

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestNativeTransportArgumentsEnvironmentAndStatus(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "工作 dir")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	args := []string{"space in argument", "", `a"b`, "日本語 🐈", `ends with slash\\`, `quote\"and\\`}
	_, p, cap := nativeStart(t, "inspect", dir, args...)
	if p.ID() <= 0 {
		t.Errorf("invalid process ID %d", p.ID())
	}
	if _, ok := p.(continuer); ok {
		t.Error("Windows process unexpectedly supports job control")
	}
	got := nativeWaitMessages(t, cap, 1)[0]
	if !reflect.DeepEqual(got.Args, args) {
		t.Errorf("args = %#v, want %#v", got.Args, args)
	}
	if len(got.Env) != 1 || !strings.EqualFold(strings.SplitN(got.Env[0], "=", 2)[0], "MISA_CASE_TEST") || !strings.HasSuffix(got.Env[0], "=last") {
		t.Errorf("duplicate environment = %#v, want one last-wins entry", got.Env)
	}
	if got.Unicode != "日本語 🐈" {
		t.Errorf("Unicode environment = %q", got.Unicode)
	}
	if !strings.EqualFold(filepath.Clean(got.Cwd), filepath.Clean(dir)) {
		t.Errorf("cwd = %q, want %q", got.Cwd, dir)
	}
	nativeStatus(t, p, 23)
}

func TestNativeTransportLargeOutput(t *testing.T) {
	_, p, cap := nativeStart(t, "large", "")
	got := nativeWaitMessages(t, cap, 1)[0]
	if got.Input != "large complete" {
		t.Errorf("payload = %#v", got)
	}
	nativeStatus(t, p, 0)
	select {
	case <-cap.done:
		err := cap.readError()
		if !errors.Is(err, io.EOF) {
			t.Fatalf("output drain: %v, want EOF", err)
		}
	case <-time.After(nativeTimeout):
		t.Fatal("timed out draining output after exit")
	}
	// The numbered, short markers tolerate terminal control sequences and CRLF,
	// but detect missing, duplicated, or reordered output rather than just bytes.
	markers := regexp.MustCompile(`SEQ[0-9]{5}Z`).FindAllString(cap.text(), -1)
	if len(markers) != 10000 {
		t.Fatalf("received %d numbered markers, want 10000", len(markers))
	}
	for i, marker := range markers {
		if want := fmt.Sprintf("SEQ%05dZ", i); marker != want {
			t.Fatalf("marker %d = %q, want %q", i, marker, want)
		}
	}
}

func TestNativeTransportInputAndResize(t *testing.T) {
	c, p, cap := nativeStart(t, "interactive", "")
	first := nativeWaitMessages(t, cap, 1)[0]
	if first.Error != "" || first.Rows != 24 || first.Cols != 80 {
		t.Errorf("initial size: %+v", first)
	}
	for _, size := range [][2]int{{0, 80}, {-1, 80}, {24, 0}, {24, -1}, {32768, 80}, {24, 32768}} {
		if err := c.Resize(size[0], size[1]); err == nil {
			t.Errorf("Resize(%d, %d) succeeded", size[0], size[1])
		}
	}
	if err := c.Resize(31, 97); err != nil {
		t.Fatalf("resize: %v", err)
	}
	done := make(chan error, 1)
	go func() { _, err := io.WriteString(c, "hello from parent\r"); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("write: %v", err)
		}
	case <-time.After(nativeTimeout):
		t.Fatal("input write timed out")
	}
	// A resize makes ConPTY repaint the screen, which can replay the first
	// payload, so look for the one that carries the echoed input.
	var second nativePayload
	ended := false
	for deadline := time.Now().Add(nativeTimeout); second.Input == ""; {
		msgs := nativeWaitMessages(t, cap, 1)
		for _, m := range msgs {
			if m.Input != "" {
				second = m
			}
		}
		if second.Input == "" {
			if ended || time.Now().After(deadline) {
				t.Fatalf("no payload with input; got %+v", msgs)
			}
			select {
			case <-cap.notify:
			case <-cap.done:
				ended = true
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	if second.Error != "" || second.Rows != 31 || second.Cols != 97 || second.Input != "hello from parent" {
		t.Errorf("input/size: %+v", second)
	}
	nativeStatus(t, p, 0)
}

func TestNativeTransportCloseAndStartFunc(t *testing.T) {
	if err := New(24, 80).StartFunc(func(*os.File) int { return 0 }); !errors.Is(err, ErrUnsupported) {
		t.Errorf("StartFunc error = %v, want ErrUnsupported", err)
	}
	c, p, _ := nativeStart(t, "inspect", "")
	nativeStatus(t, p, 23)
	done := make(chan error, 1)
	go func() {
		if err := c.Close(); err != nil {
			done <- err
			return
		}
		done <- c.Close()
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Close twice: %v", err)
		}
	case <-time.After(nativeTimeout):
		t.Fatal("Close twice timed out")
	}
}

func TestNativeTransportRelativePathResolution(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin 工作")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.Open(exe)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst, err := os.Create(filepath.Join(bin, "misa-native.EXE"))
	if err != nil {
		t.Fatal(err)
	}
	_, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil {
		t.Fatal(copyErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	_, p, cap := nativeStartExe(t, "inspect", dir, "misa-native", []string{"PATH=bin 工作", "PATHEXT=.cmd;.exe"}, "path argument")
	got := nativeWaitMessages(t, cap, 1)[0]
	if !reflect.DeepEqual(got.Args, []string{"path argument"}) {
		t.Errorf("args = %#v", got.Args)
	}
	if !strings.EqualFold(filepath.Clean(got.Cwd), filepath.Clean(dir)) {
		t.Errorf("cwd = %q, want %q", got.Cwd, dir)
	}
	nativeStatus(t, p, 23)
}

func TestNativeTransportRejectsDriveRelativeDirectory(t *testing.T) {
	if err := conAvailable(); errors.Is(err, ErrUnsupported) {
		t.Skipf("ConPTY unsupported: %v", err)
	} else if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{`C:`, `C:work`} {
		c, _, err := startProcess(startSpec{Argv: []string{`unused.exe`}, Dir: dir, Rows: 24, Cols: 80})
		if c != nil {
			_ = c.Close()
		}
		if err == nil || !strings.Contains(err.Error(), "drive-relative working directory") {
			t.Errorf("Dir %q: got %v, want drive-relative working directory error", dir, err)
		}
	}
}

// ^C written to the pseudoconsole interrupts the child even when this process
// (as under a CI runner) ignores CTRL+C, and reads as status 130.
func TestNativeTransportCtrlCInterrupts(t *testing.T) {
	t.Logf("parent ignores CTRL+C: %v", ignoringCtrlC())
	c, p, cap := nativeStart(t, "ctrlc", "")
	got := nativeWaitMessages(t, cap, 1)[0]
	if got.Input != "ignore=false" {
		t.Errorf("child CTRL+C disposition: %s, want ignore=false", got.Input)
	}
	if _, err := c.Write([]byte("\x03")); err != nil {
		t.Fatal(err)
	}
	nativeStatus(t, p, 130)
}

func TestShellStatus(t *testing.T) {
	for code, want := range map[uint32]int{0: 0, 1: 1, 23: 23, statusControlCExit: 130} {
		if got := shellStatus(code); got != want {
			t.Errorf("shellStatus(%#x) = %d, want %d", code, got, want)
		}
	}
}
