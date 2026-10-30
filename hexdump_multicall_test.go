package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHexdumpMulticallAlias(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "misa")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	cmd := exec.Command(bin, "hd")
	cmd.Stdin = strings.NewReader("ABC")
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "00000000  41 42 43                                          |ABC|\n00000003\n" {
		t.Fatalf("hd: %v: %q", err, out)
	}
}
