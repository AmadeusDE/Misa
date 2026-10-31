package vi

import "strings"

// setCmd handles the editor's supported boolean options. Multiple arguments
// are processed in order; an unknown option stops processing, like Vim.
func (e *Editor) setCmd(cmd string) {
	args := strings.Fields(strings.TrimPrefix(cmd, "set"))
	type option struct {
		name  string
		value *bool
	}
	options := []option{{"ignorecase", &e.ignoreCase}, {"hlsearch", &e.hlSearch}, {"incsearch", &e.incSearch}}
	format := func(o option) string {
		if !*o.value {
			return "no" + o.name
		}
		return o.name
	}
	if len(args) == 0 {
		var values []string
		for _, o := range options {
			values = append(values, format(o))
		}
		e.statusMsg = strings.Join(values, " ")
		return
	}
	var messages []string
	for _, arg := range args {
		original := arg
		query := strings.HasSuffix(arg, "?")
		toggle := strings.HasSuffix(arg, "!")
		if query || toggle {
			arg = arg[:len(arg)-1]
		}
		negate := false
		if strings.HasPrefix(arg, "inv") {
			toggle = true
			arg = strings.TrimPrefix(arg, "inv")
		} else if strings.HasPrefix(arg, "no") {
			negate = true
			arg = strings.TrimPrefix(arg, "no")
		}
		if arg == "cp" || arg == "compatible" {
			if negate && !query && !toggle {
				messages = append(messages, "nocompatible")
				continue
			}
		}
		var o *option
		switch arg {
		case "ic", "ignorecase":
			o = &options[0]
		case "hls", "hlsearch":
			o = &options[1]
		case "is", "incsearch":
			o = &options[2]
		}
		if o == nil || (query && (negate || toggle)) || (negate && toggle) {
			e.statusMsg = "Unknown option: " + original
			return
		}
		if !query {
			if toggle {
				*o.value = !*o.value
			} else {
				*o.value = !negate
			}
			if o.name == "hlsearch" && *o.value {
				e.searchHighlightSuppressed = false
			}
		}
		messages = append(messages, format(*o))
	}
	e.statusMsg = strings.Join(messages, " ")
}
