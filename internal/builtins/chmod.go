package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AmadeusDE/Misa/internal/proceffect"
)

const chmodSpec = "R(recursive)v(verbose)c(changes)f(silent)quiet(quiet)"

var chmodSpecParsed *Spec

func isValidChmodFlag(arg string) bool {
	if !strings.HasPrefix(arg, "-") || arg == "-" {
		return false
	}
	if strings.HasPrefix(arg, "--") {
		switch arg {
		case "--recursive", "--verbose", "--changes", "--silent", "--quiet", "--help", "--version":
			return true
		default:
			return false
		}
	}
	// Short flags: -R, -v, -c, -f
	for i := 1; i < len(arg); i++ {
		switch arg[i] {
		case 'R', 'v', 'c', 'f':
		default:
			return false
		}
	}
	return true
}

func init() {
	var err error
	chmodSpecParsed, err = ParseSpec(chmodSpec)
	if err != nil {
		panic("builtins: bad chmod spec: " + err.Error())
	}
	Register(Command{Name: "chmod", Run: runChmod})
}

func fmtPermString(mode os.FileMode) string {
	buf := []byte("---------")
	const chars = "rwxrwxrwx"
	p := mode.Perm()
	for i := 0; i < 9; i++ {
		if p&(1<<(8-i)) != 0 {
			buf[i] = chars[i]
		}
	}
	return string(buf)
}

func runChmod(ctx *Context) int {
	args := ctx.Args

	// Try to parse with ParseArgs, but handle the case where a mode spec
	// looks like an option (e.g., -w for symbolic mode)
	flags, err := chmodSpecParsed.ParseArgs(args)
	if err != nil {
		// Check if this might be a mode spec that looks like an option
		// If we have an "unknown option" error and it's a short option like -w,
		// treat the arguments as operands
		errStr := err.Error()
		if strings.Contains(errStr, "unknown option -") && len(args) > 0 && len(args[0]) == 2 && args[0][0] == '-' {
			// First arg looks like a mode spec, parse manually
			var flagArgs []string
			var operands []string
			inOperands := false
			for i, arg := range args {
				if inOperands {
					operands = append(operands, arg)
					continue
				}
				if arg == "--" {
					inOperands = true
					operands = append(operands, args[i+1:]...)
					break
				}
				// Check if it's a recognized flag
				if isValidChmodFlag(arg) {
					flagArgs = append(flagArgs, arg)
				} else {
					// Treat as start of operands
					inOperands = true
					operands = append(operands, arg)
				}
			}
			var parseErr error
			flags, parseErr = chmodSpecParsed.ParseArgs(flagArgs)
			if parseErr != nil {
				fmt.Fprintln(ctx.Stderr, "chmod:", parseErr)
				return 1
			}
			flags.Operands = operands
		} else {
			fmt.Fprintln(ctx.Stderr, "chmod:", err)
			return 1
		}
	}
	if HandleHelpVersion(ctx, flags, chmodSpecParsed) {
		return 0
	}

	operands := flags.Operands
	if len(operands) < 2 {
		fmt.Fprintln(ctx.Stderr, "chmod: missing operand")
		return 1
	}

	modeSpec := operands[0]
	targets := operands[1:]
	recursive := flags.Bool("R")
	verbose := flags.Bool("v")
	changes := flags.Bool("c")
	silent := flags.Bool("f") || flags.Bool("quiet")
	exitCode := 0

	isOctal := true
	for _, c := range modeSpec {
		if c < '0' || c > '7' {
			isOctal = false
			break
		}
	}

	applyMode := func(path, displayName string, isExplicitArg bool) error {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if !isExplicitArg {
				return nil
			}
			// Explicit symlink operand: try os.Stat (follows symlink)
			info, err = os.Stat(path)
			if os.IsNotExist(err) {
				// Dangling symlink
				return fmt.Errorf("cannot operate on dangling symlink")
			}
			if err != nil {
				return err
			}
		}

		oldPerm := info.Mode().Perm()
		oldStr := fmtPermString(info.Mode())

		var newMode os.FileMode
		if isOctal {
			n, err := strconv.ParseUint(modeSpec, 8, 32)
			if err != nil {
				return fmt.Errorf("invalid mode %q", modeSpec)
			}
			newMode = os.FileMode(n)
		} else {
			newMode, err = parseSymbolicMode(modeSpec, info.Mode())
			if err != nil {
				return err
			}
		}

		if err := os.Chmod(path, newMode); err != nil {
			return err
		}

		newPerm := newMode.Perm()
		newStr := fmtPermString(newMode)

		// Verify the actual permission after chmod
		// (in case umask prevented the operation from succeeding)
		actualInfo, err := os.Lstat(path)
		if err == nil {
			actualPerm := actualInfo.Mode().Perm()
			// For symbolic -x mode, verify that execute was removed
			if !isOctal && strings.Contains(modeSpec, "-x") && !strings.Contains(modeSpec, "+x") {
				// Check if execute was actually removed
				if (actualPerm & 0111) != 0 {
					return fmt.Errorf("new permissions are %s, not %s", fmtPermString(actualInfo.Mode()), newStr)
				}
			}
		}

		if oldPerm != newPerm {
			if verbose || changes {
				fmt.Fprintf(ctx.Stdout, "mode of '%s' changed from %04o (%s) to %04o (%s)\n", displayName, oldPerm, oldStr, newPerm, newStr)
			}
		} else {
			if verbose {
				fmt.Fprintf(ctx.Stdout, "mode of '%s' retained as %04o (%s)\n", displayName, oldPerm, oldStr)
			}
		}
		return nil
	}

	for _, name := range targets {
		path := resolvePath(ctx, name)
		printErr := func(err error, disp string) {
			if silent {
				return
			}
			if os.IsNotExist(err) {
				fmt.Fprintf(ctx.Stderr, "chmod: cannot access '%s': No such file or directory\n", disp)
			} else if strings.Contains(err.Error(), "cannot operate on dangling symlink") {
				fmt.Fprintf(ctx.Stderr, "chmod: cannot operate on dangling symlink '%s'\n", disp)
			} else if os.IsPermission(err) {
				fmt.Fprintf(ctx.Stderr, "chmod: '%s': Permission denied\n", disp)
			} else {
				fmt.Fprintf(ctx.Stderr, "chmod: %v\n", err)
			}
		}

		if recursive {
			var seenPaths map[string]bool
			seenPaths = make(map[string]bool)
			err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
				rel, _ := filepath.Rel(path, p)
				disp := name
				isExplicit := false
				if rel != "." && rel != "" {
					disp = filepath.Join(name, rel)
				} else {
					isExplicit = true
				}
				if err != nil {
					// Only print if we haven't already reported an error for this path
					if !seenPaths[p] {
						printErr(err, disp)
						seenPaths[p] = true
						exitCode = 1
					}
					return nil
				}
				seenPaths[p] = true
				if err := applyMode(p, disp, isExplicit); err != nil {
					printErr(err, disp)
					exitCode = 1
				}
				return nil
			})
			if err != nil {
				printErr(err, name)
				exitCode = 1
			}
		} else {
			if err := applyMode(path, name, true); err != nil {
				printErr(err, name)
				exitCode = 1
			}
		}
	}
	return exitCode
}

