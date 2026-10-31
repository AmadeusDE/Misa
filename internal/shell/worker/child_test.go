//go:build unix

package worker

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestProcessStubbornHelper(t *testing.T) {
	if os.Getenv("MISA_STUBBORN") != "1" {
		return
	}
	f := os.NewFile(3, "control")
	_, err := readFrame(f)
	if err != nil {
		os.Exit(2)
	}
	if writeFrame(f, frame{Type: "hello", Version: Version, Capabilities: []string{"run", "cancel", "shutdown"}}) != nil {
		os.Exit(2)
	}
	_, _ = readFrame(f)
	time.Sleep(10 * time.Second)
	os.Exit(0)
}

func TestProcessShutdownBounded(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	c, err := StartProcess(context.Background(), ProcessOptions{Executable: exe, Args: []string{"-test.run=^TestProcessStubbornHelper$"}, Env: []string{"MISA_STUBBORN=1"}, ShutdownTimeout: 100 * time.Millisecond})
	if errors.Is(err, os.ErrPermission) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	err = c.Shutdown(context.Background())
	if err == nil {
		t.Fatal("expected timeout")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("shutdown not bounded")
	}
}

func TestProcessNoHandshakeHelper(t *testing.T) {
	if os.Getenv("MISA_NO_HANDSHAKE") != "1" {
		return
	}
	time.Sleep(10 * time.Second)
	os.Exit(0)
}
func TestProcessHandshakeTimeout(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = StartProcess(ctx, ProcessOptions{Executable: exe, Args: []string{"-test.run=^TestProcessNoHandshakeHelper$"}, Env: []string{"MISA_NO_HANDSHAKE=1"}})
	if errors.Is(err, os.ErrPermission) {
		t.Skip(err)
	}
	if err == nil {
		t.Fatal("expected timeout")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("startup not bounded")
	}
}

func TestProcessTerminalFrameHelper(t *testing.T) {
	if os.Getenv("MISA_TERMINAL_FIXTURE") == "" {
		return
	}
	f := os.NewFile(3, "control")
	if _, err := readFrame(f); err != nil {
		os.Exit(2)
	}
	if err := writeFrame(f, frame{Type: "hello", Version: Version}); err != nil {
		os.Exit(2)
	}
	if _, err := readFrame(f); err != nil {
		os.Exit(2)
	}
	if os.Getenv("MISA_TERMINAL_FIXTURE") == "crash" {
		_ = writeFrame(f, frame{Type: "crash", Status: 125})
		os.Exit(42)
	}
	_ = writeFrame(f, frame{Type: "exit"})
	if os.Getenv("MISA_TERMINAL_FIXTURE") == "exit-nonzero" {
		os.Exit(42)
	}
	os.Exit(0)
}
func TestProcessTerminalFrames(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	for _, mode := range []string{"exit", "crash", "exit-nonzero"} {
		t.Run(mode, func(t *testing.T) {
			for i := 0; i < 20; i++ {
				c, err := StartProcess(context.Background(), ProcessOptions{Executable: exe, Args: []string{"-test.run=^TestProcessTerminalFrameHelper$"}, Env: []string{"MISA_TERMINAL_FIXTURE=" + mode}, ShutdownTimeout: 3 * time.Second})
				if errors.Is(err, os.ErrPermission) {
					t.Skip(err)
				}
				if err != nil {
					t.Fatal(err)
				}
				if mode == "crash" {
					_, _ = c.Run(context.Background(), "anything")
				} else {
					_ = c.Shutdown(context.Background())
				}
				// Shutdown waits for the process monitor, including any refinement.
				_ = c.Shutdown(context.Background())
				err = c.Err()
				var exit *ExitError
				switch mode {
				case "exit":
					if err != nil {
						t.Fatalf("iteration %d: %v", i, err)
					}
				case "crash":
					if !errors.As(err, &exit) || !exit.Crash {
						t.Fatalf("iteration %d: %v", i, err)
					}
				case "exit-nonzero":
					if !errors.As(err, &exit) || exit.Status != 42 {
						t.Fatalf("iteration %d: %v", i, err)
					}
				}
			}
		})
	}
}
