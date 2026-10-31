package builtins

import (
	"path/filepath"
	"strings"
)

const defaultPathExt = ".COM;.EXE;.BAT;.CMD"

// pathextNames lists the file names a command name can mean on Windows. A
// name that already ends in one of the PATHEXT extensions (compared without
// regard to case) stands for itself; any other name is tried with each
// extension appended, so python3.11 still finds python3.11.exe.
func pathextNames(name, pathext string) []string {
	if pathext == "" {
		pathext = defaultPathExt
	}
	var exts []string
	for _, e := range strings.Split(pathext, ";") {
		if e = strings.TrimSpace(e); e != "" {
			exts = append(exts, strings.ToLower(e))
		}
	}
	if have := strings.ToLower(filepath.Ext(name)); have != "" {
		for _, e := range exts {
			if e == have {
				return []string{name}
			}
		}
	}
	names := make([]string, len(exts))
	for i, e := range exts {
		names[i] = name + e
	}
	return names
}
