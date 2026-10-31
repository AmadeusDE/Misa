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

// colorState belongs to one formatting call.
type colorState struct {
	extColors, extFold map[string]string
	firstUsed          bool
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

func (c *colorState) lookupColor(key string) string {
	if c.extColors != nil {
		if c, ok := c.extColors[key]; ok {
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
func (c *colorState) lookupExtColor(ext string) string {
	if ext == "" {
		return ""
	}
	key := "*" + ext
	if c.extColors != nil {
		if c, ok := c.extColors[key]; ok {
			return c
		}
	}
	if c.extFold != nil {
		if c, ok := c.extFold[strings.ToLower(key)]; ok {
			return c
		}
	}
	return ""
}

func ColorName(fi *FileInfo, opts FormatOpts, name string) string {
	if opts.NoColor {
		return name
	}
	state := opts.colors
	if state == nil {
		state = &colorState{}
	}
	code := state.fileColor(fi)
	if code == "" {
		return name
	}
	prefix := "\033[" + code + "m"
	if !state.firstUsed {
		prefix = "\033[0m" + prefix
		state.firstUsed = true
	}
	return prefix + name + "\033[0m"
}

func (c *colorState) fileColor(fi *FileInfo) string {
	if fi.IsSymlink {
		if fi.LinkBroken {
			return c.lookupColor("or")
		}
		return c.lookupColor("ln")
	}

	if fi.IsDir {
		mode := fi.Mode
		if mode&os.ModeSticky != 0 && mode&0002 != 0 {
			return c.lookupColor("tw")
		}
		if mode&os.ModeSticky != 0 {
			return c.lookupColor("st")
		}
		if mode&0002 != 0 {
			return c.lookupColor("ow")
		}
		return c.lookupColor("di")
	}

	if fi.Mode&os.ModeSocket != 0 {
		return c.lookupColor("so")
	}
	if fi.Mode&os.ModeNamedPipe != 0 {
		return c.lookupColor("pi")
	}
	if fi.Mode&os.ModeDevice != 0 {
		if fi.Mode&os.ModeCharDevice != 0 {
			return c.lookupColor("cd")
		}
		return c.lookupColor("bd")
	}

	if fi.Mode&os.ModeSetuid != 0 {
		return c.lookupColor("su")
	}
	if fi.Mode&os.ModeSetgid != 0 {
		return c.lookupColor("sg")
	}

	if fi.Mode&0111 != 0 {
		return c.lookupColor("ex")
	}

	if fi.Nlink > 1 {
		if mh := c.lookupColor("mh"); mh != "" && mh != "00" {
			return mh
		}
	}

	ext := extFromName(fi.Name)
	if ext != "" {
		if c := c.lookupExtColor(ext); c != "" {
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
