package vm

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/confine"
	"github.com/AmadeusDE/Misa/internal/ugh/worktree"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
)

func testVM(t *testing.T, extra string) config.VM {
	t.Helper()
	c, err := config.Parse([]byte("[vm]\n" + extra))
	if err != nil {
		t.Fatal(err)
	}
	return c.VM
}

func TestUserDataPinsKeysAndUID(t *testing.T) {
	k, err := loadKeys(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := userData(testVM(t, `setup = ["echo extra"]`), k, 1234, "http://10.0.2.2:1/tok")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), "#cloud-config\n") {
		t.Fatal("missing #cloud-config header")
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw[len("#cloud-config\n"):], &cfg); err != nil {
		t.Fatal(err)
	}
	user := cfg["users"].([]any)[0].(map[string]any)
	if user["name"] != guestUser || user["uid"].(float64) != 1234 || user["ssh_authorized_keys"].([]any)[0] != authorizedLine(k.login) {
		t.Fatalf("user = %v", user)
	}
	if keys := cfg["ssh_keys"].(map[string]any); keys["ed25519_public"] != authorizedLine(k.host) || !strings.Contains(keys["ed25519_private"].(string), "OPENSSH PRIVATE KEY") {
		t.Fatalf("ssh_keys = %v", keys)
	}
	script := cfg["write_files"].([]any)[0].(map[string]any)["content"].(string)
	for _, want := range []string{"pacman -Syu --noconfirm --needed git base-devel go ripgrep", "echo extra", "cloud-init.disabled", "http://10.0.2.2:1/tok/\"$1\"", "report done"} {
		if !strings.Contains(script, want) {
			t.Errorf("prepare script lacks %q:\n%s", want, script)
		}
	}
	k2, _ := loadKeys(filepath.Dir(filepath.Join(t.TempDir(), "x")))
	if authorizedLine(k2.login) == authorizedLine(k.login) {
		t.Fatal("keys must be per state dir")
	}
}

func TestQEMUArgs(t *testing.T) {
	args := strings.Join(qemuArgs(testVM(t, "network = false\nmemory_mb = 2048"), boot{disk: "/d.qcow2", serial: "/s.log", sshPort: 2222, shares: []string{"/src/a", "/w"}}), " ")
	for _, want := range []string{"-m 2048", "hostfwd=tcp:127.0.0.1:2222-:22,restrict=on", "local,path=/src/a,mount_tag=share0,security_model=none", "path=/w,mount_tag=share1", "file=/d.qcow2,if=virtio"} {
		if !strings.Contains(args, want) {
			t.Errorf("args lack %q: %s", want, args)
		}
	}
	if strings.Contains(strings.Join(qemuArgs(testVM(t, ""), boot{disk: "/d", serial: "/s"}), " "), "restrict") {
		t.Error("network is on by default")
	}
	if q := quote("it's"); q != `'it'\''s'` {
		t.Errorf("quote = %s", q)
	}
}

// TestRealVM boots the prepared base under qemu (UGHBOT_VM_TEST=1, after `ughbot vm prepare`;
// UGHBOT_VM_HELPER names a built ughbot binary to copy in).
func TestRealVM(t *testing.T) {
	if os.Getenv("UGHBOT_VM_TEST") == "" {
		t.Skip("set UGHBOT_VM_TEST=1 to boot a real VM")
	}
	cfg, err := config.Parse([]byte("[vm]\nhelper = '" + os.Getenv("UGHBOT_VM_HELPER") + "'\n"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.StateDir = config.DefaultStateDir()
	repo, wts := t.TempDir(), t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v %s", err, out)
		}
	}
	mc, err := New(cfg, os.Stderr).Start(context.Background(), []string{repo, wts})
	if err != nil {
		t.Fatal(err)
	}
	defer mc.Close()
	ctx := worktree.WithGit(context.Background(), mc.Git)
	out, err := mc.RunTool(ctx, "bash", repo, json.RawMessage(`{"command":"echo from-vm > made.txt && uname -n"}`))
	if err != nil || !strings.Contains(out, "ughbot-vm") {
		t.Fatalf("bash = %q, %v", out, err)
	}
	fi, err := os.Stat(filepath.Join(repo, "made.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if uid := fi.Sys().(*syscall.Stat_t).Uid; int(uid) != os.Getuid() {
		t.Fatalf("a file the VM wrote belongs to uid %d", uid)
	}
	wt := filepath.Join(wts, "feature")
	if _, stderr, err := worktree.Git(ctx, repo, nil, "worktree", "add", "-q", "-b", "feature", wt); err != nil {
		t.Fatalf("worktree add in the VM: %v %s", err, stderr)
	}
	if _, err := os.Stat(filepath.Join(wt, ".git")); err != nil {
		t.Fatalf("the host does not see the worktree the VM made: %v", err)
	}
	if out, err := exec.Command("git", "-C", repo, "branch", "--list", "feature").Output(); err != nil || !strings.Contains(string(out), "feature") {
		t.Fatalf("host git does not see the branch: %q %v", out, err)
	}
	for _, d := range []string{repo, wts} {
		defer confine.Add(d)()
	}
	alloc, err := worktree.Allocate(ctx, repo, worktree.AllocateOptions{Run: "r1", Agent: "root/1", Task: "t",
		DiscloseDirtyAbsent: true, WorktreeDir: filepath.Join(wts, "managed")})
	if err != nil {
		t.Fatalf("ugh's worktree allocation through the VM: %v", err)
	}
	if _, err := mc.RunTool(ctx, "bash", alloc.Dir, json.RawMessage(`{"command":"echo w > w.txt && git add w.txt && git commit -qm w"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := mc.RunTool(ctx, "bash", repo, json.RawMessage(`{"command":"git add made.txt && git commit -qm made"}`)); err != nil {
		t.Fatal(err)
	}
	if err := alloc.MarkCompleted("done"); err != nil {
		t.Fatal(err)
	}
	branch, _, _ := worktree.Git(ctx, repo, nil, "rev-parse", "--abbrev-ref", "HEAD")
	res, err := worktree.Integrate(ctx, repo, worktree.IntegrateOptions{WorktreeDir: alloc.Dir, TargetBranch: strings.TrimSpace(string(branch)), Strategy: worktree.StrategyMerge})
	if err != nil {
		t.Fatalf("integrate through the VM: %v", err)
	}
	if out, err := exec.Command("git", "-C", repo, "log", "--oneline", "-1", res.NewCommit).Output(); err != nil || len(out) == 0 {
		t.Fatalf("host git lacks the integrated commit %s: %v", res.NewCommit, err)
	}
	if _, err := mc.RunTool(ctx, "read", repo, json.RawMessage(`{"path":"/home/`+guestUser+`/../../etc/hostname"}`)); err != nil {
		t.Fatalf("reads inside the VM work: %v", err)
	}
	if out, _ := mc.RunTool(ctx, "bash", repo, json.RawMessage(`{"command":"ls `+config.DefaultStateDir()+` 2>&1 || true"}`)); !strings.Contains(out, "No such file") {
		t.Fatalf("the VM can see ughbot's state dir: %s", out)
	}
}
