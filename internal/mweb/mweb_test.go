package mweb

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/AmadeusDE/Misa/internal/config"
)

func TestParseFlags(t *testing.T) {
	cfg := config.WebConfig{Interface: "wg0", Port: 9000, Command: []string{"misa", "-x"}}
	cases := []struct {
		name string
		args []string
		cfg  config.WebConfig
		want Options
		err  string
	}{
		{"config only", nil, cfg, Options{Interface: "wg0", Port: 9000, Command: cfg.Command}, ""},
		{"addr replaces configured interface", []string{"--web-addr", "127.0.0.1:1"}, cfg, Options{Addr: "127.0.0.1:1", Port: 9000, Command: cfg.Command}, ""},
		{"interface replaces configured addr", []string{"--web-interface", "wg1", "--web-port", "7"}, config.WebConfig{Addr: "127.0.0.1:1"}, Options{Interface: "wg1", Port: 7}, ""},
		{"loopback by default", nil, config.WebConfig{}, Options{Addr: "127.0.0.1:8787"}, ""},
		{"loopback on the given port", []string{"--web-port", "9000"}, config.WebConfig{}, Options{Addr: "127.0.0.1:9000", Port: 9000}, ""},
		{"stray argument", []string{"--web-addr", "x:1", "oops"}, config.WebConfig{}, Options{}, "unexpected argument"},
		{"unknown flag", []string{"--bogus"}, cfg, Options{}, "bogus"},
		{"port range", []string{"--web-port", "70000"}, cfg, Options{}, "port out of range"},
		{"blank command", nil, config.WebConfig{Addr: "x:1", Command: []string{" "}}, Options{}, "must name a program"},
	}
	for _, tc := range cases {
		got, err := ParseFlags(tc.args, tc.cfg)
		if tc.err != "" {
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("%s: err = %v, want %q", tc.name, err, tc.err)
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %+v %v, want %+v", tc.name, got, err, tc.want)
		}
	}
}

func TestArgvRunsMisaUnlessCommandSet(t *testing.T) {
	if got := (Options{}).Argv("/bin/misa"); !reflect.DeepEqual(got, []string{"/bin/misa"}) {
		t.Fatalf("default argv = %v", got)
	}
	if got := (Options{Command: []string{"zsh", "-l"}}).Argv("/bin/misa"); !reflect.DeepEqual(got, []string{"zsh", "-l"}) {
		t.Fatalf("override argv = %v", got)
	}
}

func TestDirsFollowXDG(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/x/state")
	t.Setenv("XDG_CONFIG_HOME", "/x/config")
	if StateDir() != "/x/state/misa/webui" || ConfigDir() != "/x/config/misa" {
		t.Fatalf("state %q config %q", StateDir(), ConfigDir())
	}
	if authorizedKeysPath() != "/x/config/misa/authorized_keys" || deviceKeyPath() != "/x/config/misa/web-device.key" {
		t.Fatalf("keys %q %q", authorizedKeysPath(), deviceKeyPath())
	}
}

func TestRunRefusesInsideWebTerminal(t *testing.T) {
	t.Setenv(ChildEnv, "1")
	var errOut bytes.Buffer
	if Run([]string{"--web-addr", "127.0.0.1:0"}, &errOut) == 0 || !strings.Contains(errOut.String(), "refused") {
		t.Fatalf("recursion not refused: %q", errOut.String())
	}
}

func TestAdminTOTPAndDevices(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	var out bytes.Buffer
	if err := admin([]string{"totp-init"}, &out, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "otpauth://totp/misa") || !strings.Contains(out.String(), "secret (base32)") {
		t.Fatalf("totp-init output: %q", out.String())
	}
	if admin([]string{"totp-init"}, &bytes.Buffer{}, nil) == nil {
		t.Fatal("second totp-init must not replace the secret")
	}
	out.Reset()
	if err := admin([]string{"devices", "list"}, &out, nil); err != nil || !strings.HasPrefix(out.String(), "ID") {
		t.Fatalf("devices list: %v %q", err, out.String())
	}
	out.Reset()
	if err := admin([]string{"sessions", "list"}, &out, nil); err != nil || !strings.HasPrefix(out.String(), "SESSION") {
		t.Fatalf("sessions list: %v %q", err, out.String())
	}
	if admin([]string{"bogus"}, &bytes.Buffer{}, nil) == nil || admin(nil, &bytes.Buffer{}, nil) == nil {
		t.Fatal("unknown or missing commands must fail")
	}
	for _, dir := range []string{ConfigDir(), StateDir()} {
		if fi, err := os.Stat(dir); err != nil || fi.Mode().Perm() != 0o700 {
			t.Errorf("%s: %v %v", dir, fi, err)
		}
	}
}

func TestDevicesBuiltinRegisteredAndRunsCLISubcommands(t *testing.T) {
	cmd, ok := builtins.Lookup("devices")
	if !ok {
		t.Fatal("devices builtin not registered")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	var out, errb bytes.Buffer
	if code := cmd.Run(&builtins.Context{Args: []string{"list"}, Stdout: &out, Stderr: &errb}); code != 0 {
		t.Fatalf("devices list = %d: %s", code, errb.String())
	}
}

func TestDevicesInvocationEnvironmentIsolation(t *testing.T) {
	root := t.TempDir()
	process := filepath.Join(root, "process")
	t.Setenv("HOME", process)
	t.Setenv("XDG_STATE_HOME", filepath.Join(process, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(process, "config"))
	configBase := func(home, xdg string) string {
		switch runtime.GOOS {
		case "darwin", "ios":
			return filepath.Join(home, "Library", "Application Support", "misa")
		case "windows":
			return "."
		case "plan9":
			return filepath.Join(home, "lib", "misa")
		default:
			if xdg != "" {
				return filepath.Join(xdg, "misa")
			}
			return filepath.Join(home, ".config", "misa")
		}
	}
	for _, tc := range []struct {
		name          string
		env           []string
		state, config string
	}{
		{"xdg", []string{"HOME=" + filepath.Join(root, "home"), "XDG_STATE_HOME=" + filepath.Join(root, "state"), "XDG_CONFIG_HOME=" + filepath.Join(root, "config")}, filepath.Join(root, "state", "misa", "webui"), configBase(filepath.Join(root, "home"), filepath.Join(root, "config"))},
		{"home", []string{"HOME=" + filepath.Join(root, "home2")}, filepath.Join(root, "home2", ".local", "state", "misa", "webui"), configBase(filepath.Join(root, "home2"), "")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			code := builtins.Run(nil, "devices", &builtins.Context{Args: []string{"list"}, Env: tc.env, Stdout: &out, Stderr: &errb})
			if code != 0 {
				t.Fatalf("exit %d: %s", code, &errb)
			}
			for _, dir := range []string{tc.state, tc.config} {
				if _, err := os.Stat(dir); err != nil {
					t.Errorf("%s: %v", dir, err)
				}
			}
		})
	}
	if _, err := os.Stat(process); !os.IsNotExist(err) {
		t.Fatalf("process environment used: %v", err)
	}
}
