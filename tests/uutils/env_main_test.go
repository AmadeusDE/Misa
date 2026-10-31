package uutils

import (
	"os"
	"testing"

	"github.com/AmadeusDE/Misa/internal/builtinexec"
	"github.com/AmadeusDE/Misa/internal/proceffect"
)

func TestMain(m *testing.M) {
	_ = proceffect.EnableSelfHelper()
	if len(os.Args) > 1 && os.Args[1] == proceffect.Flag {
		os.Exit(builtinexec.ProcessEffect(os.Args[2:]))
	}
	os.Exit(m.Run())
}
