package native

import (
	"strings"
	"testing"
)

func TestFold_Default80(t *testing.T) {
	long := strings.Repeat("a", 100) + "\n"
	r := runStdin(t, "fold", long)
	wantSuccess(t, r)
	lines := strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
	if len(lines[0]) != 80 {
		t.Errorf("first line length = %d, want 80", len(lines[0]))
	}
}

func TestFold_Width20(t *testing.T) {
	long := strings.Repeat("x", 50) + "\n"
	r := runStdin(t, "fold", long, "-w", "20")
	wantSuccess(t, r)
	lines := strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestFold_BreakAtSpaces(t *testing.T) {
	r := runStdin(t, "fold", "hello world foo bar baz\n", "-s", "-w", "12")
	wantSuccess(t, r)
	lines := strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")
	for _, line := range lines {
		if len(line) > 12 {
			t.Errorf("line too long: %q (%d chars)", line, len(line))
		}
	}
}

func TestFold_ShortLine(t *testing.T) {
	r := runStdin(t, "fold", "hi\n")
	wantSuccess(t, r)
	wantOut(t, r, "hi\n")
}

func TestFold_Empty(t *testing.T) {
	r := runStdin(t, "fold", "")
	wantSuccess(t, r)
	wantOut(t, r, "")
}

func TestFold_File(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", strings.Repeat("z", 100)+"\n")
	r := runIn(t, dir, "fold", "-w", "50", "f.txt")
	wantSuccess(t, r)
	lines := strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

func TestFold_Tabs(t *testing.T) {
	r := runStdin(t, "fold", "\thello\n", "-w", "20")
	wantSuccess(t, r)
	wantOutContains(t, r, "hello")
}

func TestFold_WideCharactersColumnMode(t *testing.T) {
	// U+B250 is width 2; three of them at width 5 wrap after two.
	input := "\uB250\uB250\uB250\n"
	r := runStdin(t, "fold", input, "-w", "5")
	wantSuccess(t, r)
	wantOut(t, r, "\uB250\uB250\n\uB250\n")
}

func TestFold_WideCharactersCharacterMode(t *testing.T) {
	input := "\uB250\uB250\uB250\n"
	r := runStdin(t, "fold", input, "--characters", "-w", "5")
	wantSuccess(t, r)
	wantOut(t, r, input)
}

func TestFold_WideCharactersCharacterShortOption(t *testing.T) {
	input := "\uB250\uB250\uB250\n"
	r := runStdin(t, "fold", input, "-c", "-w", "5")
	wantSuccess(t, r)
	wantOut(t, r, input)
}

func TestFold_CombiningCharacterHasZeroWidth(t *testing.T) {
	r := runStdin(t, "fold", "ééé", "-w", "2")
	wantSuccess(t, r)
	wantOut(t, r, "éé\né")
}

func TestFold_CharactersTabStop(t *testing.T) {
	r := runStdin(t, "fold", "ab\tcd\n", "-c", "-w", "4")
	wantSuccess(t, r)
	wantOut(t, r, "ab\n\t\ncd\n")
}
