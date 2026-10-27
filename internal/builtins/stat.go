package builtins

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const statSpec = "c(format):p:(printf):f(file-system)t(terse)L(dereference)"

var statSpecParsed *Spec

func init() {
	var err error
	statSpecParsed, err = ParseSpec(statSpec)
	if err != nil {
		panic("builtins: bad stat spec: " + err.Error())
	}
	Register(Command{Name: "stat", Run: runStat})
}

func runStat(ctx *Context) int {
	flags, err := statSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "stat:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, statSpecParsed) {
		return 0
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "stat: error: the following required arguments were not provided:\n  <FILE>...")
		return 1
	}

	format, hasFormat := flags.String("c")
	if !hasFormat {
		format, hasFormat = flags.String("p")
	}
	printfFmt, hasPrintf := flags.String("printf")
	terse := flags.Bool("t")
	isFS := flags.Bool("f")
	deref := flags.Bool("L")
	exitCode := 0

	for _, name := range flags.Operands {
		path := resolvePath(ctx, name)

		if isFS {
			var stfs syscall.Statfs_t
			if err := syscall.Statfs(path, &stfs); err != nil {
				fmt.Fprintf(ctx.Stderr, "stat: cannot read file system information for '%s': %v\n", name, err)
				exitCode = 1
				continue
			}
			printFSInfo(ctx.Stdout, name, &stfs)
			continue
		}

		var info os.FileInfo
		var err error
		if deref {
			info, err = os.Stat(path)
		} else {
			info, err = os.Lstat(path)
		}
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "stat: %v\n", err)
			exitCode = 1
			continue
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			fmt.Fprintf(ctx.Stderr, "stat: cannot get system info for %s\n", name)
			exitCode = 1
			continue
		}

		if hasPrintf {
			out, ferr := statFormat(printfFmt, name, info, st)
			if ferr != nil {
				fmt.Fprintf(ctx.Stderr, "stat: %s\n", ferr)
				exitCode = 1
				continue
			}
			fmt.Fprint(ctx.Stdout, out)
		} else if hasFormat {
			out, ferr := statFormat(format, name, info, st)
			if ferr != nil {
				fmt.Fprintf(ctx.Stderr, "stat: %s\n", ferr)
				exitCode = 1
				continue
			}
			fmt.Fprintln(ctx.Stdout, out)
		} else if terse {
			fmt.Fprintf(ctx.Stdout, "%s %d %d %x %d %d %d %d %d %d %d %d %d %d\n",
				name, info.Size(), st.Blocks,
				uint32(st.Mode), st.Nlink, st.Uid, st.Gid,
				st.Dev, st.Ino, st.Blksize,
				atim(st).Sec, mtim(st).Sec, ctim(st).Sec, 0)
		} else {
			statPrintDefault(ctx, name, info, st)
		}
	}
	return exitCode
}

func statFormat(format, name string, info os.FileInfo, st *syscall.Stat_t) (string, error) {
	var out strings.Builder
	i := 0
	for i < len(format) {
		if format[i] == '\\' && i+1 < len(format) {
			i++
			switch format[i] {
			case 'n':
				out.WriteByte('\n')
			case 't':
				out.WriteByte('\t')
			case 'r':
				out.WriteByte('\r')
			case '\\':
				out.WriteByte('\\')
			default:
				out.WriteByte('\\')
				out.WriteByte(format[i])
			}
			i++
			continue
		}
		if format[i] != '%' {
			out.WriteByte(format[i])
			i++
			continue
		}
		// Start of a % directive. Record starting position for error reporting.
		dirStart := i
		i++
		if i >= len(format) {
			out.WriteByte('%')
			break
		}
		// Skip optional printf-style width and precision (e.g. %13.6Y or %.9Y).
		hasWidthOrPrec := false
		prec := -1
		hasPrec := false
		if format[i] == '-' {
			i++
			hasWidthOrPrec = true
		}
		for i < len(format) && format[i] >= '0' && format[i] <= '9' {
			i++
			hasWidthOrPrec = true
		}
		if i < len(format) && format[i] == '.' {
			i++
			hasWidthOrPrec = true
			hasPrec = true
			pStart := i
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				i++
			}
			if pStart < i {
				prec, _ = strconv.Atoi(format[pStart:i])
			} else {
				prec = 0
			}
		}
		if i >= len(format) {
			// Ran out of input after a width/precision spec — invalid.
			directive := format[dirStart:]
			return "", fmt.Errorf("'%s': invalid directive", directive)
		}
		switch format[i] {
		case 'n':
			out.WriteString(name)
		case 's':
			fmt.Fprintf(&out, "%d", info.Size())
		case 'b':
			fmt.Fprintf(&out, "%d", st.Blocks)
		case 'B':
			fmt.Fprintf(&out, "%d", 512)
		case 'd':
			fmt.Fprintf(&out, "%d", st.Dev)
		case 'D':
			fmt.Fprintf(&out, "%x", st.Dev)
		case 'f':
			fmt.Fprintf(&out, "%x", uint32(st.Mode))
		case 'a':
			fmt.Fprintf(&out, "%o", info.Mode().Perm())
		case 'A':
			out.WriteString(info.Mode().String())
		case 'F':
			out.WriteString(statFileType(info))
		case 'm':
			// mount point — approximate with "/"
			out.WriteString("/")
		case 'o':
			fmt.Fprintf(&out, "%d", st.Blksize)
		case 'u':
			fmt.Fprintf(&out, "%d", st.Uid)
		case 'g':
			fmt.Fprintf(&out, "%d", st.Gid)
		case 'U':
			if u, err := user.LookupId(strconv.Itoa(int(st.Uid))); err == nil {
				out.WriteString(u.Username)
			} else {
				fmt.Fprintf(&out, "%d", st.Uid)
			}
		case 'G':
			if g, err := user.LookupGroupId(strconv.Itoa(int(st.Gid))); err == nil {
				out.WriteString(g.Name)
			} else {
				fmt.Fprintf(&out, "%d", st.Gid)
			}
		case 'i':
			fmt.Fprintf(&out, "%d", st.Ino)
		case 'h':
			fmt.Fprintf(&out, "%d", st.Nlink)
		case 'x':
			out.WriteString(formatStatTime(atim(st).Sec, atim(st).Nsec))
		case 'X':
			sec, nsec := atim(st).Sec, atim(st).Nsec
			if prec > 0 {
				fmt.Fprintf(&out, "%d.%0*d", sec, prec, nsec/1000000000)
			} else if prec == 0 && hasPrec {
				fmt.Fprintf(&out, "%d", sec)
			} else {
				fmt.Fprintf(&out, "%d", sec)
			}
		case 'y':
			out.WriteString(formatStatTime(mtim(st).Sec, mtim(st).Nsec))
		case 'Y':
			sec, nsec := mtim(st).Sec, mtim(st).Nsec
			if prec > 0 {
				nsStr := fmt.Sprintf("%09d", nsec)
				if prec < len(nsStr) {
					nsStr = nsStr[:prec]
				}
				fmt.Fprintf(&out, "%d.%s", sec, nsStr)
			} else {
				fmt.Fprintf(&out, "%d", sec)
			}
		case 'z':
			out.WriteString(formatStatTime(ctim(st).Sec, ctim(st).Nsec))
		case 'Z':
			sec, nsec := ctim(st).Sec, ctim(st).Nsec
			if prec > 0 {
				nsStr := fmt.Sprintf("%09d", nsec)
				if prec < len(nsStr) {
					nsStr = nsStr[:prec]
				}
				fmt.Fprintf(&out, "%d.%s", sec, nsStr)
			} else {
				fmt.Fprintf(&out, "%d", sec)
			}
		case '%':
			out.WriteByte('%')
		default:
			if hasWidthOrPrec {
				// Width/precision with an unrecognized format char is invalid.
				directive := format[dirStart : i+1]
				return "", fmt.Errorf("'%s': invalid directive", directive)
			}
			// Bare unknown letter: output '?' (backward compat).
			out.WriteByte('?')
		}
		i++
	}
	return out.String(), nil
}

