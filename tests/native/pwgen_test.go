package native

import (
	"strings"
	"testing"
)

func isAlnumString(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			continue
		}
		return false
	}
	return true
}

func TestPwgen_Default(t *testing.T) {
	r := run(t, "pwgen")
	wantSuccess(t, r)
	if got := strings.Count(r.stdout, "\n"); got != 1 {
		t.Fatalf("stdout = %q, want a single line", r.stdout)
	}
	line := strings.TrimSuffix(r.stdout, "\n")
	if len(line) != 8 {
		t.Fatalf("password %q has length %d, want 8", line, len(line))
	}
	if !isAlnumString(line) {
		t.Fatalf("password %q is not [0-9A-Za-z]", line)
	}
}

func TestPwgen_LengthCount(t *testing.T) {
	r := run(t, "pwgen", "6", "2")
	wantSuccess(t, r)
	lines := strings.Split(strings.TrimSuffix(r.stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("stdout = %q, want two lines", r.stdout)
	}
	for _, line := range lines {
		if len(line) != 6 {
			t.Fatalf("line %q has length %d, want 6", line, len(line))
		}
	}
}

func TestPwgen_NoNumerals(t *testing.T) {
	r := run(t, "pwgen", "-0", "200")
	wantSuccess(t, r)
	body := strings.TrimSuffix(r.stdout, "\n")
	if len(body) != 200 {
		t.Fatalf("got %d characters in %q, want 200", len(body), body)
	}
	if strings.ContainsAny(body, "0123456789") {
		t.Fatalf("output %q contains a digit despite -0", body)
	}
	if !isAlnumString(body) {
		t.Fatalf("output %q is not [0-9A-Za-z]", body)
	}
}

func TestPwgen_Symbols(t *testing.T) {
	r := run(t, "pwgen", "-y", "200")
	wantSuccess(t, r)
	body := strings.TrimSuffix(r.stdout, "\n")
	if len(body) != 200 {
		t.Fatalf("got %d characters in %q, want 200", len(body), body)
	}
	for i := 0; i < len(body); i++ {
		if body[i] < 33 || body[i] > 126 {
			t.Fatalf("output %q contains byte %q outside the printable range", body, body[i])
		}
	}
	if isAlnumString(body) {
		t.Fatalf("output %q has no symbols despite -y", body)
	}
}

func TestPwgen_UnknownOption(t *testing.T) {
	r := run(t, "pwgen", "-z")
	wantFail(t, r)
	wantErrContains(t, r, "pwgen: unknown option -z")
}

func TestPwgen_BadLength(t *testing.T) {
	for _, arg := range []string{"0", "abc", "5000"} {
		r := run(t, "pwgen", arg)
		wantCode(t, r, 1)
		wantErr(t, r, "pwgen: bad length\n")
	}
}

func TestPwgen_ConflictingOptions(t *testing.T) {
	r := run(t, "pwgen", "-c", "-A")
	wantCode(t, r, 1)
	wantErr(t, r, "pwgen: -c and -A are mutually exclusive\n")
}

func TestPwgen_Help(t *testing.T) {
	r := run(t, "pwgen", "--help")
	wantSuccess(t, r)
	wantOutContains(t, r, "pwgen")
	wantOutContains(t, r, "usage: pwgen")
}
