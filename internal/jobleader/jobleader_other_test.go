//go:build !unix

package jobleader

import "testing"

func TestRunWithoutCommandFails(t *testing.T) {
	if code := Run(nil); code != 2 {
		t.Errorf("exit = %d", code)
	}
}

func TestRunMissingCommandIs127(t *testing.T) {
	if code := Run([]string{"misa-no-such-command"}); code != 127 {
		t.Errorf("exit = %d", code)
	}
}
