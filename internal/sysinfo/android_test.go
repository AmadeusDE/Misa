package sysinfo

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestDetectPlatformTermuxAppAndLinux(t *testing.T) {
	prop := func(name string) string {
		switch name {
		case "ro.build.version.release":
			return "14"
		case "ro.product.model":
			return "Pixel 8"
		case "ro.product.manufacturer":
			return "Google"
		default:
			t.Errorf("unexpected property %q", name)
			return ""
		}
	}
	termux := detectPlatformFrom(func(k string) string {
		if k == "TERMUX_VERSION" {
			return "0.118.0"
		}
		if k == "PREFIX" {
			return "/data/data/com.termux/files/usr"
		}
		return ""
	}, prop)
	if !termux.Android || !termux.Termux || termux.App || termux.distroID() != "termux" {
		t.Fatalf("termux: %+v", termux)
	}
	if termux.osName() != "Termux (Android 14)" || termux.hostName() != "Google Pixel 8" {
		t.Fatalf("names: %q %q", termux.osName(), termux.hostName())
	}

	prefixOnly := detectPlatformFrom(func(k string) string {
		if k == "PREFIX" {
			return "/data/data/com.termux/files/usr"
		}
		return ""
	}, func(string) string { return "" })
	if !prefixOnly.Termux || prefixOnly.osName() != "Termux" {
		t.Fatalf("prefix: %+v", prefixOnly)
	}

	app := detectPlatformFrom(func(k string) string {
		switch k {
		case "MISA_ANDROID_APP":
			return "1"
		case "TERMUX_VERSION":
			return "0.118.0"
		default:
			return ""
		}
	}, prop)
	if !app.App || app.Termux || app.distroID() != "android" || app.osName() != "Android 14" {
		t.Fatalf("app should win over Termux: %+v", app)
	}
	if app.hostName() != "Google Pixel 8" {
		t.Fatalf("host %q", app.hostName())
	}

	if runtime.GOOS == "android" {
		return
	}
	linux := detectPlatformFrom(func(string) string { return "" }, func(string) string {
		t.Fatal("getprop on a non-Android host")
		return ""
	})
	if linux.Android || linux.distroID() != "" {
		t.Fatalf("linux: %+v", linux)
	}
}

func TestGetDistroUsesAndroidDetection(t *testing.T) {
	t.Setenv("TERMUX_VERSION", "0.118.0")
	t.Setenv("PREFIX", "/data/data/com.termux/files/usr")
	t.Setenv("MISA_ANDROID_APP", "")
	old := readProperty
	readProperty = func(name string) string {
		if name == "ro.build.version.release" {
			return "14"
		}
		if name == "ro.product.manufacturer" {
			return "Google"
		}
		if name == "ro.product.model" {
			return "Pixel"
		}
		return ""
	}
	t.Cleanup(func() { readProperty = old })

	if got := getDistro(); got != "termux" {
		t.Fatalf("distro %q", got)
	}
	if got := getOS(); !strings.Contains(got, "Termux") || !strings.Contains(got, "14") {
		t.Fatalf("os %q", got)
	}
	if got := getHost(); got != "Google Pixel" {
		t.Fatalf("host %q", got)
	}

	t.Setenv("MISA_ANDROID_APP", "1")
	if got := getDistro(); got != "android" {
		t.Fatalf("app distro %q", got)
	}
	if got := getOS(); got != "Android 14" {
		t.Fatalf("app os %q", got)
	}
}

func TestProcPermissionIsDash(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("Unix file modes: chmod 0 does not make a file unreadable on Windows")
	}
	dir := t.TempDir()
	for _, name := range []string{"meminfo", "uptime"} {
		path := dir + "/" + name
		if err := os.WriteFile(path, []byte("MemTotal: 1024 kB\nMemFree: 512 kB\n1.0 1.0\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0); err != nil {
			t.Fatal(err)
		}
	}
	old := procRoot
	procRoot = dir
	t.Cleanup(func() { procRoot = old })

	if got := getMemory(); got != "—" {
		t.Fatalf("memory %q", got)
	}
	if got := getSwap(); got != "—" {
		t.Fatalf("swap %q", got)
	}
	if got := getUptime(); got != "—" {
		t.Fatalf("uptime %q", got)
	}
}
