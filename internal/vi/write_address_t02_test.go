package vi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestT02InvalidWriteAddresses(t *testing.T) {
	for _, command := range []string{
		"0w!", "3w!", "2,1w!", "1,3w!", "1,0w!", "$+1w!", "1-1w!",
		"999999999999999999999999999999999999999999999999w!",
		"$+999999999999999999999999999999999999999999999w!",
		"999999999999999999999-999999998w!", "$+999999999999999999999-999999999w!",
		"3wq!", "3x!", "3w! out", "3w! >> out", "3w !echo bad > out",
	} {
		t.Run(command, func(t *testing.T) {
			e, p := f07Editor(t, "one\ntwo\n")
			originalFile, originalRevision := e.file, e.savedRevision
			e.executeCommand(command)
			if !strings.Contains(e.statusMsg, "E16") || e.Quit || e.modified || e.file != originalFile || e.savedRevision != originalRevision {
				t.Errorf("status=%q quit=%v modified=%v file=%+v", e.statusMsg, e.Quit, e.modified, e.file)
			}
			if got := f07Read(t, p); got != "one\ntwo\n" {
				t.Errorf("current file = %q", got)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(p), "out")); !os.IsNotExist(err) {
				t.Errorf("output created: %v", err)
			}
		})
	}
}

func TestT02ValidWriteAddressesAndOtherRanges(t *testing.T) {
	e, p := f07Editor(t, "one\ntwo\n")
	e.executeCommand("1,2w out")
	if got := f07Read(t, filepath.Join(filepath.Dir(p), "out")); got != "one\ntwo\n" {
		t.Errorf("valid write = %q", got)
	}
	// Existing non-write commands retain their clamped range semantics.
	e.executeCommand("99d")
	if len(e.lines) != 1 || e.lines[0] != "one" {
		t.Errorf("clamped delete = %#v", e.lines)
	}
}
