package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExampleShipsNoBots(t *testing.T) {
	c, err := Parse(ExampleConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Bots) != 0 {
		t.Fatalf("the example must not define bots, got %v", c.BotNames())
	}
	entries, _ := fs.ReadDir(Example(), ".")
	for _, e := range entries {
		if e.Name() != "ughbot.toml" {
			t.Errorf("unexpected shipped file %q", e.Name())
		}
	}
}

func TestDefaultsAndEnv(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/7")
	t.Setenv("XDG_STATE_HOME", "/st")
	t.Setenv("XDG_CONFIG_HOME", "/cf")
	c, err := Parse([]byte("[bots.a]\nmodel='m'\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Socket != "/run/user/7/ughbot.sock" || c.StateDir != "/st/ughbot" || c.ConfigDir != "/cf/ughbot" {
		t.Fatalf("%+v", c)
	}
	if c.DBPath() != "/st/ughbot/ughbot.db" || c.SessionDir("a") != "/st/ughbot/bots/a" {
		t.Fatal(c.DBPath())
	}
}

func TestValidationErrors(t *testing.T) {
	cases := map[string]string{
		"missing model":   "[bots.a]\n",
		"bad files":       "[bots.a]\nmodel='m'\ncapabilities={files='all'}\n",
		"negative tasks":  "[bots.a]\nmodel='m'\ncapabilities={tasks=-1}\n",
		"bad bot entry":   "[bots.a]\nmodel='m'\ncapabilities={manage=['Not A Bot']}\n",
		"old grants":      "[bots.a]\nmodel='m'\ngrants={message=['*']}\n",
		"negative spend":  "[bots.a]\nmodel='m'\nspend={daily=-1}\n",
		"daily>monthly":   "[bots.a]\nmodel='m'\nspend={daily=5, monthly=1}\n",
		"old tools":       "[bots.a]\nmodel='m'\ntools=['read']\n",
		"duplicate entry": "[bots.a]\nmodel='m'\ncapabilities={message=['*','*']}\n",
		"bad name":        "[bots.'Bad Name']\nmodel='m'\n",
		"unknown key":     "[bots.a]\nmodel='m'\nmodle='x'\n",
	}
	for name, src := range cases {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	_, err := Parse([]byte("[bots.a]\ncapabilities={files='x', tasks=-1}\n"))
	if err == nil || !strings.Contains(err.Error(), `model is required`) || !strings.Contains(err.Error(), `tasks must not be negative`) {
		t.Fatalf("errors should accumulate: %v", err)
	}
}

func TestReferencesCheckedOnWriteNotLoad(t *testing.T) {
	c, err := Parse([]byte("[bots.a]\nmodel='m'\ncapabilities={message=['gone']}\n"))
	if err != nil {
		t.Fatalf("a dangling reference must not stop loading: %v", err)
	}
	if err := c.CheckReferences("a"); err == nil || !strings.Contains(err.Error(), `unknown bot "gone"`) {
		t.Fatalf("CheckReferences: %v", err)
	}
	if _, err := c.With(Bot{Name: "b", Model: "m", Capabilities: Capabilities{Manage: []string{"ghost"}}}, false); err == nil {
		t.Fatal("With must refuse a new bot naming an unknown one")
	}
	if _, err := c.With(Bot{Name: "a"}, true); err != nil {
		t.Fatalf("removing is always fine: %v", err)
	}
}

func TestLoadResolvesPromptsRelativeToFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ughbot.toml")
	os.WriteFile(p, []byte("[bots.a]\nmodel='m'\nprompt='prompts/a.md'\n"), 0o644)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Bots["a"].Prompt != filepath.Join(dir, "prompts/a.md") {
		t.Fatal(c.Bots["a"].Prompt)
	}
	if _, err := Load(filepath.Join(dir, "none.toml")); err == nil {
		t.Fatal("missing file")
	}
}

func TestWebSection(t *testing.T) {
	const bots = "[bots.a]\nmodel='m'\n"
	c, err := Parse([]byte(bots))
	if err != nil || c.Web.Enabled {
		t.Fatalf("web should default to disabled: %v %+v", err, c.Web)
	}
	c, err = Parse([]byte("[web]\nenabled=true\nlisten_interface='wg0'\nport=9000\n" + bots))
	if err != nil || c.Web.ListenInterface != "wg0" || c.Web.Port != 9000 {
		t.Fatalf("%v %+v", err, c.Web)
	}
	if _, err := Parse([]byte("[web]\nenabled=true\nlisten_addr='127.0.0.1:8787'\n" + bots)); err != nil {
		t.Fatal(err)
	}
	c, err = Parse([]byte("[web]\ncommand=['misa','-x']\n" + bots))
	if err != nil || len(c.Web.Command) != 2 || c.Web.Command[0] != "misa" {
		t.Fatalf("web command: %v %+v", err, c.Web)
	}
	for name, src := range map[string]string{
		"empty command": "[web]\ncommand=[]\n",
		"blank command": "[web]\ncommand=[' ']\n",
		"bad port":      "[web]\nlisten_addr='x:1'\nport=70000\n",
		"unknown key":   "[web]\nenable=true\n",
	} {
		if _, err := Parse([]byte(src + bots)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}
