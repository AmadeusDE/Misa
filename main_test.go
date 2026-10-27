package main

import (
	"errors"
	"slices"
	"testing"
)

func lookPathIn(installed ...string) func(string) (string, error) {
	return func(name string) (string, error) {
		if slices.Contains(installed, name) {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
}

func TestGUICommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		installed []string
		argv      []string
		title     string
	}{
		{"misa itself", nil, nil, []string{"/bin/misa"}, "Misa"},
		{"sshc builtin", []string{"sshc", "-p", "2222", "me@host"}, nil, []string{"/bin/misa", "sshc", "-p", "2222", "me@host"}, "sshc -p 2222 me@host"},
		{"installed program shadows builtin", []string{"ls", "-l"}, []string{"ls"}, []string{"ls", "-l"}, "ls -l"},
		{"builtin when not installed", []string{"ls"}, nil, []string{"/bin/misa", "ls"}, "ls"},
		{"other command", []string{"htop-nonexistent-xyz"}, nil, []string{"htop-nonexistent-xyz"}, "htop-nonexistent-xyz"},
	}
	for _, tt := range tests {
		argv, title := guiCommand("/bin/misa", tt.args, lookPathIn(tt.installed...))
		if !slices.Equal(argv, tt.argv) || title != tt.title {
			t.Errorf("%s: got %q %q, want %q %q", tt.name, argv, title, tt.argv, tt.title)
		}
	}
}

func TestStartSSHNeedsDestination(t *testing.T) {
	if code := startSSH(nil); code != 2 {
		t.Fatalf("startSSH() = %d, want 2", code)
	}
}
