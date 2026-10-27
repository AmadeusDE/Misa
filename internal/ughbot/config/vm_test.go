package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVMDefaults(t *testing.T) {
	c, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	v := c.VM
	if v.Image != DefaultVMImage || v.DiskGB != 40 || v.MemoryMB != 4096 || v.CPUs != 4 || v.MaxRunning != 2 || !*v.Network {
		t.Fatalf("%+v", v)
	}
	if strings.Join(v.Packages, " ") != "git base-devel go ripgrep" {
		t.Fatalf("packages = %v", v.Packages)
	}
	c, err = Parse([]byte("[vm]\nnetwork = false\npackages = []\n"))
	if err != nil || *c.VM.Network || len(c.VM.Packages) != 0 {
		t.Fatalf("explicit values must stay: %+v %v", c.VM, err)
	}
	if _, err := Parse([]byte("[vm]\nmemory_mb = 64\n")); err == nil {
		t.Fatal("tiny memory accepted")
	}
}

func TestCodeDirectoriesMustNotShareSecrets(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	for _, tc := range []struct {
		dir string
		ok  bool
	}{
		{"~/src/misa", true},
		{"/srv/repo", true},
		{"~", false},
		{"/", false},
		{"/home", false},
		{"~/.ssh/keys", false},
		{"~/.local/state", false},
		{"relative/repo", false},
	} {
		_, err := Parse([]byte("[bots.c]\nmodel = 'm'\ncapabilities = { code = ['" + tc.dir + "'] }\n"))
		if (err == nil) != tc.ok {
			t.Errorf("%s: err = %v", tc.dir, err)
		}
	}
	c, _ := Parse([]byte("[bots.c]\nmodel = 'm'\ncapabilities = { code = ['~/src/misa/'] }\n"))
	if got := c.Bots["c"].Capabilities.Code; len(got) != 1 || got[0] != "/home/u/src/misa" {
		t.Fatalf("code = %v", got)
	}
	if d, err := CodeDir(c.Bots["c"].Capabilities.Code, "~/src/misa/internal"); err != nil || d != "/home/u/src/misa" {
		t.Fatalf("CodeDir = %q, %v", d, err)
	}
	if _, err := CodeDir(c.Bots["c"].Capabilities.Code, "/home/u/src/misa-other"); err == nil {
		t.Fatal("a sibling with a common prefix is not inside")
	}
}

func TestClonesUnderReposDirMayBeShared(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	c, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.CheckCodeDir("b", c.ReposDir()+"/tool"); err != nil {
		t.Fatalf("a clone under the repos dir: %v", err)
	}
	for _, d := range []string{c.ReposDir(), c.StateDir, c.StateDir + "/vm", c.ReposDir() + "/../vm"} {
		if err := c.CheckCodeDir("b", filepath.Clean(d)); err == nil {
			t.Errorf("%s accepted", d)
		}
	}
}
