package lscore

import (
	"os"
	"strings"
)

// defaultColors mirrors GNU coreutils' built-in dircolors defaults (the
// values `dircolors --print-database` reports when LS_COLORS is unset),
// so ls looks right without requiring the user to configure LS_COLORS.
var defaultColors = map[string]string{
	"di": "01;34",    // directory: bold blue
	"ln": "01;36",    // symlink: bold cyan
	"or": "40;31;01", // orphan symlink: bold red on black
	"ex": "01;32",    // executable: bold green
	"pi": "40;33",    // pipe: yellow on black
	"so": "01;35",    // socket: bold magenta
	"bd": "40;33;01", // block device: bold yellow on black
	"cd": "40;33;01", // char device: bold yellow on black
	"su": "37;41",    // setuid: white on red
	"sg": "30;43",    // setgid: black on yellow
	"tw": "30;42",    // sticky+other-write: black on green
	"ow": "34;42",    // other-writable: blue on green
	"st": "37;44",    // sticky: white on blue
}

var extColors map[string]string

// extFold maps a lowercased extension to the single color it resolves to
// when every case-variant of that extension in LS_COLORS agrees on the
// same value. When case-variants disagree, the extension is absent here
// and only an exact-case match in extColors applies.
var extFold map[string]string

func init() {
	extColors, extFold = parseLSColors(os.Getenv("LS_COLORS"))
}

func parseLSColors(val string) (map[string]string, map[string]string) {
	if val == "" {
		return nil, nil
	}
	m := make(map[string]string)
	for _, entry := range strings.Split(val, ":") {
		k, v, ok := strings.Cut(entry, "=")
		if !ok || k == "" {
			continue
		}
		m[k] = v
	}

	// Group extension patterns by their case-folded form. A fold entry is
	// only usable for case-insensitive matching when every literal
	// spelling in that group ultimately resolved to the same color.
	groups := make(map[string]map[string]bool)
	for k, v := range m {
		if !strings.HasPrefix(k, "*.") {
			continue
		}
		lower := strings.ToLower(k)
		if groups[lower] == nil {
			groups[lower] = map[string]bool{}
		}
		groups[lower][v] = true
	}
	fold := make(map[string]string)
	for lower, vals := range groups {
		if len(vals) == 1 {
			for v := range vals {
				fold[lower] = v
			}
		}
	}
	return m, fold
}

func lookupColor(key string) string {
	if extColors != nil {
		if c, ok := extColors[key]; ok {
			return c
		}
	}
	if c, ok := defaultColors[key]; ok {
		return c
	}
	return ""
}

// lookupExtColor resolves the color for a file extension (including the
// leading dot, e.g. ".jpg"), honoring GNU ls's case-sensitivity rules:
// an exact-case match always wins; otherwise a case-insensitive match is
// used only if every case-variant of that extension maps to the same color.
func lookupExtColor(ext string) string {
	if ext == "" {
		return ""
	}
	key := "*" + ext
	if extColors != nil {
		if c, ok := extColors[key]; ok {
			return c
		}
	}
	if extFold != nil {
		if c, ok := extFold[strings.ToLower(key)]; ok {
			return c
		}
	}
	return ""
}

// colorFirstUsed tracks whether a color escape has already been emitted
// during the current Format() invocation. GNU ls only emits the leading
// "reset to normal" sequence before the first colored name it prints in a
// given run; later colored names skip that leading reset. ls invocations
// run one at a time within a single process, so this package-level flag
// (reset by Format at the start of each call) is safe.
var colorFirstUsed bool

func resetColorState() {
	colorFirstUsed = false
}

func ColorName(fi *FileInfo, opts FormatOpts, name string) string {
	if opts.NoColor {
		return name
	}
	code := fileColor(fi)
	if code == "" {
		return name
	}
	prefix := "\033[" + code + "m"
	if !colorFirstUsed {
		prefix = "\033[0m" + prefix
		colorFirstUsed = true
	}
	return prefix + name + "\033[0m"
}

func fileColor(fi *FileInfo) string {
	if fi.IsSymlink {
		if fi.LinkBroken {
			return lookupColor("or")
		}
		return lookupColor("ln")
	}

	if fi.IsDir {
		mode := fi.Mode
		if mode&os.ModeSticky != 0 && mode&0002 != 0 {
			return lookupColor("tw")
		}
		if mode&os.ModeSticky != 0 {
			return lookupColor("st")
		}
		if mode&0002 != 0 {
			return lookupColor("ow")
		}
		return lookupColor("di")
	}

	if fi.Mode&os.ModeSocket != 0 {
		return lookupColor("so")
	}
	if fi.Mode&os.ModeNamedPipe != 0 {
		return lookupColor("pi")
	}
	if fi.Mode&os.ModeDevice != 0 {
		if fi.Mode&os.ModeCharDevice != 0 {
			return lookupColor("cd")
		}
		return lookupColor("bd")
	}

	if fi.Mode&os.ModeSetuid != 0 {
		return lookupColor("su")
	}
	if fi.Mode&os.ModeSetgid != 0 {
		return lookupColor("sg")
	}

	if fi.Mode&0111 != 0 {
		return lookupColor("ex")
	}

	if fi.Nlink > 1 {
		if mh := lookupColor("mh"); mh != "" && mh != "00" {
			return mh
		}
	}

	ext := extFromName(fi.Name)
	if ext != "" {
		if c := lookupExtColor(ext); c != "" {
			return c
		}
	}

	return ""
}

func extFromName(name string) string {
	for i := len(name) - 1; i > 0; i-- {
		if name[i] == '.' {
			return name[i:]
		}
	}
	return ""
}
