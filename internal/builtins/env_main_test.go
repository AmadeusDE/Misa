package builtins

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/proceffect"
)

// TestMain pins the timezone and locale the expected outputs were written
// for, so results don't depend on the machine running them.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--nice-no-isolation-helper" {
		os.Exit(Run(context.Background(), "nice", &Context{Args: []string{"-n", "5", "true"}, Stderr: os.Stderr}))
	}
	_ = proceffect.EnableSelfHelper()
	if len(os.Args) > 1 && os.Args[1] == proceffect.Flag {
		os.Exit(proceffect.Handle(os.Args[2:], func(argv []string) int {
			if _, ok := Lookup(argv[0]); ok {
				return Run(context.Background(), argv[0], &Context{Args: argv[1:], Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr})
			}
			return proceffect.Exec(argv)
		}))
	}
	if len(os.Args) > 1 && os.Args[1] == "--nice-test-helper" {
		os.Exit(niceTestHelper(os.Args[2:]))
	}
	os.Setenv("TZ", "UTC")
	time.Local = time.UTC
	for _, k := range []string{"LC_ALL", "LC_CTYPE", "LC_COLLATE", "LC_MESSAGES", "LC_TIME", "LC_NUMERIC"} {
		os.Unsetenv(k)
	}
	os.Setenv("LANG", "en_US.UTF-8")
	os.Exit(m.Run())
}
