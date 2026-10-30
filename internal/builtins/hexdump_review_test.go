package builtins

import (
	"strings"
	"testing"
)

func TestHexdumpStringNULAndPartialFields(t *testing.T) {
	out, err, code := hexRun("a\x00bc", "-e", `1/4 "%s"`)
	if code != 0 || out != "a" {
		t.Fatalf("string: %q %q %d", out, err, code)
	}
	out, err, code = hexRun("a", "-e", `2/1 "%3_u "`)
	if code != 0 || strings.Contains(out, "%!") {
		t.Fatalf("partial: %q %q %d", out, err, code)
	}
}
