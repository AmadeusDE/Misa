package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AmadeusDE/Misa/internal/awk"
)

const awkSpec = "^F:v:f:b(characters)"

var awkSpecParsed *Spec

func init() {
	var err error
	awkSpecParsed, err = ParseSpec(awkSpec)
	if err != nil {
		panic("builtins: bad awk spec: " + err.Error())
	}
	Register(Command{Name: "awk", Run: runAwk})
}

func awkProgram(ctx *Context, files []string, operands []string) (string, []string, error) {
	if len(files) == 0 {
		if len(operands) == 0 {
			return "", nil, fmt.Errorf("usage: awk [-F fs] [-v var=value] [-b] 'program' [file ...]\n       awk [-F fs] [-v var=value] [-b] -f progfile [file ...]")
		}
		return operands[0], operands[1:], nil
	}
	var src []string
	for _, name := range files {
		path := name
		if !filepath.IsAbs(path) {
			path = filepath.Join(ctx.Dir, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", nil, fmt.Errorf("can't open file %s: %v", name, errUnwrap(err))
		}
		src = append(src, string(data))
	}
	return strings.Join(src, "\n"), operands, nil
}

func errUnwrap(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}

func runAwk(ctx *Context) int {
	flags, err := awkSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "awk:", err)
		return 2
	}
	if HandleHelpVersion(ctx, flags, awkSpecParsed) {
		return 0
	}
	program, operands, err := awkProgram(ctx, flags.AllStrings("f"), flags.Operands)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "awk:", err)
		return 2
	}
	cfg := awk.Config{
		Program:  program,
		Operands: operands,
		Assign:   flags.AllStrings("v"),
		Bytes:    flags.Bool("b"),
		Dir:      ctx.Dir,
		Stdin:    ctx.Stdin,
		Stdout:   ctx.Stdout,
		Stderr:   ctx.Stderr,
	}
	if fs, ok := flags.ExplicitString("F"); ok {
		cfg.FS = &fs
	}
	if cfg.Stdin == nil {
		cfg.Stdin = strings.NewReader("")
	}
	return awk.Run(cfg)
}
