// Package mweb serves Misa in a browser: a webui.Server whose terminal runs
// misa itself. Kept apart from internal/ui, which never imports webui.
package mweb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/webadmin"
	"github.com/AmadeusDE/Misa/internal/webui"
)

const title = "misa"

// Run serves the web UI until interrupted; args are the flags after --web.
func Run(args []string, stderr io.Writer) int {
	// Run is a process-level CLI entrypoint, not a builtin invocation.
	if os.Getenv(ChildEnv) != "" {
		fmt.Fprintln(stderr, "misa: --web refused inside a web terminal")
		return 1
	}
	o, err := ParseFlags(args, config.Load().Web)
	if err != nil {
		fmt.Fprintln(stderr, "misa --web:", err)
		return 2
	}
	if err := serve(o, stderr); err != nil {
		fmt.Fprintln(stderr, "misa --web:", err)
		return 1
	}
	return 0
}

func serve(o Options, stderr io.Writer) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	store, err := openStore()
	if err != nil {
		return err
	}
	srv, err := webui.New(webui.Options{
		Store: store, Title: title,
		ListenInterface: o.Interface, ListenAddr: o.Addr, Port: o.Port,
		AuthorizedKeys: authorizedKeysPath(),
	})
	if err != nil {
		return err
	}
	mount(srv, o.Argv(exe))
	ln, err := srv.Listen()
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "misa: web UI at http://%s/ (a login is a shell on this host)\n", ln.Addr())
	if !totpConfigured(store) {
		fmt.Fprintln(stderr, "misa: no TOTP secret yet; run `misa --web-admin totp-init` and enroll a device at /enroll")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return srv.ServeListener(ctx, ln)
}

func mount(srv *webui.Server, argv []string) {
	dir, _ := os.UserHomeDir()
	srv.MountTerminal("/term/", webui.SpawnArgv(argv, dir, []string{ChildEnv + "=1"}))
	srv.Mount("GET /{$}", http.RedirectHandler("/term/", http.StatusSeeOther))
}

func openStore() (*webui.FileStore, error) {
	return openStoreAt(StateDir(), ConfigDir())
}

func openStoreAt(stateDir, configDir string) (*webui.FileStore, error) {
	if err := securePrivateDir(configDir); err != nil {
		return nil, err
	}
	if err := securePrivateDir(stateDir); err != nil {
		return nil, err
	}
	return webui.NewFileStore(stateDir)
}

func totpConfigured(store webui.DeviceStore) bool {
	set := false
	_ = store.View(func(st *webui.State) { set = st.TOTP.Secret != "" })
	return set
}

const adminUsage = `usage: misa --web-admin <command>

  totp-init | totp-reset
  devices list | revoke <id|name> | rename <id|name> <new> | enroll [-name n] <url>
  sessions list | kill <prefix>
`

// Admin runs one --web-admin command against the device store; it works with or without a server running.
func Admin(args []string, out io.Writer, in io.Reader) int {
	if err := admin(args, out, in); err != nil {
		fmt.Fprintln(os.Stderr, "misa --web-admin:", err)
		return 1
	}
	return 0
}

func admin(args []string, out io.Writer, in io.Reader) error {
	if len(args) == 0 {
		fmt.Fprint(out, adminUsage)
		return errors.New("missing command")
	}
	store, err := openStore()
	if err != nil {
		return err
	}
	env := webadmin.Env{
		Admin: webui.NewAdmin(store, nil), Out: out, In: in,
		Program: "misa --web-admin devices", Issuer: title, KeyPath: deviceKeyPath(),
	}
	switch args[0] {
	case "totp-init", "totp-reset":
		return webadmin.Devices(env, args)
	case "devices":
		return webadmin.Devices(env, args[1:])
	case "sessions":
		return webadmin.Sessions(env, args[1:])
	}
	fmt.Fprint(out, adminUsage)
	return fmt.Errorf("unknown command %q", args[0])
}
