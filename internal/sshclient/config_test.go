package sshclient

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const sampleConfig = `
# global
Port 2200

Host web *.example.org !secret.example.org
    HostName %h.internal
    User deploy
    IdentityFile ~/.ssh/web_key

Host secret.example.org
    User spy

Match host web
    User nobody

Host *
    User fallback
    Port=22
    IdentityFile "%d/.ssh/id_%r"
    StrictHostKeyChecking accept-new
`

func parseSample(t *testing.T, host string) settings {
	t.Helper()
	var s settings
	parseConfig(strings.NewReader(sampleConfig), host, &s, nil)
	return s
}

func TestConfigFirstValueWins(t *testing.T) {
	s := parseSample(t, "web")
	if s.Port != "2200" || s.User != "deploy" || s.HostName != "%h.internal" {
		t.Errorf("got %+v", s)
	}
	if s.StrictHostKeyChecking != "accept-new" {
		t.Errorf("strict = %q", s.StrictHostKeyChecking)
	}
}

func TestConfigIdentityFilesAccumulate(t *testing.T) {
	s := parseSample(t, "web")
	want := []string{"~/.ssh/web_key", "%d/.ssh/id_%r"}
	if !reflect.DeepEqual(s.IdentityFiles, want) {
		t.Errorf("identities = %q, want %q", s.IdentityFiles, want)
	}
}

func TestConfigWildcardAndNegation(t *testing.T) {
	if s := parseSample(t, "a.example.org"); s.User != "deploy" {
		t.Errorf("wildcard user = %q", s.User)
	}
	if s := parseSample(t, "secret.example.org"); s.User != "spy" {
		t.Errorf("negated host user = %q", s.User)
	}
	if s := parseSample(t, "WEB"); s.User != "deploy" {
		t.Errorf("hosts match case-insensitively, user = %q", s.User)
	}
}

func TestConfigSkipsMatchBlocks(t *testing.T) {
	if s := parseSample(t, "other"); s.User != "fallback" {
		t.Errorf("user = %q", s.User)
	}
}

func TestConfigCommandLineGoesFirst(t *testing.T) {
	s := settings{}
	s.set("user", "cli")
	parseConfig(strings.NewReader(sampleConfig), "web", &s, nil)
	if s.User != "cli" {
		t.Errorf("user = %q", s.User)
	}
}

func TestConfigInclude(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "extra"), []byte("Host box\n  HostName 10.0.0.9\n"), 0o600)
	main := filepath.Join(dir, "config")
	os.WriteFile(main, []byte("Include extra\nHost *\n  HostName never\n"), 0o600)

	var s settings
	readConfig(main, "box", &s, dir, 0)
	if s.HostName != "10.0.0.9" {
		t.Errorf("hostname = %q", s.HostName)
	}
}

func TestExpandTokens(t *testing.T) {
	got := expandTokens("%d/.ssh/id_%r@%h %%", map[byte]string{'d': "/home/u", 'r': "root", 'h': "box"})
	if got != "/home/u/.ssh/id_root@box %" {
		t.Errorf("got %q", got)
	}
}
