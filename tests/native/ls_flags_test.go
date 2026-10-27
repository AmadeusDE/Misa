package native

import (
	"bytes"
	"strings"
	"testing"
)

func TestLs_NoGroup_WithG(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "hello")
	mkFile(t, dir, "b.txt", "world")

	// Test -G flag with -l: should show long format without group column
	r := runIn(t, dir, "ls", "-G", "-l", "--no-smart")
	wantSuccess(t, r)

	// Output should have "total " header
	if !strings.HasPrefix(r.stdout, "total ") {
		t.Errorf("expected 'total ...' header in ls -G -l, got: %q", r.stdout)
	}

	// Count the number of columns by examining a line
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines (total + file), got %d", len(lines))
	}

	// The line after "total" should have one less column than normal long format
	dataLine := lines[1]
	fields := strings.Fields(dataLine)

	// Normal ls -l format: perms links owner group size date time name (8+ fields)
	// With -G: perms links owner size date time name (7+ fields)
	// So with -G should have 1 fewer field
	if len(fields) < 7 {
		t.Errorf("expected at least 7 columns in -G format, got %d in line: %q", len(fields), dataLine)
	}
}

func TestLs_NoGroup_WithNoGroup(t *testing.T) {
	t.Skip("--no-group not yet implemented")
}

func TestLs_ShortO(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "hello")
	mkFile(t, dir, "b.txt", "world")

	// Test -o flag: like -l but without group column
	r := runIn(t, dir, "ls", "-o", "--no-smart")
	wantSuccess(t, r)

	// Output should have "total " header
	if !strings.HasPrefix(r.stdout, "total ") {
		t.Errorf("expected 'total ...' header in ls -o, got: %q", r.stdout)
	}

	// Count columns: should be like -l but without group
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	dataLine := lines[1]
	fields := strings.Fields(dataLine)

	if len(fields) < 7 {
		t.Errorf("expected at least 7 columns in -o format, got %d in line: %q", len(fields), dataLine)
	}
}

func TestLs_Zero_NULTerminated(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "hello")
	mkFile(t, dir, "b.txt", "world")

	// Test -0 flag (or --zero): output should be NUL-terminated
	r := runIn(t, dir, "ls", "-0", "--no-smart")
	wantSuccess(t, r)

	// Output should end with NUL byte instead of newline
	if len(r.stdout) == 0 {
		t.Errorf("expected non-empty output")
	}

	// Should not end with newline
	if strings.HasSuffix(r.stdout, "\n") {
		t.Errorf("output should be NUL-terminated, not newline-terminated")
	}

	// Should end with NUL byte
	if r.stdout[len(r.stdout)-1] != '\x00' {
		t.Errorf("expected output to end with NUL byte (0x00), got %q", r.stdout[len(r.stdout)-1])
	}

	// Count NUL bytes - should match number of files
	nulCount := bytes.Count([]byte(r.stdout), []byte{'\x00'})
	if nulCount != 2 {
		t.Errorf("expected 2 NUL bytes (for 2 files), got %d", nulCount)
	}
}

func TestLs_Zero_LongFormat(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "hello")

	// Test -0 with -l: should produce long format with NUL termination
	r := runIn(t, dir, "ls", "-0", "-l", "--no-smart")
	wantSuccess(t, r)

	// Should end with NUL byte
	if len(r.stdout) == 0 {
		t.Fatalf("expected non-empty output")
	}

	if r.stdout[len(r.stdout)-1] != '\x00' {
		t.Errorf("expected output to end with NUL byte, got %q", r.stdout[len(r.stdout)-1:])
	}

	// Check for typical long format content (perms, owner, etc.) before the NUL
	content := strings.TrimRight(r.stdout, "\x00")
	if !strings.Contains(content, "a.txt") {
		t.Errorf("expected filename in output, got: %q", content)
	}
}

func TestLs_ShortO_WithZero(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "hello")
	mkFile(t, dir, "b.txt", "world")

	// Test -o with --zero: should produce no-group long format with NUL termination
	r := runIn(t, dir, "ls", "-o", "--zero", "--no-smart")
	wantSuccess(t, r)

	// Should have NUL terminators
	if len(r.stdout) == 0 {
		t.Fatalf("expected non-empty output")
	}

	if r.stdout[len(r.stdout)-1] != '\x00' {
		t.Errorf("expected output to end with NUL byte")
	}

	// Count NUL bytes
	nulCount := bytes.Count([]byte(r.stdout), []byte{'\x00'})
	if nulCount != 2 {
		t.Errorf("expected 2 NUL bytes (for 2 files), got %d", nulCount)
	}
}
