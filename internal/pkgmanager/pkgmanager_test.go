package pkgmanager

import (
	"testing"
)

func names(ms []Manager) map[string]Manager {
	out := map[string]Manager{}
	for _, m := range ms {
		out[m.Name] = m
	}
	return out
}

func TestTermuxPkgDefinition(t *testing.T) {
	t.Setenv("MISA_ANDROID_APP", "")
	t.Setenv("TERMUX_VERSION", "0.118.0")
	t.Setenv("PREFIX", "/data/data/com.termux/files/usr")
	got := names(All())
	pkg, ok := got["pkg"]
	if !ok {
		t.Fatal("Termux pkg menu missing")
	}
	if !pkg.Distro || pkg.Detect != "pkg" {
		t.Fatalf("pkg = %+v", pkg)
	}
	var sawSearch, sawInstall, sawRemove, sawUpgrade bool
	for _, a := range pkg.Actions {
		switch a.Title {
		case "Search":
			sawSearch = a.ShellPrefill == "pkg search "
		case "Install":
			sawInstall = a.ShellPrefill == "pkg install "
		case "Remove":
			sawRemove = a.ShellPrefill == "pkg uninstall "
		case "Upgrade":
			sawUpgrade = len(a.Run) == 2 && a.Run[0] == "pkg" && a.Run[1] == "upgrade"
		}
	}
	if !sawSearch || !sawInstall || !sawRemove || !sawUpgrade {
		t.Fatalf("actions = %+v", pkg.Actions)
	}
	if _, ok := got["Git"]; !ok {
		t.Fatal("git menu missing on Termux")
	}
}

func TestAndroidAppHidesDistroMenus(t *testing.T) {
	t.Setenv("MISA_ANDROID_APP", "1")
	t.Setenv("TERMUX_VERSION", "0.118.0")
	t.Setenv("PREFIX", "/data/data/com.termux/files/usr")
	got := names(All())
	for _, name := range []string{"Pacman", "Flatpak", "pkg"} {
		if _, ok := got[name]; ok {
			t.Errorf("distro menu %s visible in the APK", name)
		}
	}
	if _, ok := got["Git"]; !ok {
		t.Fatal("git menu hidden with the distro menus")
	}
}

func TestLinuxOmitsTermuxPkg(t *testing.T) {
	t.Setenv("MISA_ANDROID_APP", "")
	t.Setenv("TERMUX_VERSION", "")
	t.Setenv("PREFIX", "")
	got := names(builtins())
	if _, ok := got["pkg"]; ok {
		t.Fatal("pkg menu shown without Termux")
	}
	if _, ok := got["Pacman"]; !ok {
		t.Fatal("pacman menu missing on linux")
	}
}
