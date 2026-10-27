package editor

import (
	"os"
	"os/exec"
	"strings"
)

func Resolve() []string {
	if e := os.Getenv("EDITOR"); e != "" {
		fields := strings.Fields(e)
		if len(fields) > 0 {
			if path, err := exec.LookPath(fields[0]); err == nil {
				fields[0] = path
				return fields
			}
		}
	}

	for _, c := range []string{"micro", "nano"} {
		if path, err := exec.LookPath(c); err == nil {
			return []string{path}
		}
	}
	return []string{"nano"}
}
