package gitinfo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Info struct {
	Branch string
	Dirty  bool
	Ahead  int
	Behind int
	InRepo bool
}

func Fetch(dir string) Info {
	if !isRepo(dir) {
		return Info{}
	}
	info := Info{InRepo: true}
	info.Branch = getBranch(dir)
	info.Dirty = isDirty(dir)
	info.Ahead, info.Behind = getAheadBehind(dir)
	return info
}

func (i Info) String() string {
	if !i.InRepo {
		return ""
	}
	s := i.Branch
	if i.Dirty {
		s += "*"
	}
	if i.Ahead > 0 {
		s += fmt.Sprintf(" +%d", i.Ahead)
	}
	if i.Behind > 0 {
		s += fmt.Sprintf(" -%d", i.Behind)
	}
	return s
}

func isRepo(dir string) bool {
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(d)
		if parent == d {
			return false
		}
		d = parent
	}
}

func getBranch(dir string) string {
	out, err := gitCmd(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}
	return out
}

func isDirty(dir string) bool {
	out, err := gitCmd(dir, "status", "--porcelain", "-uno")
	if err != nil {
		return false
	}
	return out != ""
}

func getAheadBehind(dir string) (int, int) {
	out, err := gitCmd(dir, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		return 0, 0
	}
	fields := strings.Fields(out)
	if len(fields) < 2 {
		return 0, 0
	}
	ahead := atoi(fields[0])
	behind := atoi(fields[1])
	return ahead, behind
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

func gitCmd(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
