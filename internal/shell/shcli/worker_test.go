package shcli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestWorkerRejectsExtraArguments(t *testing.T) {
	var errout bytes.Buffer
	if got := Run(context.Background(), "sh", []string{"--worker", "extra"}, Options{Stderr: &errout}); got == 0 || !strings.Contains(errout.String(), "worker") {
		t.Fatalf("status=%d stderr=%q", got, errout.String())
	}
}