func statFileType(info os.FileInfo) string {
	m := info.Mode()
	switch {
	case m.IsRegular():
		if info.Size() == 0 {
			return "regular empty file"
		}
		return "regular file"
	case m.IsDir():
		return "directory"
	case m&os.ModeSymlink != 0:
		return "symbolic link"
	case m&os.ModeNamedPipe != 0:
		return "fifo"
	case m&os.ModeSocket != 0:
		return "socket"
	case m&os.ModeDevice != 0:
		if m&os.ModeCharDevice != 0 {
			return "character special file"
		}
		return "block special file"
	}
	return "unknown"
}

func statPrintDefault(ctx *Context, name string, info os.FileInfo, st *syscall.Stat_t) {
	fmt.Fprintf(ctx.Stdout, "  File: %s\n", name)
	fmt.Fprintf(ctx.Stdout, "  Size: %-15d Blocks: %-10d IO Block: %-6d %s\n",
		info.Size(), st.Blocks, st.Blksize, statFileType(info))
	fmt.Fprintf(ctx.Stdout, "Device: %xh/%-10d Inode: %-11d Links: %d\n",
		st.Dev, st.Dev, st.Ino, st.Nlink)
	fmt.Fprintf(ctx.Stdout, "Access: (%04o/%s)  Uid: (%5d/ %s)   Gid: (%5d/ %s)\n",
		info.Mode().Perm(), info.Mode().String(),
		st.Uid, statLookupUser(st.Uid), st.Gid, statLookupGroup(st.Gid))
	fmt.Fprintf(ctx.Stdout, "Access: %s\n", time.Unix(atim(st).Sec, atim(st).Nsec).Format("2006-01-02 15:04:05.000000000 -0700"))
	fmt.Fprintf(ctx.Stdout, "Modify: %s\n", time.Unix(mtim(st).Sec, mtim(st).Nsec).Format("2006-01-02 15:04:05.000000000 -0700"))
	fmt.Fprintf(ctx.Stdout, "Change: %s\n", time.Unix(ctim(st).Sec, ctim(st).Nsec).Format("2006-01-02 15:04:05.000000000 -0700"))
}

func statLookupUser(uid uint32) string {
	if u, err := user.LookupId(strconv.Itoa(int(uid))); err == nil {
		return u.Username
	}
	return strconv.Itoa(int(uid))
}

func statLookupGroup(gid uint32) string {
	if g, err := user.LookupGroupId(strconv.Itoa(int(gid))); err == nil {
		return g.Name
	}
	return strconv.Itoa(int(gid))
}

func formatStatTime(sec, nsec int64) string {
	t := time.Unix(sec, nsec)
	if tz := os.Getenv("TZ"); tz != "" {
		if strings.EqualFold(tz, "utc") {
			t = t.UTC()
		} else if loc, err := time.LoadLocation(tz); err == nil {
			t = t.In(loc)
		}
	}
	return t.Format("2006-01-02 15:04:05.000000000 -0700")
}
