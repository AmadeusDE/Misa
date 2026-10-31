package sshclient

import (
	"os"
	"testing"
)

func TestInvocationEnvironment(t *testing.T) {
	t.Setenv("HOME", "/host-home")
	t.Setenv("SSH_AUTH_SOCK", "/host-agent")
	t.Setenv("USER", "host-user")
	for _, tt := range []struct {
		name        string
		values      []string
		home, agent string
	}{
		{"inherit", nil, "/host-home", "/host-agent"},
		{"empty", []string{}, "", ""},
		{"explicit", []string{"HOME=/local-home", "SSH_AUTH_SOCK=/local-agent", "USER=local-user"}, "/local-home", "/local-agent"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := invocationEnv(Options{Env: tt.values}, "", nil, nil, nil)
			if e.Home != tt.home || e.AgentSocket != tt.agent {
				t.Fatalf("home %q agent %q", e.Home, e.AgentSocket)
			}
		})
	}
	if got := lookupOptionEnv([]string{}, "USER"); got != "" {
		t.Errorf("empty USER = %q", got)
	}
	if got := lookupOptionEnv(nil, "USER"); got != os.Getenv("USER") {
		t.Errorf("inherited USER = %q", got)
	}
	if got := lookupOptionEnv([]string{"USER=local-user"}, "USER"); got != "local-user" {
		t.Errorf("explicit USER = %q", got)
	}
}
