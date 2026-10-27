package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const treeSpec = "a(all)d(dirs-only)f(full-path)i(no-indent)s(size)h(human-readable)L(max-depth)#A(ascii)N(no-quotes)(noreport)"

var treeSpecParsed *Spec

func init() {
	var err error
	treeSpecParsed, err = ParseSpec(treeSpec)
	if err != nil {
		panic("builtins: bad tree spec: " + err.Error())
	}
	Register(Command{Name: "tree", Run: runTreeCmd})
}

func runTreeCmd(ctx *Context) int {
	flags, err := treeSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "tree:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, treeSpecParsed) {
		return 0
	}

	showAll := flags.Bool("a")
	dirsOnly := flags.Bool("d")
	fullPath := flags.Bool("f")
	noIndent := flags.Bool("i")
	showSize := flags.Bool("s")
	human := flags.Bool("h")
	ascii := flags.Bool("A")
	noReport := flags.Bool("noreport")

	maxDepth := int64(-1)
	if v, ok := flags.Num("L"); ok {
		maxDepth = v
	}

	targets := flags.Operands
	if len(targets) == 0 {
		targets = []string{"."}
	}

	dirsCount := 0
	filesCount := 0

	for _, target := range targets {
		abs := resolvePath(ctx, target)
		info, err := os.Lstat(abs)
		if err != nil {
			fmt.Fprintf(ctx.Stdout, "%s [error opening dir]\n", target)
			continue
		}

		if !info.IsDir() {
			printTreeLine(ctx, "", target, info, showSize, human)
			filesCount++
			continue
		}

		entries, rderr := os.ReadDir(abs)
		if rderr != nil {
			fmt.Fprintf(ctx.Stdout, "%s [error opening dir]\n", target)
			continue
		}

		dispHead := target
		if showSize {
			if human {
				fmt.Fprintf(ctx.Stdout, "[%s]  %s\n", humanSize(info.Size()), dispHead)
			} else {
				fmt.Fprintf(ctx.Stdout, "[%11d]  %s\n", info.Size(), dispHead)
			}
		} else {
			fmt.Fprintln(ctx.Stdout, dispHead)
		}

		d, f := walkTreeEntries(ctx, entries, abs, target, 1, maxDepth, "", showAll, dirsOnly, fullPath, noIndent, showSize, human, ascii)
		dirsCount += d
		filesCount += f
	}

	if !noReport {
		dirStr := "directories"
		if dirsCount == 1 {
			dirStr = "directory"
		}
		if dirsOnly {
			fmt.Fprintf(ctx.Stdout, "\n%d %s\n", dirsCount, dirStr)
		} else {
			fileStr := "files"
			if filesCount == 1 {
				fileStr = "file"
			}
			fmt.Fprintf(ctx.Stdout, "\n%d %s, %d %s\n", dirsCount, dirStr, filesCount, fileStr)
		}
	}

	return 0
}

func printTreeLine(ctx *Context, prefix, name string, info os.FileInfo, showSize, human bool) {
	if showSize {
		szStr := fmt.Sprintf("[%11d]", info.Size())
		if human {
			szStr = fmt.Sprintf("[%s]", humanSize(info.Size()))
		}
		fmt.Fprintf(ctx.Stdout, "%s%s  %s\n", prefix, szStr, name)
	} else {
		fmt.Fprintf(ctx.Stdout, "%s%s\n", prefix, name)
	}
}

func walkTree(ctx *Context, absDir, displayDir string, depth, maxDepth int64, indent string, showAll, dirsOnly, fullPath, noIndent, showSize, human, ascii bool) (int, int) {
	if maxDepth > 0 && depth > maxDepth {
		return 0, 0
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return 0, 0
	}

	return walkTreeEntries(ctx, entries, absDir, displayDir, depth, maxDepth, indent, showAll, dirsOnly, fullPath, noIndent, showSize, human, ascii)
}

func walkTreeEntries(ctx *Context, entries []os.DirEntry, absDir, displayDir string, depth, maxDepth int64, indent string, showAll, dirsOnly, fullPath, noIndent, showSize, human, ascii bool) (int, int) {
	if maxDepth > 0 && depth > maxDepth {
		return 0, 0
	}

	var filtered []os.DirEntry
	for _, e := range entries {
		if !showAll && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if dirsOnly && !e.IsDir() {
			continue
		}
		filtered = append(filtered, e)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Name() < filtered[j].Name()
	})

	dirsCount := 0
	filesCount := 0

	for i, e := range filtered {
		isLast := i == len(filtered)-1

		childAbs := filepath.Join(absDir, e.Name())
		childDisp := e.Name()
		if fullPath {
			childDisp = filepath.Join(displayDir, e.Name())
		}

		childInfo, err := os.Lstat(childAbs)
		if err != nil {
			continue
		}

		isSymlink := childInfo.Mode()&os.ModeSymlink != 0
		if isSymlink {
			if linkTarget, lerr := os.Readlink(childAbs); lerr == nil {
				childDisp = childDisp + " -> " + linkTarget
			}
		}

		prefix := "├── "
		nextIndent := indent + "│   "
		if ascii {
			prefix = "|-- "
			nextIndent = indent + "|   "
		}
		if isLast {
			prefix = "└── "
			if ascii {
				prefix = "`-- "
			}
			nextIndent = indent + "    "
		}

		linePrefix := indent + prefix
		if noIndent {
			linePrefix = ""
		}

		if showSize {
			szStr := fmt.Sprintf("[%11d]", childInfo.Size())
			if human {
				szStr = fmt.Sprintf("[%s]", humanSize(childInfo.Size()))
			}
			fmt.Fprintf(ctx.Stdout, "%s%s  %s\n", linePrefix, szStr, childDisp)
		} else {
			fmt.Fprintf(ctx.Stdout, "%s%s\n", linePrefix, childDisp)
		}

		if childInfo.IsDir() && !isSymlink {
			dirsCount++
			d, f := walkTree(ctx, childAbs, filepath.Join(displayDir, e.Name()), depth+1, maxDepth, nextIndent, showAll, dirsOnly, fullPath, noIndent, showSize, human, ascii)
			dirsCount += d
			filesCount += f
		} else {
			filesCount++
		}
	}

	return dirsCount, filesCount
}
