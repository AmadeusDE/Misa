package native

import (
	"testing"
)

func TestFmt_Basic(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "input.txt", "one\ntwo\nthree\n")
	r := runIn(t, dir, "fmt", "-w", "20", "input.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "one two three")
}

func TestFmt_PrefersFullerLineOnEqualCost(t *testing.T) {
	r := runStdin(t, "fmt", "aa bb cc dd ee\n", "-w", "8")
	wantSuccess(t, r)
	wantOut(t, r, "aa bb cc\ndd ee\n")
}

func TestFmt_Goal(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "input.txt", "this is a file with one word per line\n")
	r := runIn(t, dir, "fmt", "-w", "75", "-g", "10", "input.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "this is a file with one word per line")
}

func TestFmt_InvalidWidth(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "input.txt", "text")
	r := runIn(t, dir, "fmt", "-w", "invalid", "input.txt")
	wantCode(t, r, 1)
	wantErrContains(t, r, "invalid width")
}

func TestFmt_GoalBiggerThanWidth(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "input.txt", "text")
	r := runIn(t, dir, "fmt", "-w", "10", "-g", "20", "input.txt")
	wantCode(t, r, 1)
	wantErrContains(t, r, "GOAL cannot be greater than WIDTH")
}

func TestFmt_PositionalWidthFirst(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "input.txt", "one two three\n")
	r := runIn(t, dir, "fmt", "-10", "input.txt")
	wantSuccess(t, r)
}
