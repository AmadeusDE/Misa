//go:build !unix || android

package gui

import "testing"

func TestRunErrorsWithoutFrontend(t *testing.T) {
	if code := Run(t.TempDir(), []string{"true"}, "misa"); code != 1 {
		t.Fatalf("Run = %d, want 1", code)
	}
}
