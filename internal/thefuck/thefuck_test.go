package thefuck

import (
	"path/filepath"
	"testing"
)

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "b", 1},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"abc", "abcd", 1},
		{"kitten", "sitting", 3},
		{"ls", "sl", 2},
		{"gti", "git", 2},
	}
	for _, tt := range tests {
		got := levenshtein(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestRulePermissionDenied(t *testing.T) {
	fix := Match("pacman -S vim", "error: you cannot perform this operation unless you are root.", 1)
	if fix != "sudo pacman -S vim" {
		t.Errorf("got %q, want %q", fix, "sudo pacman -S vim")
	}
}

func TestRulePermissionDeniedAlreadySudo(t *testing.T) {
	fix := Match("sudo pacman -S vim", "Permission denied", 1)
	if fix != "" {
		t.Errorf("got %q, want empty", fix)
	}
}

func TestRuleMkdirP(t *testing.T) {
	fix := Match("mkdir a/b/c", "mkdir: cannot create directory 'a/b/c': No such file or directory", 1)
	if fix != "mkdir -p a/b/c" {
		t.Errorf("got %q, want %q", fix, "mkdir -p a/b/c")
	}
}

func TestRuleCdFile(t *testing.T) {
	fix := Match("cd /etc/hosts", "cd: Not a directory: /etc/hosts", 1)
	want := "cd " + filepath.Dir("/etc/hosts")
	if fix != want {
		t.Errorf("got %q, want %q", fix, want)
	}
}

func TestRuleGitPushUpstream(t *testing.T) {
	output := `fatal: The current branch feature has no upstream branch.
To push the current branch and set the remote as upstream, use

    git push --set-upstream origin feature`
	fix := Match("git push", output, 1)
	if fix != "git push --set-upstream origin feature" {
		t.Errorf("got %q, want %q", fix, "git push --set-upstream origin feature")
	}
}

func TestRuleGitPushRejected(t *testing.T) {
	fix := Match("git push", "! [rejected] main -> main (fetch first)", 1)
	if fix != "git pull && git push" {
		t.Errorf("got %q, want %q", fix, "git pull && git push")
	}
}

func TestRuleGitAdd(t *testing.T) {
	fix := Match("git commit -m 'test'", "Changes not staged for commit", 1)
	if fix != "git add -A && git commit -m 'test'" {
		t.Errorf("got %q, want %q", fix, "git add -A && git commit -m 'test'")
	}
}

func TestRuleGitStash(t *testing.T) {
	fix := Match("git checkout main", "error: Your local changes to the following files would be overwritten by checkout", 1)
	if fix != "git stash && git checkout main && git stash pop" {
		t.Errorf("got %q, want %q", fix, "git stash && git checkout main && git stash pop")
	}
}

func TestRuleRmDir(t *testing.T) {
	fix := Match("rm mydir", "rm: cannot remove 'mydir': Is a directory", 1)
	if fix != "rm -r mydir" {
		t.Errorf("got %q, want %q", fix, "rm -r mydir")
	}
}

func TestRuleCpDir(t *testing.T) {
	fix := Match("cp mydir dest", "cp: -r not specified; omitting directory 'mydir'", 1)
	if fix != "cp -r mydir dest" {
		t.Errorf("got %q, want %q", fix, "cp -r mydir dest")
	}
}

func TestRuleCdMkdir(t *testing.T) {
	fix := Match("cd newdir", "cd: no such file or directory: newdir", 1)
	if fix != "mkdir -p newdir && cd newdir" {
		t.Errorf("got %q, want %q", fix, "mkdir -p newdir && cd newdir")
	}
}

func TestRuleAptSudo(t *testing.T) {
	fix := Match("apt install vim", "E: Could not open lock file - are you root?", 1)
	if fix != "sudo apt install vim" {
		t.Errorf("got %q, want %q", fix, "sudo apt install vim")
	}
}

func TestExitCodeZero(t *testing.T) {
	fix := Match("ls", "file.txt", 0)
	if fix != "" {
		t.Errorf("got %q, want empty for exit code 0", fix)
	}
}

func TestReplaceCommand(t *testing.T) {
	got := replaceCommand("gti push", "gti", "git")
	if got != "git push" {
		t.Errorf("got %q, want %q", got, "git push")
	}
}
