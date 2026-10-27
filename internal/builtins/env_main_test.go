package builtins

import (
	"os"
	"testing"
	"time"
)

// TestMain pins the timezone and locale the expected outputs were written
// for, so results don't depend on the machine running them.
func TestMain(m *testing.M) {
	os.Setenv("TZ", "UTC")
	time.Local = time.UTC
	for _, k := range []string{"LC_ALL", "LC_CTYPE", "LC_COLLATE", "LC_MESSAGES", "LC_TIME", "LC_NUMERIC"} {
		os.Unsetenv(k)
	}
	os.Setenv("LANG", "en_US.UTF-8")
	os.Exit(m.Run())
}
