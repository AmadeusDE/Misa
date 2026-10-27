package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/AmadeusDE/Misa/internal/ughbot/config"
	"github.com/AmadeusDE/Misa/internal/ughbot/daemon"
	"github.com/AmadeusDE/Misa/internal/ughbot/webhost"
)

func serveCmd(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	path := fs.String("c", config.DefaultPath(), "config file")
	web := fs.Bool("web", false, "serve the web UI (also enabled by [web] enabled)")
	webAddr := fs.String("web-addr", "", "web UI listen address host:port (implies -web, overrides [web])")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w (run `ughbot init` to write an example)", err)
		}
		return err
	}
	if err := applyWebFlags(cfg, *web, *webAddr); err != nil {
		return err
	}
	if err := securePrivateDir(cfg.ConfigDir); err != nil {
		return err
	}
	d, err := daemon.New(daemon.Options{Config: cfg})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Fprintf(os.Stderr, "ughbot: serving %d bots on %s\n", len(cfg.Bots), cfg.Socket)
	if len(cfg.Bots) == 0 {
		fmt.Fprintln(os.Stderr, "ughbot: no bots yet; run `ughbot tui` and press p (bot-manager preset) or n on the Bots screen")
	}
	if !cfg.Web.Enabled {
		return d.Run(ctx)
	}
	host, ln, err := startWeb(cfg, *path, os.Stderr)
	if err != nil {
		_ = d.Stop(ctx)
		_ = d.Run(ctx)
		return err
	}
	d.SetWebAdmin(webhost.APIAdmin(host.Admin))
	webErr := make(chan error, 1)
	go func() {
		err := host.Serve(ctx, ln)
		if err != nil {
			_ = d.Stop(ctx)
		}
		webErr <- err
	}()
	runErr := d.Run(ctx)
	stop()
	if err := <-webErr; err != nil && runErr == nil {
		runErr = err
	}
	return runErr
}

// applyWebFlags folds -web/-web-addr into the config and revalidates it.
func applyWebFlags(cfg *config.Config, enable bool, addr string) error {
	if enable || addr != "" {
		cfg.Web.Enabled = true
	}
	if addr != "" {
		cfg.Web.ListenAddr, cfg.Web.ListenInterface = addr, ""
	}
	return cfg.Validate()
}

func startWeb(cfg *config.Config, configPath string, log io.Writer) (*webhost.Host, net.Listener, error) {
	host, err := webhost.New(webhost.Options{Config: cfg, ConfigPath: abs(configPath)})
	if err != nil {
		return nil, nil, err
	}
	ln, err := host.Listen()
	if err != nil {
		return nil, nil, err
	}
	fmt.Fprintf(log, "ughbot: web UI at http://%s/\n", ln.Addr())
	if !webhost.TOTPConfigured(host.Store) {
		fmt.Fprintln(log, "ughbot: no TOTP secret yet; run `ughbot devices totp-init` and enroll a device at /enroll")
	}
	return host, ln, nil
}

// securePrivateDir creates dir if needed and makes it 0700: bots keep Codex
// credentials there and ugh's file backend refuses any other mode.
func securePrivateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}

// abs makes the config path usable from the terminal's working directory.
func abs(path string) string {
	if a, err := filepath.Abs(path); err == nil {
		return a
	}
	return path
}
