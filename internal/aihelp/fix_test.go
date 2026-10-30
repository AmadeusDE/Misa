package aihelp

import (
	"strings"
	"testing"
)

func TestParseFix(t *testing.T) {
	for _, c := range []struct{ name, in, cmd, why string }{
		{"plain", "pacman -S firefox\n\n-s is not an operation.", "pacman -S firefox", "-s is not an operation."},
		{"backticks", "`ls -la`\n\nTypo.", "ls -la", "Typo."},
		{"fence", "```sh\ngit status\n```\n\nIt is status.", "git status", "It is status."},
		{"no explanation", "ls", "ls", ""},
		{"no blank line", "ls -l\nlong format", "ls -l", "long format"},
		{"multi paragraph", "x\n\nOne.\n\nTwo.", "x", "One.\n\nTwo."},
		{"leading blank", "\n\n  cd /tmp  \n\nok", "cd /tmp", "ok"},
	} {
		cmd, why, err := ParseFix(c.in)
		if err != nil || cmd != c.cmd || why != c.why {
			t.Errorf("%s: ParseFix = %q, %q, %v; want %q, %q", c.name, cmd, why, err, c.cmd, c.why)
		}
	}
	for _, in := range []string{"", "   \n ", "``\n\nx", "```\n```"} {
		if cmd, _, err := ParseFix(in); err == nil {
			t.Errorf("ParseFix(%q) = %q, want an error", in, cmd)
		}
	}
}

func TestFixPromptIncludesStatusAndTheEndOfTheOutput(t *testing.T) {
	long := strings.Repeat("x", maxErrorOutput) + "THE END"
	p := fixPrompt("pacman -s foo", long, 1)
	if !strings.HasPrefix(p, "Failed command: pacman -s foo\nExit status: 1\n\nError output:\n...") || !strings.HasSuffix(p, "THE END") {
		t.Errorf("prompt = %q", p)
	}
	if len(p) > maxErrorOutput+120 {
		t.Errorf("prompt is %d bytes", len(p))
	}
	if got := fixPrompt("ls", "", 0); got != "Failed command: ls" {
		t.Errorf("prompt = %q", got)
	}
}