func parseSymbolicMode(spec string, current os.FileMode) (os.FileMode, error) {
	mode := current.Perm()
	if current&os.ModeSetuid != 0 {
		mode |= 04000
	}
	if current&os.ModeSetgid != 0 {
		mode |= 02000
	}
	if current&os.ModeSticky != 0 {
		mode |= 01000
	}

	for _, clause := range strings.Split(spec, ",") {
		if len(clause) == 0 {
			continue
		}

		i := 0
		var who os.FileMode
		hasWho := false
		for i < len(clause) {
			switch clause[i] {
			case 'u':
				who |= 0700 | 04000
				hasWho = true
			case 'g':
				who |= 0070 | 02000
				hasWho = true
			case 'o':
				who |= 0007
				hasWho = true
			case 'a':
				who |= 07777
				hasWho = true
			default:
				goto doneWho
			}
			i++
		}
	doneWho:
		if !hasWho {
			mask, err := proceffect.Umask()
			if err != nil {
				return 0, fmt.Errorf("cannot read umask: %w", err)
			}
			who = os.FileMode(07777 &^ mask)
		}

		for i < len(clause) {
			if clause[i] != '+' && clause[i] != '-' && clause[i] != '=' {
				return 0, fmt.Errorf("invalid mode %q", spec)
			}
			op := clause[i]
			i++

			var bits os.FileMode
			var hasRefClass bool
			var refClass os.FileMode
			for i < len(clause) && clause[i] != '+' && clause[i] != '-' && clause[i] != '=' && clause[i] != ',' {
				switch clause[i] {
				case 'r':
					bits |= 0444
				case 'w':
					bits |= 0222
				case 'x':
					bits |= 0111
				case 'X':
					if current.IsDir() || current.Perm()&0111 != 0 {
						bits |= 0111
					}
				case 's':
					bits |= 06000
				case 't':
					bits |= 01000
				case 'u':
					// Copy user bits (extract as 000700)
					hasRefClass = true
					refClass |= (mode >> 6) & 07
				case 'g':
					// Copy group bits (extract as 000070)
					hasRefClass = true
					refClass |= (mode >> 3) & 07
				case 'o':
					// Copy other bits (extract as 000007)
					hasRefClass = true
					refClass |= mode & 07
				default:
					return 0, fmt.Errorf("invalid mode %q", spec)
				}
				i++
			}

			// If we have reference classes, expand them to all target bits
			if hasRefClass {
				bits = 0
				if (refClass & 04) != 0 {
					bits |= 0444
				}
				if (refClass & 02) != 0 {
					bits |= 0222
				}
				if (refClass & 01) != 0 {
					bits |= 0111
				}
			}

			bits &= who

			switch op {
			case '+':
				mode |= bits
			case '-':
				mode &^= bits
			case '=':
				// For '=', if who was not explicitly specified (default umask-based),
				// clear all permission bits and set only the specified ones
				// Otherwise, keep the structure but only change the specified who bits
				if !hasWho {
					// Umask-based: clear everything and set only the specified bits
					mode = bits
				} else {
					// Explicit who: clear only those bits, set new bits
					mode = (mode &^ who) | bits
				}
			}
		}
	}

	var result os.FileMode
	result = os.FileMode(mode & 0777)
	if mode&04000 != 0 {
		result |= os.ModeSetuid
	}
	if mode&02000 != 0 {
		result |= os.ModeSetgid
	}
	if mode&01000 != 0 {
		result |= os.ModeSticky
	}
	return result, nil
}
