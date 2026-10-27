package ui

import (
	"testing"
)

func TestSelBeginExtend(t *testing.T) {
	var s textSel
	s.begin(2, 5)
	if !s.active {
		t.Error("expected active after begin")
	}
	if s.hasRange {
		t.Error("should not have range before extend")
	}

	s.extend(4, 10)
	if !s.hasRange {
		t.Error("expected hasRange after extend to different position")
	}

	r1, c1, r2, c2 := s.ordered()
	if r1 != 2 || c1 != 5 || r2 != 4 || c2 != 10 {
		t.Errorf("ordered = (%d,%d)-(%d,%d), want (2,5)-(4,10)", r1, c1, r2, c2)
	}
}

func TestSelOrderedReverse(t *testing.T) {
	var s textSel
	s.begin(5, 10)
	s.extend(2, 3)
	r1, c1, r2, c2 := s.ordered()
	if r1 != 2 || c1 != 3 || r2 != 5 || c2 != 10 {
		t.Errorf("ordered = (%d,%d)-(%d,%d), want (2,3)-(5,10)", r1, c1, r2, c2)
	}
}

func TestSelExtractFromLines(t *testing.T) {
	lines := []string{"hello world", "second line", "third line"}

	var s textSel
	s.begin(0, 6)
	s.extend(0, 11)
	text := s.extractFromLines(lines)
	if text != "world" {
		t.Errorf("single line extract = %q, want %q", text, "world")
	}

	s.begin(0, 6)
	s.extend(1, 6)
	text = s.extractFromLines(lines)
	if text != "world\nsecond" {
		t.Errorf("multiline extract = %q, want %q", text, "world\nsecond")
	}
}

func TestSelClear(t *testing.T) {
	var s textSel
	s.begin(1, 2)
	s.extend(3, 4)
	s.clear()
	if s.active || s.hasRange {
		t.Error("expected cleared state")
	}
}

func TestExtractRange(t *testing.T) {
	lines := []string{"aaaa", "bbbb", "cccc"}
	text := extractRange(lines, 0, 2, 2, 2)
	if text != "aa\nbbbb\ncc" {
		t.Errorf("extractRange = %q, want %q", text, "aa\nbbbb\ncc")
	}
}
