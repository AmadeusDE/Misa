package main

import (
	"os"
	"runtime"
	"testing"
)

func TestAgentFilesAreLinksToDocsAgents(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("git checks out symlinks as plain files on Windows by default")
	}
	links := map[string]string{
		"AGENTS.md":      "docs/AGENTS.md",
		"CLAUDE.md":      "docs/CLAUDE.md",
		"GEMINI.md":      "docs/GEMINI.md",
		"docs/CLAUDE.md": "AGENTS.md",
		"docs/GEMINI.md": "AGENTS.md",
	}
	for link, want := range links {
		got, err := os.Readlink(link)
		if err != nil {
			t.Errorf("%s is not a symlink (edit docs/AGENTS.md, never a link): %v", link, err)
			continue
		}
		if got != want {
			t.Errorf("%s -> %s, want -> %s", link, got, want)
		}
	}
	if info, err := os.Lstat("docs/AGENTS.md"); err != nil || !info.Mode().IsRegular() {
		t.Errorf("docs/AGENTS.md must be the one regular file: %v", err)
	}
}
