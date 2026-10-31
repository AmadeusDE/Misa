//go:build unix

package proceffect

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func execTarget(argv []string) int {
	if len(argv) == 0 || argv[0] == "" {
		return 127
	}
	name := argv[0]
	var candidates []string
	if strings.ContainsRune(name, '/') {
		candidates = []string{name}
	} else {
		path, ok := os.LookupEnv("PATH")
		if !ok {
			fmt.Fprintf(os.Stderr, "%s: not found\n", name)
			return 127
		}
		for _, p := range strings.Split(path, ":") {
			if p == "" {
				p = "."
			}
			candidates = append(candidates, filepath.Join(p, name))
		}
	}
	denied := false
	for _, p := range candidates {
		err := syscall.Exec(p, argv, os.Environ())
		if errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ENOTDIR) {
			continue
		}
		if errors.Is(err, syscall.EACCES) {
			denied = true
			continue
		}
		fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
		return 126
	}
	if denied {
		fmt.Fprintf(os.Stderr, "%s: permission denied\n", name)
		return 126
	}
	fmt.Fprintf(os.Stderr, "%s: not found\n", name)
	return 127
}
func umaskChild() (int, error) {
	exe, e := registeredHelper()
	if e != nil {
		return 0, e
	}
	c := exec.Command(exe, Flag, "umask")
	b, e := c.Output()
	if e != nil {
		return 0, e
	}
	n, e := strconv.Atoi(strings.TrimSpace(string(b)))
	return n, e
}
