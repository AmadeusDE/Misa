package justfile

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var candidateNames = []string{"justfile", "Justfile", "JUSTFILE", ".justfile"}

func Present(dir string) bool {
	if _, err := exec.LookPath("just"); err != nil {
		return false
	}
	for _, name := range candidateNames {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func Recipes(dir string) ([]string, error) {
	cmd := exec.Command("just", "--summary")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}
