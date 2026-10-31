package native

import (
	"os"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/builtinexec"
	"github.com/AmadeusDE/Misa/internal/proceffect"
)

// TestMain pins the timezone and locale the expected outputs were written
// for, so results don't depend on the machine running them.
func TestMain(m *testing.M) {
	_ = proceffect.EnableSelfHelper()
	if len(os.Args) > 1 && os.Args[1] == proceffect.Flag {
		os.Exit(builtinexec.ProcessEffect(os.Args[2:]))
	}
	os.Setenv("TZ", "UTC")
	time.Local = time.UTC
	for _, k := range []string{"LC_ALL", "LC_CTYPE", "LC_COLLATE", "LC_MESSAGES", "LC_TIME", "LC_NUMERIC"} {
		os.Unsetenv(k)
	}
	os.Setenv("LANG", "en_US.UTF-8")
	os.Exit(m.Run())
}
