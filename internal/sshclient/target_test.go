package sshclient

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSplitDestination(t *testing.T) {
	cases := []struct{ in, user, host, port string }{
		{"box", "", "box", ""},
		{"root@box", "root", "box", ""},
		{"me@corp@box", "me@corp", "box", ""},
		{"ssh://root@box:2222", "root", "box", "2222"},
		{"ssh://[::1]:22", "", "::1", "22"},
		{"ssh://box", "", "box", ""},
	}
	for _, c := range cases {
		user, host, port, err := splitDestination(c.in)
		if err != nil || user != c.user || host != c.host || port != c.port {
			t.Errorf("%q: got %q %q %q %v", c.in, user, host, port, err)
		}
	}
	for _, bad := range []string{"", "@box", "root@", "ssh://"} {
		if _, _, _, err := splitDestination(bad); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
}

func testEnv(t *testing.T, config string) env {
	t.Helper()
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".ssh"), 0o700)
	if config != "" {
		os.WriteFile(filepath.Join(home, ".ssh", "config"), []byte(config), 0o600)
	}
	return env{Home: home, LocalUser: "me"}
}

func TestResolveDefaults(t *testing.T) {
	e := testEnv(t, "")
	tg, err := resolve(Options{Destination: "box"}, e)
	if err != nil {
		t.Fatal(err)
	}
	if tg.Addr != "box:22" || tg.User != "me" || tg.Strict != "ask" {
		t.Errorf("got %+v", tg)
	}
	ssh := filepath.Join(e.Home, ".ssh")
	wantIDs := []string{filepath.Join(ssh, "id_ed25519"), filepath.Join(ssh, "id_ecdsa"), filepath.Join(ssh, "id_rsa")}
	if !reflect.DeepEqual(tg.Identities, wantIDs) {
		t.Errorf("identities = %q", tg.Identities)
	}
	if !reflect.DeepEqual(tg.KnownHosts, []string{filepath.Join(ssh, "known_hosts")}) {
		t.Errorf("known hosts = %q", tg.KnownHosts)
	}
}

func TestResolveConfigAndPrecedence(t *testing.T) {
	e := testEnv(t, "Host box\n  HostName 10.1.2.3\n  User cfg\n  Port 2022\n  IdentityFile ~/.ssh/box_%r\n")

	tg, err := resolve(Options{Destination: "box"}, e)
	if err != nil {
		t.Fatal(err)
	}
	if tg.Addr != "10.1.2.3:2022" || tg.User != "cfg" || tg.Alias != "box" {
		t.Errorf("got %+v", tg)
	}
	if !reflect.DeepEqual(tg.Identities, []string{filepath.Join(e.Home, ".ssh", "box_cfg")}) {
		t.Errorf("identities = %q", tg.Identities)
	}

	tg, _ = resolve(Options{Destination: "dest@box", Port: "1", User: "flag"}, e)
	if tg.Addr != "10.1.2.3:1" || tg.User != "flag" {
		t.Errorf("flags first: got %+v", tg)
	}
	tg, _ = resolve(Options{Destination: "dest@box", Settings: []string{"Port=7", "HostName=other"}}, e)
	if tg.Addr != "other:7" || tg.User != "dest" {
		t.Errorf("-o and destination: got %+v", tg)
	}
}

func TestResolveIPv6(t *testing.T) {
	tg, _ := resolve(Options{Destination: "ssh://[::1]:2200"}, testEnv(t, ""))
	if tg.Addr != "[::1]:2200" {
		t.Errorf("addr = %q", tg.Addr)
	}
}

func TestResolveRejectsBadPort(t *testing.T) {
	if _, err := resolve(Options{Destination: "box", Port: "http"}, testEnv(t, "")); err == nil {
		t.Error("want error")
	}
}
