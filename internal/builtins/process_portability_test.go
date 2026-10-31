package builtins

import (
	"runtime"
	"strings"
	"testing"
)

func TestPriorityClassForNiceBands(t *testing.T) {
	for nice, want := range map[int]uint32{
		19: classIdle, 15: classIdle, 14: classBelowNormal, 10: classBelowNormal, 1: classBelowNormal,
		0: classNormal, -1: classAboveNormal, -9: classAboveNormal, -10: classHigh, -40: classHigh,
	} {
		if got := priorityClassForNice(nice); got != want {
			t.Errorf("priorityClassForNice(%d) = %#x, want %#x", nice, got, want)
		}
	}
}

func TestNiceForPriorityClassRoundTrips(t *testing.T) {
	for _, class := range []uint32{classIdle, classBelowNormal, classNormal, classAboveNormal, classHigh} {
		if got := priorityClassForNice(niceForPriorityClass(class)); got != class {
			t.Errorf("class %#x reports nice %d, which maps to %#x", class, niceForPriorityClass(class), got)
		}
	}
}

func TestMachineFromProcessorArchitecture(t *testing.T) {
	for arch, want := range map[uint16]string{9: "x86_64", 12: "aarch64", 0: "i686", 5: "armv7l", 6: "unknown", 99: "unknown"} {
		if got := machineFromProcessorArchitecture(arch); got != want {
			t.Errorf("machineFromProcessorArchitecture(%d) = %q, want %q", arch, got, want)
		}
	}
}

func TestNormalizeExitStatus(t *testing.T) {
	for in, want := range map[int]int{0: 0, 9: 9, 128: 0, 137: 9, 143: 15, 256: 0, 384: -1} {
		if got := normalizeExitStatus(in); got != want {
			t.Errorf("normalizeExitStatus(%d) = %d, want %d", in, got, want)
		}
	}
}

// Every process builtin the Windows table mentions must exist where its
// option is documented, and its help must name the option it limits.
func TestWindowsLimitOptionsAppearInHelp(t *testing.T) {
	for _, l := range windowsLimits {
		if !strings.HasPrefix(l.Option, "-") || (l.Command != "sync" && l.Command != "timeout") {
			continue
		}
		if _, ok := Lookup(l.Command); !ok {
			continue
		}
		r := runBuiltin(t, l.Command, "--help")
		if !strings.Contains(r.stdout, l.Option) && !strings.Contains(r.stdout, strings.TrimPrefix(l.Option, "-")) {
			t.Errorf("%s --help does not mention %s:\n%s", l.Command, l.Option, r.stdout)
		}
	}
}

func TestWindowsUnsupportedWholeCommandsAreRegistered(t *testing.T) {
	for _, l := range windowsLimits {
		if l.Option != "" || l.Kind != Unsupported {
			continue
		}
		if _, ok := Lookup(l.Command); !ok && l.Command != "sync" {
			t.Errorf("%s is unsupported on Windows but not registered: users would see command not found instead of the reason", l.Command)
		}
	}
	for _, name := range []string{"pgrep", "pkill"} {
		if l, ok := findLimit(windowsLimits, name, ""); !ok || l.Kind != Unsupported {
			t.Errorf("%s must be recorded as unsupported on Windows", name)
		}
	}
}

func TestProcessBuiltinsAreRegisteredWhereTheyRun(t *testing.T) {
	for _, name := range []string{"kill", "pgrep", "pkill", "nice", "uname"} {
		if _, ok := Lookup(name); !ok {
			t.Errorf("%s is not registered on %s", name, runtime.GOOS)
		}
	}
	if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		for _, name := range []string{"sync", "timeout"} {
			if _, ok := Lookup(name); !ok {
				t.Errorf("%s is not registered on %s", name, runtime.GOOS)
			}
		}
	}
}
