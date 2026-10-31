// Tar option handling informed by Toybox tar.c (0BSD).
package builtins

import (
	"fmt"
	"strings"
)

const tarSpec = "c(create)t(list)x(extract)f(file):C(directory):T(files-from):v(verbose)h(dereference)O(to-stdout)z(gzip)j(bzip2)J(xz)a(auto-compress)k(keep-old-files)(keep-old)p(same-permissions)o(no-same-owner)m(touch)(overwrite)(strip-components):(null)(numeric-owner)(owner):(group):(exclude):"

var tarFlags *Spec

func init() {
	var err error
	tarFlags, err = ParseSpec(tarSpec)
	if err != nil {
		panic(err)
	}
	Register(Command{Name: "tar", Run: runTar})
}

type tarEvent struct{ kind, value string }

// tarScan preserves option/operand order, unlike the general permutation parser.
func tarScan(args []string) ([]tarEvent, error) {
	args = append([]string(nil), args...)
	if len(args) > 0 && len(args[0]) > 0 && args[0][0] != '-' && strings.ContainsAny(args[0], "ctx") {
		old := true
		for i := range len(args[0]) {
			if _, ok := tarFlags.byShort[args[0][i]]; !ok {
				old = false
				break
			}
		}
		if old {
			cluster := args[0]
			rest := args[1:]
			var expanded []string
			for i := range len(cluster) {
				def := tarFlags.byShort[cluster[i]]
				expanded = append(expanded, "-"+string(cluster[i]))
				if def.kind == kindString && len(rest) > 0 {
					expanded = append(expanded, rest[0])
					rest = rest[1:]
				}
			}
			args = append(expanded, rest...)
		}
	}
	var events []tarEvent
	end := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !end && a == "--" {
			end = true
			continue
		}
		if end || a == "-" || !strings.HasPrefix(a, "-") {
			events = append(events, tarEvent{"operand", a})
			continue
		}
		if a == "--help" || a == "--version" {
			events = append(events, tarEvent{a, ""})
			continue
		}
		if strings.HasPrefix(a, "--") {
			name, val, has := strings.Cut(a[2:], "=")
			def, ok := tarFlags.byLong[name]
			if !ok {
				return nil, fmt.Errorf("unknown option --%s", name)
			}
			if def.kind == kindString && !has {
				i++
				if i == len(args) {
					return nil, fmt.Errorf("--%s requires an argument", name)
				}
				val = args[i]
			}
			token := "--" + name
			if def.kind == kindString || has {
				token += "=" + val
			}
			if _, err := tarFlags.ParseArgs([]string{token}); err != nil {
				return nil, err
			}
			events = append(events, tarEvent{def.key(), val})
			continue
		}
		for j := 1; j < len(a); j++ {
			def, ok := tarFlags.byShort[a[j]]
			if !ok {
				return nil, fmt.Errorf("unknown option -%c", a[j])
			}
			val := ""
			if def.kind == kindString {
				if j+1 < len(a) {
					val = a[j+1:]
					j = len(a)
				} else {
					i++
					if i == len(args) {
						return nil, fmt.Errorf("-%c requires an argument", def.short)
					}
					val = args[i]
				}
			}
			token := "-" + string(def.short)
			var argv = []string{token}
			if def.kind == kindString {
				argv = append(argv, val)
			}
			if _, err := tarFlags.ParseArgs(argv); err != nil {
				return nil, err
			}
			events = append(events, tarEvent{def.key(), val})
		}
	}
	return events, nil
}

func tarHelpVersion(ctx *Context, kind string) {
	local := *ctx
	if local.CmdName == "" {
		local.CmdName = "tar"
	}
	if kind == "--help" {
		usage := tarFlags.GenerateUsage(local.CmdName)
		// Unlike most builtins, tar reserves -h for dereferencing symlinks.
		fmt.Fprint(ctx.Stdout, strings.Replace(usage, "  -h, --help", "      --help", 1))
		fmt.Fprintln(ctx.Stdout, "Supports archive creation, listing and extraction, with gzip compression.")
		return
	}
	HandleHelpVersion(&local, &ParseResult{VersionRequested: true}, tarFlags)
}
