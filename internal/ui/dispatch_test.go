package ui

import (
	"slices"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestPlanCommand(t *testing.T) {
	m := New(t.TempDir())
	defer func(old string) { BuiltinExecutable = old }(BuiltinExecutable)

	BuiltinExecutable = "/opt/misa"
	if p := m.planCommand("ls -la"); !slices.Equal(p.argv, []string{"/opt/misa", "ls", "-la"}) {
		t.Errorf("multicall builtin argv = %q", p.argv)
	}
	m.builtinMode = builtinModeInProcess
	if p := m.planCommand("ls -la"); !p.inProcess() || p.builtin.Name != "ls" {
		t.Errorf("inprocess builtin plan = %+v", p)
	}
	if p := m.planCommand("go version"); len(p.argv) == 0 || p.argv[0] != "go" {
		t.Errorf("external command should exec directly, got %q", p.argv)
	}
	for _, in := range []string{"ls | wc -l", "FOO=1 env", "./script.sh", "no-such-cmd-xyz"} {
		if p := m.planCommand(in); !slices.Equal(p.argv, []string{"sh", "-c", in}) {
			t.Errorf("%q should go through sh, got %q", in, p.argv)
		}
	}
}

func TestBuiltinDoesNotBlockUpdate(t *testing.T) {
	m := New(t.TempDir())
	m.nestedMode = true
	m.width, m.height = 80, 24
	m.enterFullShell()
	m.cmdInput.SetValue("sleep 2")
	start := time.Now()
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Fatalf("Update blocked for %v running a builtin", d)
	}
	if m.terminal == nil || cmd == nil {
		t.Fatal("builtin did not start in the pane's terminal")
	}
	m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	pumpUntilExit(t, m, cmd)
	if m.lastExitCode != 130 {
		t.Errorf("exit = %d, want 130 after ctrl+c", m.lastExitCode)
	}
}

func TestCompoundCommandsReachShell(t *testing.T) {
	m := New(t.TempDir())
	m.nestedMode = true
	m.width, m.height = 80, 24
	m.enterFullShell()
	m.shellBuffer = []string{"old output"}
	m.cmdInput.SetValue("clear && printf audit-after-clear")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.terminal == nil {
		t.Fatal("compound clear did not run in sh")
	}
	pumpUntilExit(t, m, cmd)
	found := false
	for _, l := range m.shellBuffer {
		if l == "audit-after-clear" {
			found = true
		}
	}
	if !found {
		t.Errorf("right-hand command output missing: %q", m.shellBuffer)
	}
}

func TestAliasKeepsArgumentsVerbatim(t *testing.T) {
	m := New(t.TempDir())
	m.aliases = map[string]string{"e": "echo"}
	if got := m.expandAlias(`e "a  b"`); got != `echo "a  b"` {
		t.Errorf("got %q", got)
	}
	if got := m.expandAlias("e"); got != "echo" {
		t.Errorf("bare alias got %q", got)
	}
}

func TestProgramsRunUnderJobLeader(t *testing.T) {
	defer func(old string) { BuiltinExecutable = old }(BuiltinExecutable)
	BuiltinExecutable = "/bin/misa"
	if got := underJobLeader([]string{"vim", "f"}); !slices.Equal(got, []string{"/bin/misa", "--job-leader", "vim", "f"}) {
		t.Errorf("program argv = %q", got)
	}
	if got := underJobLeader([]string{"/bin/misa", "ls"}); !slices.Equal(got, []string{"/bin/misa", "ls"}) {
		t.Errorf("builtin argv = %q", got)
	}
	BuiltinExecutable = ""
	if got := underJobLeader([]string{"vim"}); !slices.Equal(got, []string{"vim"}) {
		t.Errorf("without a binary argv = %q", got)
	}
}
