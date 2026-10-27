package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/config"
	"github.com/AmadeusDE/Misa/internal/ughbot/webhost"
	"github.com/AmadeusDE/Misa/internal/webui"
)

func devicesConfig(t *testing.T) (string, *config.Config) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ughbot.toml")
	writeFile(t, path, `state_dir = "`+filepath.Join(dir, "state")+`"
socket = "`+filepath.Join(dir, "u.sock")+`"
[web]
enabled = true
listen_addr = "127.0.0.1:0"
[bots.a]
model = "m"
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, cfg
}

func TestApplyWebFlags(t *testing.T) {
	cfg, err := config.Parse([]byte("[bots.a]\nmodel='m'\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := applyWebFlags(cfg, true, ""); err != nil || !cfg.Web.Enabled {
		t.Fatalf("-web alone must enable the default listener: %v", err)
	}
	if err := applyWebFlags(cfg, false, "127.0.0.1:9"); err != nil || !cfg.Web.Enabled || cfg.Web.ListenAddr != "127.0.0.1:9" {
		t.Fatalf("-web-addr: %v %+v", err, cfg.Web)
	}
	cfg.Web.ListenInterface = "wg0"
	if err := applyWebFlags(cfg, false, "127.0.0.1:1"); err != nil || cfg.Web.ListenInterface != "" {
		t.Fatalf("-web-addr should override the interface: %+v", cfg.Web)
	}
}

func TestDevicesLifecycleThroughTheCLI(t *testing.T) {
	cfgPath, cfg := devicesConfig(t)
	run := func(in string, args ...string) string {
		t.Helper()
		var out bytes.Buffer
		if err := devicesCmd(append([]string{"-c", cfgPath}, args...), &out, strings.NewReader(in)); err != nil {
			t.Fatalf("devices %v: %v\n%s", args, err, out.String())
		}
		return out.String()
	}

	init := run("", "totp-init")
	m := regexp.MustCompile(`secret \(base32\): (\S+)`).FindStringSubmatch(init)
	if m == nil || !strings.Contains(init, "otpauth://") {
		t.Fatalf("totp-init output = %q", init)
	}
	if err := devicesCmd([]string{"-c", cfgPath, "totp-init"}, &bytes.Buffer{}, nil); err == nil {
		t.Fatal("second totp-init must refuse to replace the secret")
	}

	// a later period than the init counter, as it would be once the user waits
	store, _ := webhost.OpenStore(cfg)
	past := time.Now().Add(-2 * time.Minute)
	if _, _, err := webui.NewAdmin(store, func() time.Time { return past }).TOTPReset("ughbot", "ughbot"); err != nil {
		t.Fatal(err)
	}
	var secret string
	_ = store.View(func(st *webui.State) { secret = st.TOTP.Secret })

	host, err := webhost.New(webhost.Options{Config: cfg, Logf: func(string, ...any) {}})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := host.Listen()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- host.Serve(ctx, ln) }()
	defer func() { cancel(); <-done }()

	code, _ := webui.TOTPCode(secret, time.Now())
	out := run(code+"\n", "enroll", "-name", "cli-box", "http://"+ln.Addr().String())
	if !strings.Contains(out, "enrolled") {
		t.Fatalf("enroll output = %q", out)
	}
	if list := run("", "list"); !strings.Contains(list, "cli-box") || !strings.Contains(list, "active") {
		t.Fatalf("list = %q", list)
	}
	var sessions bytes.Buffer
	if err := sessionsCmd([]string{"-c", cfgPath, "list"}, &sessions); err != nil || !strings.Contains(sessions.String(), "cli-box") {
		t.Fatalf("sessions = %q err=%v", sessions.String(), err)
	}
	run("", "rename", "cli-box", "laptop")
	run("", "revoke", "laptop")
	if list := run("", "list"); !strings.Contains(list, "laptop") || !strings.Contains(list, "revoked") {
		t.Fatalf("list after revoke = %q", list)
	}
	var after bytes.Buffer
	_ = sessionsCmd([]string{"-c", cfgPath, "list"}, &after)
	if strings.Contains(after.String(), "laptop") {
		t.Fatalf("revoke left a session: %q", after.String())
	}
	if err := devicesCmd([]string{"-c", cfgPath, "revoke", "nobody"}, &bytes.Buffer{}, nil); err == nil {
		t.Fatal("revoking an unknown device must fail")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
