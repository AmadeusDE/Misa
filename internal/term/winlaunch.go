package term

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode/utf16"
)

// windowsCommandLine encodes arguments according to the Microsoft C runtime's
// backslash/quote parsing rules. The executable is supplied separately to
// CreateProcess, so every element is encoded with the same argument rules.
func windowsCommandLine(argv []string) (string, error) {
	var args []string
	for _, arg := range argv {
		if strings.ContainsRune(arg, 0) {
			return "", fmt.Errorf("argument contains NUL")
		}
		if arg != "" && !strings.ContainsAny(arg, " \t\"") {
			args = append(args, arg)
			continue
		}
		var b strings.Builder
		b.WriteByte('"')
		slashes := 0
		for _, c := range arg {
			if c == '\\' {
				slashes++
				continue
			}
			if c == '"' {
				b.WriteString(strings.Repeat(`\`, 2*slashes+1))
			} else {
				b.WriteString(strings.Repeat(`\`, slashes))
			}
			slashes = 0
			b.WriteRune(c)
		}
		b.WriteString(strings.Repeat(`\`, 2*slashes))
		b.WriteByte('"')
		args = append(args, b.String())
	}
	return strings.Join(args, " "), nil
}

// windowsEnvName recognizes the special =C:=value drive-current-directory
// entries in addition to ordinary NAME=value entries.
func windowsEnvName(entry string) (string, bool) {
	start := 0
	if strings.HasPrefix(entry, "=") {
		start = 1
	}
	i := strings.IndexByte(entry[start:], '=')
	if i < 0 || i+start == 0 {
		return "", false
	}
	return entry[:i+start], true
}

func windowsEnvBlock(env []string) ([]uint16, error) {
	entries := make([]string, 0, len(env))
	for _, entry := range env {
		if strings.ContainsRune(entry, 0) {
			return nil, fmt.Errorf("environment entry contains NUL")
		}
		name, ok := windowsEnvName(entry)
		if !ok {
			return nil, fmt.Errorf("invalid environment entry %q", entry)
		}
		found := false
		for i, old := range entries {
			oldName, _ := windowsEnvName(old)
			if strings.EqualFold(name, oldName) {
				entries[i] = entry
				found = true
				break
			}
		}
		if !found {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		a, _ := windowsEnvName(entries[i])
		b, _ := windowsEnvName(entries[j])
		return windowsEnvLess(a, b)
	})
	var block []uint16
	for _, entry := range entries {
		block = append(block, utf16.Encode([]rune(entry))...)
		block = append(block, 0)
	}
	block = append(block, 0)
	if len(entries) == 0 {
		block = append(block, 0)
	}
	return block, nil
}

// windowsEnvLess compares case-folded names by UTF-16 code units, including
// surrogate pairs, rather than by the UTF-8 representation used by Go strings.
func windowsEnvLess(a, b string) bool {
	au := utf16.Encode([]rune(strings.ToUpper(a)))
	bu := utf16.Encode([]rune(strings.ToUpper(b)))
	for i := 0; i < len(au) && i < len(bu); i++ {
		if au[i] != bu[i] {
			return au[i] < bu[i]
		}
	}
	return len(au) < len(bu)
}

func windowsEnvValue(env []string, key string) (string, bool) {
	for i := len(env) - 1; i >= 0; i-- {
		name, ok := windowsEnvName(env[i])
		if ok && strings.EqualFold(name, key) {
			return env[i][len(name)+1:], true
		}
	}
	return "", false
}

func winDrive(p string) bool {
	return len(p) >= 2 && ((p[0] >= 'A' && p[0] <= 'Z') || (p[0] >= 'a' && p[0] <= 'z')) && p[1] == ':'
}

// winAbsolute resolves a Windows path without relying on the host OS's path
// package semantics. Drive-relative paths require per-drive state we don't have.
func winAbsolute(p, dir string) (string, error) {
	p = strings.ReplaceAll(p, `\`, "/")
	dir = strings.ReplaceAll(dir, `\`, "/")
	if winDrive(p) && (len(p) == 2 || p[2] != '/') {
		return "", fmt.Errorf("drive-relative path %q is unsupported", p)
	}
	if strings.HasPrefix(p, "//") {
		parts := strings.SplitN(strings.TrimPrefix(p, "//"), "/", 3)
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return "", fmt.Errorf("invalid UNC path %q", p)
		}
		root := `\\` + parts[0] + `\` + parts[1]
		if len(parts) == 2 {
			return root, nil
		}
		return root + strings.ReplaceAll(path.Clean("/"+parts[2]), "/", `\`), nil
	}
	if winDrive(p) {
		return p[:2] + strings.ReplaceAll(path.Clean(p[2:]), "/", `\`), nil
	}
	if strings.HasPrefix(p, "/") {
		if !winDrive(dir) {
			return "", fmt.Errorf("rooted path requires a drive directory")
		}
		return winAbsolute(dir[:2]+p, dir)
	}
	if !(winDrive(dir) && len(dir) >= 3 && dir[2] == '/') && !strings.HasPrefix(dir, "//") {
		return "", fmt.Errorf("child directory must be absolute")
	}
	return winAbsolute(dir+"/"+p, dir)
}

func resolveWindowsExecutable(name, dir string, env []string, exists func(string) bool) (string, error) {
	if strings.ContainsRune(name, 0) || strings.ContainsRune(dir, 0) {
		return "", fmt.Errorf("path contains NUL")
	}
	if name == "" {
		return "", fmt.Errorf("empty executable name")
	}
	for _, e := range env {
		if strings.ContainsRune(e, 0) {
			return "", fmt.Errorf("environment contains NUL")
		}
	}
	normalized := strings.ReplaceAll(name, `\`, "/")
	ext := strings.ToLower(path.Ext(normalized))
	if ext == ".cmd" || ext == ".bat" {
		return "", fmt.Errorf("batch scripts require explicit cmd.exe /c (I01)")
	}
	if ext != "" && ext != ".exe" && ext != ".com" {
		return "", fmt.Errorf("unsupported executable extension %q", ext)
	}
	var extensions []string
	if ext != "" {
		extensions = []string{""}
	} else {
		pe, ok := windowsEnvValue(env, "PATHEXT")
		if !ok {
			pe = ".exe;.com"
		}
		for _, e := range strings.Split(pe, ";") {
			if strings.EqualFold(e, ".exe") || strings.EqualFold(e, ".com") {
				extensions = append(extensions, e)
			}
		}
	}
	var bases []string
	var skipped []string
	if strings.ContainsAny(name, `\/`) || winDrive(name) {
		base, err := winAbsolute(name, dir)
		if err != nil {
			return "", err
		}
		bases = []string{base}
	} else {
		p, _ := windowsEnvValue(env, "PATH")
		for _, d := range strings.Split(p, ";") {
			// PATH components may be enclosed in quotes, notably when edited by
			// installers. Quotes are delimiters, not part of the directory name.
			if len(d) >= 2 && d[0] == '"' && d[len(d)-1] == '"' {
				d = d[1 : len(d)-1]
			}
			if d == "" {
				continue
			}
			absolute, err := winAbsolute(d, dir)
			if err != nil {
				skipped = append(skipped, fmt.Sprintf("%q (%v)", d, err))
				continue
			}
			bases = append(bases, strings.TrimRight(absolute, `\`)+`\`+name)
		}
	}
	for _, base := range bases {
		for _, e := range extensions {
			if exists(base + e) {
				return base + e, nil
			}
		}
	}
	if len(skipped) != 0 {
		return "", fmt.Errorf("executable %q not found; skipped invalid PATH entries: %s", name, strings.Join(skipped, ", "))
	}
	return "", fmt.Errorf("executable %q not found", name)
}
