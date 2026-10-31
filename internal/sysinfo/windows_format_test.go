package sysinfo

import "testing"

func TestWindowsVersionFormatting(t *testing.T) {
	for _, tt := range []struct {
		product, display, build string
		ubr                     uint64
		os, distro              string
	}{
		{"Windows 10 Pro", "23H2", "22631", 3155, "Windows 11 Pro 23H2 (build 22631.3155)", "windows_11"},
		{"Windows 10 Home", "22H2", "19045", 0, "Windows 10 Home 22H2 (build 19045.0)", "windows_10"},
		{"Windows Server 2022", "", "20348", 12, "Windows Server 2022 (build 20348.12)", "windows"},
		{"", "", "", 0, "Windows", "windows"},
	} {
		got, distro := windowsVersion(tt.product, tt.display, tt.build, tt.ubr, tt.build != "", false)
		if got != tt.os || distro != tt.distro {
			t.Errorf("got %q, %q; want %q, %q", got, distro, tt.os, tt.distro)
		}
	}
}

func TestWindowsMemoryFormatting(t *testing.T) {
	if got := windowsMemory(8<<30, 3<<30); got != "5120MB / 8192MB (62%)" {
		t.Fatal(got)
	}
	if got := windowsMemory(0, 0); got != "?" {
		t.Fatal(got)
	}
	if got := windowsMemory(100, 101); got != "?" {
		t.Fatal(got)
	}
}

func TestWindowsUptimeFormatting(t *testing.T) {
	if got := windowsUptime(90061000); got != "1d 1h 1m" {
		t.Fatal(got)
	}
	if got := windowsUptime(0); got != "?" {
		t.Fatal(got)
	}
}

func TestWindowsServerNotRebranded(t *testing.T) {
	name, distro := windowsVersion("Windows 10 Enterprise", "", "26000", 1, true, true)
	if name != "Windows 10 Enterprise (build 26000.1)" || distro != "windows" {
		t.Fatalf("%q %q", name, distro)
	}
}

func TestWindowsVersionMissingUBR(t *testing.T) {
	name, _ := windowsVersion("Windows 10 Pro", "", "22621", 0, false, false)
	if name != "Windows 11 Pro (build 22621)" {
		t.Fatal(name)
	}
}

func TestWindowsVersionFallbackAndKernel(t *testing.T) {
	name, distro := windowsVersionFallback("", "Professional", "", 10, 0, 22631, 0, false, false)
	if name != "Windows 11 Professional (build 22631)" || distro != "windows_11" {
		t.Fatalf("%q %q", name, distro)
	}
	if got := windowsKernel(10, 0, 22631, 3155, true); got != "10.0.22631.3155" {
		t.Fatal(got)
	}
	if got := windowsKernel(0, 0, 0, 0, false); got != "?" {
		t.Fatal(got)
	}
	name, distro = windowsVersionFallback("", "", "", 10, 0, 22631, 0, false, true)
	if name != "Windows (build 22631)" || distro != "windows" {
		t.Fatalf("server fallback %q %q", name, distro)
	}
}

func TestWindowsStorageFormatting(t *testing.T) {
	if got := windowsStorageLine(`C:\`, 100<<30, 25<<30); got != `C:\: 75 GB / 100 GB (75%)` {
		t.Fatal(got)
	}
	if got := windowsStorageLine(`D:\`, 0, 0); got != "" {
		t.Fatal(got)
	}
	if got := windowsStorageLine(`D:\`, 10, 11); got != "" {
		t.Fatal(got)
	}
}
