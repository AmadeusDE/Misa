package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/AmadeusDE/Misa/internal/shell/shcli"
	"golang.org/x/term"
)

func runShell(name string, args []string) int {
	ctx, stop := standaloneBuiltinContext("sh")
	defer stop()
	return shcli.Run(ctx, name, args, shcli.Options{
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
		StdinTTY: term.IsTerminal(int(os.Stdin.Fd())),
	})
}

// shellInstallTarget resolves the running binary even when argv0 is a bare name
// found on PATH rather than a usable path.
func shellInstallTarget(self string) (string, error) {
	if strings.ContainsAny(self, `/:\\`) {
		if path, err := filepath.Abs(self); err == nil {
			if resolved, err := filepath.EvalSymlinks(path); err == nil {
				return resolved, nil
			}
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved, nil
	}
	return exe, nil
}

// Shell aliases are opt-in and never part of the builtin installation set.
func installShellLinks(self string, args []string, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: misa --install-sh <directory>")
		return 1
	}
	fail := func(err error) int {
		fmt.Fprintln(stderr, "misa: --install-sh:", err)
		return 1
	}
	exe, err := shellInstallTarget(self)
	if err != nil {
		return fail(err)
	}
	dir := args[0]
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fail(err)
	}
	names := []string{"sh", "misa-sh"}
	for _, name := range names {
		if fi, err := os.Lstat(filepath.Join(dir, name)); err == nil && fi.Mode()&os.ModeSymlink == 0 {
			return fail(fmt.Errorf("%s: exists and is not a symlink", filepath.Join(dir, name)))
		}
	}
	status := 0
	for _, name := range names {
		link := filepath.Join(dir, name)
		if fi, err := os.Lstat(link); err == nil && fi.Mode()&os.ModeSymlink != 0 {
			if target, err := os.Readlink(link); err == nil && target == exe {
				continue
			}
			if err := os.Remove(link); err != nil {
				status = fail(fmt.Errorf("%s: %w", link, err))
				continue
			}
		}
		if err := os.Symlink(exe, link); err != nil {
			status = fail(fmt.Errorf("%s: %w", link, err))
			continue
		}
		fmt.Fprintf(stderr, "%s -> %s\n", link, exe)
	}
	return status
}
