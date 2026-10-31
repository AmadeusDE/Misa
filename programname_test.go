package main

import (
	"runtime"
	"testing"
)

func TestProgramNameDropsExeOnWindows(t *testing.T) {
	want := "misa-gui.exe"
	if runtime.GOOS == "windows" {
		want = "misa-gui"
	}
	if got := programName("C:/bin/misa-gui.exe"); got != want {
		t.Errorf("programName = %q, want %q", got, want)
	}
}
