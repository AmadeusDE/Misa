package jobleader

import (
	"runtime"
	"testing"
)

func TestSupportedMatchesPlatform(t *testing.T) {
	if want := runtime.GOOS != "windows"; Supported != want {
		t.Errorf("Supported = %v on %s", Supported, runtime.GOOS)
	}
}
