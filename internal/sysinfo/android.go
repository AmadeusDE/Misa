package sysinfo

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// platform is the Android environment Misa is running in. Termux and the
// native APK are both Android; the APK sets MISA_ANDROID_APP=1 and is not
// treated as Termux.
type platform struct {
	Android      bool
	Termux       bool
	App          bool
	Release      string
	Model        string
	Manufacturer string
}

func detectPlatform() platform {
	return detectPlatformFrom(os.Getenv, readProperty)
}

// readProperty returns one Android system property. The cgo reader
// (__system_property_get) wins when this binary is linked with it; otherwise
// /system/bin/getprop is used. Tests replace it.
var readProperty = func(name string) string {
	if v := systemProperty(name); v != "" {
		return v
	}
	return getpropCommand(name)
}

func getpropCommand(name string) string {
	out, err := exec.Command("/system/bin/getprop", name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectPlatformFrom(getenv func(string) string, prop func(string) string) platform {
	app := getenv("MISA_ANDROID_APP") == "1"
	termux := getenv("TERMUX_VERSION") != "" || strings.Contains(getenv("PREFIX"), "/com.termux/")
	if !app && !termux && runtime.GOOS != "android" {
		return platform{}
	}
	if prop == nil {
		prop = func(string) string { return "" }
	}
	return platform{
		Android:      true,
		Termux:       termux && !app,
		App:          app,
		Release:      prop("ro.build.version.release"),
		Model:        prop("ro.product.model"),
		Manufacturer: prop("ro.product.manufacturer"),
	}
}

func (p platform) distroID() string {
	if !p.Android {
		return ""
	}
	if p.Termux {
		return "termux"
	}
	return "android"
}

func (p platform) osName() string {
	if !p.Android {
		return ""
	}
	if p.Termux {
		if p.Release != "" {
			return "Termux (Android " + p.Release + ")"
		}
		return "Termux"
	}
	if p.Release != "" {
		return "Android " + p.Release
	}
	return "Android"
}

func (p platform) hostName() string {
	man := strings.TrimSpace(p.Manufacturer)
	model := strings.TrimSpace(p.Model)
	switch {
	case man == "":
		return model
	case model == "":
		return man
	case strings.HasPrefix(strings.ToLower(model), strings.ToLower(man)):
		return model
	default:
		return man + " " + model
	}
}
