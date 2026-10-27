// Package webhost wires the generic web shell for the daemon process: login
// and enroll pages plus a browser terminal running the ughbot TUI.
package webhost

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/config"
	"github.com/AmadeusDE/Misa/internal/webui"
)

const (
	title      = "ughbot"
	terminalAt = "/term/"
)

// DeviceDir is where the device store (webui.json) lives.
func DeviceDir(cfg *config.Config) string { return filepath.Join(cfg.StateDir, "webui") }

// AuthorizedKeysPath is the authorized_keys-style file whose ssh-ed25519 keys may log in.
func AuthorizedKeysPath(cfg *config.Config) string {
	return filepath.Join(cfg.ConfigDir, "authorized_keys")
}

// KeyPath is the CLI device key used by `ughbot devices enroll`.
func KeyPath(cfg *config.Config) string { return filepath.Join(cfg.ConfigDir, "web-device.key") }

// OpenStore opens the device store the daemon uses; safe alongside a running daemon.
func OpenStore(cfg *config.Config) (*webui.FileStore, error) {
	return webui.NewFileStore(DeviceDir(cfg))
}

// TOTPConfigured reports whether enrollment has been initialised.
func TOTPConfigured(store webui.DeviceStore) bool {
	set := false
	_ = store.View(func(st *webui.State) { set = st.TOTP.Secret != "" })
	return set
}

type Host struct {
	Server *webui.Server
	Store  *webui.FileStore
	Admin  *webui.Admin
}

type Options struct {
	Config *config.Config
	// ConfigPath is passed to the TUI so it finds the same config and socket.
	ConfigPath string
	// Executable is the ughbot binary the TUI runs from; default os.Executable().
	Executable string
	Logf       func(format string, args ...any)
	Now        func() time.Time // tests
}

// TerminalArgv is the program the browser terminal runs: [web] command, else
// the ughbot TUI talking to this daemon's socket.
func TerminalArgv(cfg *config.Config, exe, configPath string) []string {
	if len(cfg.Web.Command) > 0 {
		return cfg.Web.Command
	}
	argv := []string{exe, "tui"}
	if configPath != "" {
		argv = append(argv, "-c", configPath)
	}
	return append(argv, "-s", cfg.Socket)
}

// New builds the shell from cfg.Web and mounts the terminal on it.
func New(o Options) (*Host, error) {
	store, err := OpenStore(o.Config)
	if err != nil {
		return nil, err
	}
	w := o.Config.Web
	srv, err := webui.New(webui.Options{
		Store: store, Title: title,
		ListenInterface: w.ListenInterface, ListenAddr: w.ListenAddr, Port: w.Port,
		AuthorizedKeys: AuthorizedKeysPath(o.Config), Logf: o.Logf, Now: o.Now,
	})
	if err != nil {
		return nil, err
	}
	exe := o.Executable
	if exe == "" {
		if exe, err = os.Executable(); err != nil {
			return nil, err
		}
	}
	dir, _ := os.UserHomeDir()
	argv := TerminalArgv(o.Config, exe, o.ConfigPath)
	srv.MountTerminal(terminalAt, webui.SpawnArgv(argv, dir, nil))
	srv.Mount("GET /{$}", http.RedirectHandler(terminalAt, http.StatusSeeOther))
	return &Host{Server: srv, Store: store, Admin: webui.NewAdmin(store, o.Now)}, nil
}

func (h *Host) Listen() (net.Listener, error) { return h.Server.Listen() }

func (h *Host) Serve(ctx context.Context, ln net.Listener) error {
	return h.Server.ServeListener(ctx, ln)
}
