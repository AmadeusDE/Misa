package builtins

import (
	"os"
	"testing"
	"time"
)

// sleepHelperEnv makes the test binary behave as sleep(1); see TestMain.
const sleepHelperEnv = "MISA_TEST_SLEEP_HELPER"

// TestMain pins the timezone and locale the expected outputs were written
// for, so results don't depend on the machine running them.
func TestMain(m *testing.M) {
	if d := os.Getenv(sleepHelperEnv); d != "" {
		// Helper mode: act as a portable sleep(1) for process-matching tests,
		// independent of how the host packages its coreutils.
		if dur, err := time.ParseDuration(d + "s"); err == nil {
			time.Sleep(dur)
		}
		os.Exit(0)
	}
	os.Setenv("TZ", "UTC")
	time.Local = time.UTC
	for _, k := range []string{"LC_ALL", "LC_CTYPE", "LC_COLLATE", "LC_MESSAGES", "LC_TIME", "LC_NUMERIC"} {
		os.Unsetenv(k)
	}
	os.Setenv("LANG", "en_US.UTF-8")
	os.Exit(m.Run())
}
