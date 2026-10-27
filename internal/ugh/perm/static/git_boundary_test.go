package static

import "testing"

func TestGitBoundarySeesThroughWrappersAndShells(t *testing.T) {
	owned := "/work/lane"
	for _, cmd := range []string{
		"git push origin main",
		"timeout -s KILL 5 git push",
		"env -S 'git push'",
		"echo x | xargs -n1 git push",
		`sh -c "git push origin main"`,
		"bash -lc 'git -C /work/other commit -m x'",
		`eval "git push"`,
		"cd /work/other && git status",
	} {
		if v := GitOutsideOwnerViolations(cmd, owned); len(v) == 0 {
			t.Errorf("%s: no violation", cmd)
		}
	}
	for _, cmd := range []string{"git status", "git add -A && git commit -m x", `sh -c "git diff"`, "timeout 5 git log"} {
		if v := GitOutsideOwnerViolations(cmd, owned); len(v) != 0 {
			t.Errorf("%s: %v", cmd, v)
		}
	}
}
