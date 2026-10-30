package builtins

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const envSpec = "i(ignore-environment)u(unset):0(null)"

var envSpecParsed *Spec

func init() {
	var err error
	envSpecParsed, err = ParseSpec(envSpec)
	if err != nil {
		panic("builtins: bad env spec: " + err.Error())
	}
	Register(Command{Name: "env", Run: runEnv})
	Register(Command{Name: "printenv", Run: runPrintenv})
}

func runEnv(ctx *Context) int {
	ignoreEnv := false
	sep := "\n"
	unsetMap := make(map[string]bool)
	var envAssignments []string

	args := append([]string(nil), ctx.Args...)
	cmdIdx := len(args)
	workDir := ctx.Dir
	options := true

	for i := 0; i < len(args); i++ {
		a := args[i]
		if options && a == "--" {
			options = false
			continue
		}
		if options && a == "-C" {
			if i+1 >= len(args) {
				fmt.Fprintln(ctx.Stderr, "env: option requires an argument -- 'C'")
				return 125
			}
			i++
			workDir = resolvePath(&Context{Dir: workDir}, args[i])
			continue
		}
		if options && strings.HasPrefix(a, "-C") && len(a) > 2 {
			workDir = resolvePath(&Context{Dir: workDir}, a[2:])
			continue
		}
		if options && strings.HasPrefix(a, "-iC") {
			ignoreEnv = true
			value := a[3:]
			if value == "" {
				if i+1 >= len(args) {
					fmt.Fprintln(ctx.Stderr, "env: option requires an argument -- 'C'")
					return 125
				}
				i++
				value = args[i]
			}
			workDir = resolvePath(&Context{Dir: workDir}, value)
			continue
		}
		if options && (a == "-0" || a == "--null") {
			sep = "\x00"
			continue
		}
		if options && (a == "-i" || a == "--ignore-environment" || a == "-") {
			ignoreEnv = true
			continue
		}
		if options && (a == "--block-signal" || a == "--ignore-signal" || a == "--default-signal") {
			if i+1 >= len(args) {
				fmt.Fprintf(ctx.Stderr, "env: option requires an argument -- '%s'\n", strings.TrimPrefix(a, "--"))
				return 125
			}
			i++
			if args[i] == "__ALL__" {
				fmt.Fprintf(ctx.Stderr, "env: invalid signal: '%s'\n", args[i])
				return 125
			}
			continue
		}
		if options && (strings.HasPrefix(a, "--block-signal=") || strings.HasPrefix(a, "--ignore-signal=") || strings.HasPrefix(a, "--default-signal=")) {
			value := a[strings.IndexByte(a, '=')+1:]
			if value == "__ALL__" {
				fmt.Fprintf(ctx.Stderr, "env: invalid signal: '%s'\n", value)
				return 125
			}
			continue
		}
		if options && (a == "-u" || a == "--unset") {
			if i+1 < len(args) {
				unsetMap[args[i+1]] = true
				i++
			}
			continue
		}
		if options && strings.HasPrefix(a, "-u") {
			unsetMap[a[2:]] = true
			continue
		}
		if options && (a == "-S" || a == "--split-string" || strings.HasPrefix(a, "-S") || strings.HasPrefix(a, "--split-string=")) {
			var sStr string
			if a == "-S" || a == "--split-string" {
				if i+1 >= len(args) {
					fmt.Fprintln(ctx.Stderr, "env: option requires an argument -- 'S'")
					return 125
				}
				i++
				sStr = args[i]
			} else if strings.HasPrefix(a, "-S") {
				sStr = a[2:]
			} else {
				sStr = a[15:]
			}

			// Construct current envMap for variable expansion in -S
			currentEnvMap := make(map[string]string)
			if !ignoreEnv {
				sourceEnv := ctx.Environ()
				for _, e := range sourceEnv {
					if eqIdx := strings.IndexByte(e, '='); eqIdx >= 0 {
						key := e[:eqIdx]
						val := e[eqIdx+1:]
						if !unsetMap[key] {
							currentEnvMap[key] = val
						}
					}
				}
			}
			for _, op := range envAssignments {
				if eqIdx := strings.IndexByte(op, '='); eqIdx >= 0 {
					currentEnvMap[op[:eqIdx]] = op[eqIdx+1:]
				}
			}
			for u := range unsetMap {
				delete(currentEnvMap, u)
			}

			words, err := parseSplitString(sStr, currentEnvMap)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "env: %v\n", err)
				return 125
			}
			args = append(args[:i], append(words, args[i+1:]...)...)
			i--
			continue
		}
		if options && strings.HasPrefix(a, "-") && len(a) > 1 && !strings.Contains(a, "=") {
			if a == "--help" {
				fmt.Fprintln(ctx.Stdout, "Usage: env [OPTION]... [-] [NAME=VALUE]... [COMMAND [ARG]...]\n\nOptions:\n  -i, --ignore-environment  start with an empty environment\n  -u, --unset=NAME          remove variable from the environment\n  -C, --chdir=DIR           change the working directory\n  -0, --null                end each output line with NUL")
				return 0
			}
			if a == "--version" {
				fmt.Fprintln(ctx.Stdout, "env (Misa) 0.1.0")
				return 0
			}
			fmt.Fprintf(ctx.Stderr, "env: invalid option -- '%s'\n", strings.TrimPrefix(a, "-"))
			return 125
		}
		if strings.Contains(a, "=") {
			envAssignments = append(envAssignments, a)
			continue
		}
		cmdIdx = i
		break
	}

	if sep == "\x00" && cmdIdx < len(args) {
		fmt.Fprintln(ctx.Stderr, "env: cannot specify --null (-0) with command")
		return 125
	}

	envMap := make(map[string]string)
	if !ignoreEnv {
		sourceEnv := ctx.Environ()
		for _, e := range sourceEnv {
			eqIdx := strings.IndexByte(e, '=')
			if eqIdx >= 0 {
				key := e[:eqIdx]
				val := e[eqIdx+1:]
				if key == "" {
					// Entry like =BLAH or =BLAH=val
					if secondEq := strings.IndexByte(val, '='); secondEq >= 0 {
						key = "=" + val[:secondEq]
						val = val[secondEq+1:]
					} else {
						key = "=" + val
						val = ""
					}
				}
				if !unsetMap[key] {
					envMap[key] = val
				}
			}
		}
	}

	for _, op := range envAssignments {
		eqIdx := strings.IndexByte(op, '=')
		if eqIdx >= 0 {
			key := op[:eqIdx]
			val := op[eqIdx+1:]
			if key == "" {
				fmt.Fprintf(ctx.Stderr, "env: warning: no name specified for value '%s'\n", val)
			} else {
				envMap[key] = val
			}
		}
	}

	for u := range unsetMap {
		delete(envMap, u)
	}

	if cmdIdx < len(args) {
		subCmdName := args[cmdIdx]
		subArgs := args[cmdIdx+1:]

		subCtx := *ctx
		subCtx.Dir, subCtx.Args, subCtx.CmdName = workDir, subArgs, subCmdName
		subCtx.Env = make([]string, 0, len(envMap))
		for k, v := range envMap {
			subCtx.Env = append(subCtx.Env, k+"="+v)
		}
		if ctx.Exec != nil {
			execName := subCmdName
			// TODO(F06): decide cwd poisoning and resolution policy in the shared launcher.
			if !strings.ContainsAny(subCmdName, `/\`) {
				localPath, err := filepath.Abs(filepath.Join(workDir, subCmdName))
				if err == nil {
					if info, err := os.Stat(localPath); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
						execName = localPath
					}
				}
			}
			return ctx.Exec(ctx.CancellationContext(), execName, &subCtx)
		}

		// Keep the legacy local executable preference when no executor is injected.
		localPath := subCmdName
		if !filepath.IsAbs(localPath) {
			localPath = filepath.Join(workDir, localPath)
		}
		if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
			if info.Mode()&0111 == 0 {
				fmt.Fprintf(ctx.Stderr, "env: '%s': Permission denied\n", subCmdName)
				return 126
			}
			cmd := exec.CommandContext(ctx.CancellationContext(), localPath, subArgs...)
			cmd.Dir, cmd.Env = workDir, subCtx.Env
			cmd.Stdout, cmd.Stderr, cmd.Stdin = ctx.Stdout, ctx.Stderr, ctx.Stdin
			if err := cmd.Run(); err != nil {
				if ctx.CancellationContext().Err() != nil {
					return ExitStatus(ctx.CancellationContext().Err())
				}
				if exitErr, ok := err.(*exec.ExitError); ok {
					return exitErr.ExitCode()
				}
				return 1
			}
			return 0
		}

		cmd, ok := Lookup(subCmdName)
		if !ok {
			cmd, ok = Lookup(filepath.Base(subCmdName))
		}
		if ok {
			return cmd.Run(&subCtx)
		}

		if strings.HasPrefix(subCmdName, "'-") || (strings.HasPrefix(subCmdName, "'") && strings.Contains(subCmdName, "-")) {
			fmt.Fprintf(ctx.Stderr, "env: %s: No such file or directory\nenv: use -[v]S to pass options in shebang lines\n", subCmdName)
			return 127
		}
		fmt.Fprintf(ctx.Stderr, "env: '%s': No such file or directory\n", subCmdName)
		return 127
	}

	var keys []string
	for k := range envMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if strings.HasPrefix(k, "=") {
			fmt.Fprintf(ctx.Stdout, "%s%s%s", k, envMap[k], sep)
		} else {
			fmt.Fprintf(ctx.Stdout, "%s=%s%s", k, envMap[k], sep)
		}
	}
	return 0
}

func parseSplitString(s string, envMap map[string]string) ([]string, error) {
	var words []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escaped {
			current.WriteByte(ch)
			escaped = false
			continue
		}

		if inSingle {
			if ch == '\'' {
				inSingle = false
			} else {
				current.WriteByte(ch)
			}
			continue
		}

		if inDouble {
			if ch == '"' {
				inDouble = false
			} else if ch == '\\' {
				escaped = true
			} else {
				current.WriteByte(ch)
			}
			continue
		}

		switch ch {
		case '\\':
			escaped = true
		case '\'':
			inSingle = true
		case '"':
			inDouble = true
		case ' ', '\t', '\n':
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}

	if inSingle {
		return nil, fmt.Errorf("no terminating quote in -S string at position %d for quote '''", len(s))
	}
	if inDouble {
		return nil, fmt.Errorf("no terminating quote in -S string at position %d for quote '\"'", len(s))
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}

	var expandedWords []string
	for _, w := range words {
		expanded, err := expandEnvVarInS(w, envMap)
		if err != nil {
			return nil, err
		}
		expandedWords = append(expandedWords, expanded)
	}

	return expandedWords, nil
}

func expandEnvVarInS(w string, envMap map[string]string) (string, error) {
	var sb strings.Builder
	for i := 0; i < len(w); {
		if w[i] == '$' {
			if i+1 < len(w) && w[i+1] == '{' {
				end := strings.IndexByte(w[i+2:], '}')
				if end < 0 {
					return "", fmt.Errorf("only ${VARNAME} expansion is supported, error at: %s", w[i:])
				}
				endIdx := i + 2 + end
				varName := w[i+2 : endIdx]
				if !isValidVarName(varName) {
					return "", fmt.Errorf("only ${VARNAME} expansion is supported, error at: ${%s}", varName)
				}
				val := envMap[varName]
				sb.WriteString(val)
				i = endIdx + 1
			} else {
				return "", fmt.Errorf("only ${VARNAME} expansion is supported")
			}
		} else {
			sb.WriteByte(w[i])
			i++
		}
	}
	return sb.String(), nil
}

func isValidVarName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r == '_') {
				return false
			}
		} else {
			if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
				return false
			}
		}
	}
	return true
}
