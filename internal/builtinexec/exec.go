// Package builtinexec composes the builtin registry with external execution.
package builtinexec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

// For selects the default nested executor for a builtin.
func For(name string) builtins.Executor {
	return Run
}

// Run executes an exact builtin name or an external command with invocation-local
// streams, directory and environment. Builtin dispatch selects the default
// nested executor with For.
func Run(ctx context.Context, name string, inv *builtins.Context) int {
	if ctx == nil {
		ctx = inv.CancellationContext()
	}
	if err := ctx.Err(); err != nil {
		return builtins.ExitStatus(err)
	}
	if inv == nil {
		inv = &builtins.Context{}
	}
	local := *inv
	if local.Exec == nil {
		local.Exec = For(name)
	}
	if _, ok := builtins.Lookup(name); ok && !strings.ContainsAny(name, `/\`) {
		local.Exec = For(name)
		return builtins.Run(ctx, name, &local)
	}
	dir, err := filepath.Abs(local.Dir)
	if err != nil {
		return builtins.ExitInternal
	}
	local.Dir = dir
	path, err := resolve(name, &local)
	if err != nil {
		if local.Stderr != nil {
			fmt.Fprintf(local.Stderr, "%s: %v\n", name, err)
		}
		if errors.Is(err, os.ErrNotExist) {
			return builtins.ExitNotFound
		}
		return builtins.ExitCannotExecute
	}
	cmd := exec.CommandContext(ctx, path, local.Args...)
	cmd.Args[0] = name
	if local.CmdName != "" {
		cmd.Args[0] = local.CmdName
	}
	cmd.Dir, cmd.Env = local.Dir, local.Env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = local.Stdin, local.Stdout, local.Stderr
	err = cmd.Run()
	if ctx.Err() != nil {
		return builtins.ExitStatus(ctx.Err())
	}
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			if local.Stderr != nil {
				fmt.Fprintf(local.Stderr, "%s: %v\n", name, err)
			}
			if errors.Is(err, os.ErrNotExist) {
				return builtins.ExitNotFound
			}
			return builtins.ExitCannotExecute
		}
	}
	return builtins.ExitStatus(err)
}

func resolve(name string, inv *builtins.Context) (string, error) {
	if name == "" {
		return "", os.ErrNotExist
	}
	if strings.ContainsAny(name, `/\`) {
		if !filepath.IsAbs(name) {
			return filepath.Join(inv.Dir, name), nil
		}
		return name, nil
	}
	path, ok := inv.LookupEnv("PATH")
	if !ok {
		return "", os.ErrNotExist
	}
	var denied bool
	dirs := filepath.SplitList(path)
	if path == "" {
		dirs = []string{""}
	}
	for _, dir := range dirs {
		if dir == "" {
			dir = "."
		}
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(inv.Dir, dir)
		}
		candidate := filepath.Join(dir, name)
		info, err := os.Stat(candidate)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				denied = true
			}
			continue
		}
		if info.IsDir() || info.Mode().Perm()&0111 == 0 {
			denied = true
			continue
		}
		return candidate, nil
	}
	if denied {
		return "", os.ErrPermission
	}
	return "", os.ErrNotExist
}
