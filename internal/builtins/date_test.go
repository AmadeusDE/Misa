package builtins

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDateDefault(t *testing.T) {
	r := runBuiltin(t, "date")
	assertExitCode(t, r, 0)
	if len(r.stdout) < 10 {
		t.Errorf("date output too short: %q", r.stdout)
	}
}

func TestDateFormatYear(t *testing.T) {
	r := runBuiltin(t, "date", "+%Y")
	assertExitCode(t, r, 0)
	year := strings.TrimSpace(r.stdout)
	if len(year) != 4 {
		t.Errorf("expected 4-digit year, got %q", year)
	}
	n, err := strconv.Atoi(year)
	if err != nil || n < 2020 || n > 2100 {
		t.Errorf("unexpected year %q", year)
	}
}

func TestDateUTC(t *testing.T) {
	r := runBuiltin(t, "date", "-u", "+%Z")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "UTC\n")
}

func TestDateEpoch(t *testing.T) {
	r := runBuiltin(t, "date", "+%s")
	assertExitCode(t, r, 0)
	epoch, err := strconv.ParseInt(strings.TrimSpace(r.stdout), 10, 64)
	if err != nil {
		t.Fatalf("expected epoch number, got %q", r.stdout)
	}
	now := time.Now().Unix()
	if epoch < now-5 || epoch > now+5 {
		t.Errorf("epoch %d too far from current time %d", epoch, now)
	}
}

func TestDateRFC(t *testing.T) {
	r := runBuiltin(t, "date", "-R")
	assertExitCode(t, r, 0)
	if !regexp.MustCompile(`\w{3}, \d{2} \w{3} \d{4}`).MatchString(r.stdout) {
		t.Errorf("RFC output unexpected: %q", r.stdout)
	}
}

func TestDateISO(t *testing.T) {
	r := runBuiltin(t, "date", "-I")
	assertExitCode(t, r, 0)
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(strings.TrimSpace(r.stdout)) {
		t.Errorf("ISO output unexpected: %q", r.stdout)
	}
}

func TestDateISOSeconds(t *testing.T) {
	r := runBuiltin(t, "date", "-Iseconds")
	assertExitCode(t, r, 0)
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`).MatchString(strings.TrimSpace(r.stdout)) {
		t.Errorf("ISO seconds output unexpected: %q", r.stdout)
	}
}

func TestDateFormatFull(t *testing.T) {
	r := runBuiltin(t, "date", "+%F %T")
	assertExitCode(t, r, 0)
	if !regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`).MatchString(r.stdout) {
		t.Errorf("full format unexpected: %q", r.stdout)
	}
}

func TestDatePercent(t *testing.T) {
	r := runBuiltin(t, "date", "+%%")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "%\n")
}

// TestDateTimeOnlyWithOffsetDefaultsSecondsToZero verifies a bare "HH:MM
// +ZZZZ" date string parses (using today's date) with seconds defaulting to
// 0, matching GNU date's documented behavior for time-only --date inputs.
func TestDateTimeOnlyWithOffsetDefaultsSecondsToZero(t *testing.T) {
	r := runBuiltin(t, "date", "--date=21:04 +0100", "+%S")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "00\n")
}

func TestDateEmptyReferenceIsError(t *testing.T) {
	r := runBuiltin(t, "date", "--reference=")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "No such file or directory")
}

func TestDateReferenceAndDateAreMutuallyExclusive(t *testing.T) {
	r := runBuiltin(t, "date", "-d", "2025-10-23 03:00", "-r/")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "mutually exclusive")
}
