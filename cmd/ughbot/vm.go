package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/config"
	"github.com/AmadeusDE/Misa/internal/ughbot/sandbox"
	"github.com/AmadeusDE/Misa/internal/ughbot/vm"
)

const vmUsage = `usage: ughbot vm [-c config] <command>

  prepare      download the image and build the base every code VM starts from
  status       whether a base is prepared, and from what
  try [dir]    boot a throwaway VM (sharing dir), run a few checks in it, discard it
`

func vmCmd(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("vm", flag.ContinueOnError)
	cfgPath := fs.String("c", config.DefaultPath(), "config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := loadOrDefault(*cfgPath)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	m := vm.New(cfg, out)
	switch sub := fs.Arg(0); sub {
	case "prepare":
		return m.Prepare(ctx, out)
	case "status":
		return vmStatus(m, out)
	case "try":
		return vmTry(ctx, m, fs.Arg(1), out)
	default:
		fmt.Fprint(os.Stderr, vmUsage)
		return fmt.Errorf("unknown vm command %q", sub)
	}
}

// loadOrDefault reads the config, or uses the defaults when there is no file yet.
func loadOrDefault(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if errors.Is(err, fs.ErrNotExist) {
		return config.Parse(nil)
	}
	return cfg, err
}

func vmStatus(m *vm.Manager, out io.Writer) error {
	s := m.Status()
	if !s.Prepared {
		fmt.Fprintln(out, "no base prepared: run `ughbot vm prepare`")
		return nil
	}
	fmt.Fprintf(out, "base prepared %s from %s\npackages: %s\n", s.Base.At.Format(time.DateTime), s.Base.Image, strings.Join(s.Base.Packages, " "))
	for _, c := range s.Base.Setup {
		fmt.Fprintln(out, "setup:", c)
	}
	fmt.Fprintf(out, "running in this process: %d of at most %d\n", len(s.Running), s.Max)
	return nil
}

func vmTry(ctx context.Context, m *vm.Manager, dir string, out io.Writer) error {
	var shares []string
	if dir != "" {
		shares = []string{config.ExpandHome(dir)}
	}
	began := time.Now()
	mc, err := m.Start(ctx, shares)
	if err != nil {
		return err
	}
	defer mc.Close()
	fmt.Fprintf(out, "booted %s in %s\n", mc.Describe(), time.Since(began).Round(100*time.Millisecond))
	cwd := "/tmp"
	if dir != "" {
		cwd = shares[0]
	}
	checks := []string{"uname -sr; id; df -h / | tail -1", "git --version; go version || true"}
	if dir != "" {
		checks = append(checks, "pwd; ls | head -5; git status --short | head -5 || true")
	}
	for _, c := range checks {
		raw, _ := json.Marshal(map[string]string{"command": c})
		res, err := mc.RunTool(ctx, "bash", cwd, raw)
		if err != nil {
			return fmt.Errorf("%s: %w", c, err)
		}
		fmt.Fprintf(out, "$ %s\n%s\n", c, strings.TrimSpace(res))
	}
	fmt.Fprintln(out, "discarding the VM")
	return nil
}

// vmAgentCmd is the helper the daemon starts inside each VM: tool calls on stdin, results on stdout.
func vmAgentCmd() error {
	return sandbox.Serve(context.Background(), os.Stdin, os.Stdout)
}
