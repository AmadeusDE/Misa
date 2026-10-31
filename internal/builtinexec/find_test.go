package builtinexec

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

func TestFindLauncherBuiltinFirst(t *testing.T) {
	dir := t.TempDir()
	name := "a ;\nname"
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
		t.Fatal(err)
	}
	// No PATH and no shell: exact argument bytes must reach the nested builtin.
	var out, stderr bytes.Buffer
	inv := &builtins.Context{Dir: dir, Env: []string{}, Args: []string{".", "-type", "f", "-exec", "printf", "<%s>", "{}", ";"}, Stdout: &out, Stderr: &stderr}
	if status := Run(context.Background(), "find", inv); status != 0 || stderr.Len() != 0 || out.String() != "<./"+name+">" {
		t.Fatalf("status=%d out=%q err=%q", status, out.String(), stderr.String())
	}
	if inv.Exec != nil || inv.CmdName != "" || len(inv.Env) != 0 {
		t.Fatalf("mutated invocation: %+v", inv)
	}
}

func TestFindExecLauncherHelper(t *testing.T) {
	if os.Getenv("U07_FIND_HELPER") != "1" {
		return
	}
	split := 0
	for split < len(os.Args) && os.Args[split] != "--" {
		split++
	}
	dir, err := os.Getwd()
	if err != nil {
		os.Exit(2)
	}
	fmt.Fprintf(os.Stdout, "%s|%s|%s", dir, os.Getenv("LOCAL"), strings.Join(os.Args[split+1:], "|"))
	fmt.Fprint(os.Stderr, "child-error-stream")
	os.Exit(0)
}

func TestFindLauncherExplicitPathAndExecdir(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	name := "-dash ;\nfile"
	if err := os.WriteFile(filepath.Join(dir, "sub", name), nil, 0600); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// A builtin basename must not turn an explicit path into a builtin call.
	if err := os.Symlink(exe, filepath.Join(dir, "echo")); err != nil {
		t.Skip(err)
	}
	for _, ending := range []string{";", "+"} {
		var out, stderr bytes.Buffer
		inv := &builtins.Context{Dir: dir, Env: []string{"U07_FIND_HELPER=1", "LOCAL=only-child", "PATH="}, Args: []string{"sub", "-type", "f", "-execdir", filepath.Join(dir, "echo"), "-test.run=^TestFindExecLauncherHelper$", "--", "{}", ending}, Stdout: &out, Stderr: &stderr}
		want := filepath.Join(dir, "sub") + "|only-child|./" + name
		if status := Run(context.Background(), "find", inv); status != 0 || out.String() != want || stderr.String() != "child-error-stream" {
			t.Fatalf("%s: status=%d out=%q err=%q want=%q", ending, status, out.String(), stderr.String(), want)
		}
	}
}

func TestFindLauncherCancellation(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, ending := range []string{";", "+"} {
		t.Run(ending, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reader, writer := io.Pipe()
			defer reader.Close()
			defer writer.Close()
			var stderr bytes.Buffer
			inv := &builtins.Context{Dir: t.TempDir(), Env: []string{"BUILTINEXEC_HELPER=1", "BUILTINEXEC_WAIT=1"}, Args: []string{".", "-maxdepth", "0", "-exec", exe, "-test.run=^TestHelperProcess$", "--", "{}", ending}, Stdout: writer, Stderr: &stderr}
			done := make(chan int, 1)
			go func() { done <- Run(ctx, "find", inv) }()
			ready := make(chan string, 1)
			go func() { line, _ := bufio.NewReader(reader).ReadString('\n'); ready <- line }()
			select {
			case line := <-ready:
				if line != "ready\n" {
					t.Fatalf("readiness %q", line)
				}
			case <-time.After(5 * time.Second):
				cancel()
				t.Fatal("target failed to start")
			}
			cancel()
			select {
			case status := <-done:
				if status == 0 {
					t.Fatalf("cancellation succeeded unexpectedly: %q", stderr.String())
				}
			case <-time.After(5 * time.Second):
				t.Fatal("find launcher did not cancel/reap target")
			}
		})
	}
}
