package vi

import (
	"fmt"
	"strings"
)

func (e *Editor) editorEnv(name string) (string, bool) {
	for i := len(e.shellEnv) - 1; i >= 0; i-- {
		key, value, ok := strings.Cut(e.shellEnv[i], "=")
		if ok && key == name {
			return value, true
		}
	}
	return "", false
}

func (e *Editor) expandEditPath(arg string) (string, error) {
	if strings.HasPrefix(arg, "+") {
		return "", fmt.Errorf("Edit +commands and ++options are unsupported")
	}
	var path strings.Builder
	if strings.HasPrefix(arg, "~") {
		if arg != "~" && !strings.HasPrefix(arg, "~/") {
			return "", fmt.Errorf("Edit ~user expansion is unsupported")
		}
		home, ok := e.editorEnv("HOME")
		if !ok || home == "" {
			return "", fmt.Errorf("HOME is not set")
		}
		path.WriteString(home)
		arg = arg[1:]
	}
	for arg != "" {
		i := strings.IndexAny(arg, "%#")
		if i < 0 {
			i = len(arg)
		}
		literal, err := e.expandEditEnv(arg[:i])
		if err != nil {
			return "", err
		}
		path.WriteString(literal)
		arg = arg[i:]
		if arg == "" {
			break
		}
		if len(arg) > 1 && arg[1] == ':' {
			return "", fmt.Errorf("Edit filename modifiers are unsupported")
		}
		name := e.filename
		if arg[0] == '#' {
			name = e.alternateFilename
		}
		if name == "" {
			return "", fmt.Errorf("No file name for %c", arg[0])
		}
		path.WriteString(name)
		arg = arg[1:]
	}
	if path.Len() == 0 {
		return "", fmt.Errorf("E32: No file name")
	}
	return e.resolveFile(path.String()), nil
}

func envNameByte(b byte, first bool) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || !first && b >= '0' && b <= '9'
}

func (e *Editor) expandEditEnv(arg string) (string, error) {
	var result strings.Builder
	for arg != "" {
		i := strings.IndexByte(arg, '$')
		if i < 0 {
			result.WriteString(arg)
			break
		}
		result.WriteString(arg[:i])
		arg = arg[i+1:]
		braced := strings.HasPrefix(arg, "{")
		if braced {
			arg = arg[1:]
		}
		i = 0
		for i < len(arg) && envNameByte(arg[i], i == 0) {
			i++
		}
		if i == 0 || braced && (i == len(arg) || arg[i] != '}') {
			return "", fmt.Errorf("Invalid environment expansion: use $NAME or ${NAME}")
		}
		name := arg[:i]
		value, ok := e.editorEnv(name)
		if !ok || value == "" {
			return "", fmt.Errorf("Environment variable is unset or empty: %s", name)
		}
		result.WriteString(value)
		arg = arg[i:]
		if braced {
			arg = arg[1:]
		}
	}
	return result.String(), nil
}
