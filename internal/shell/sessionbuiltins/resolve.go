package sessionbuiltins

import (
	"strings"

	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/AmadeusDE/Misa/internal/commandexec"
	"github.com/AmadeusDE/Misa/internal/shell/session"
)

// Configure fills a session's execution and resolution hooks from this adapter,
// so that command execution, `type`, `command -v`, and `hash` all agree.
func Configure(o *session.Options) *session.Options {
	o.Exec = Run
	o.MisaBuiltin = MisaBuiltin
	o.LookPath = LookPath
	return o
}

// MisaBuiltin reports whether name is a Misa registry utility. Explicit paths
// are never registry utilities, so `/bin/ls` stays external.
func MisaBuiltin(name string) bool {
	if name == "" || strings.ContainsAny(name, `/\`) {
		return false
	}
	_, ok := builtins.Lookup(name)
	return ok
}

// LookPath resolves an executable name through the invocation's PATH using the
// same F06 rules as execution. It never consults the Misa registry, which is
// why `type -p` and `command -p` can select an external utility.
func LookPath(name, dir string, env []string) (string, bool) {
	path, err := commandexec.Resolve(name, commandexec.Invocation{Dir: dir, Env: env})
	if err != nil {
		return "", false
	}
	return path, true
}
