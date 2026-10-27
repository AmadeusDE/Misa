package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const pwdSpec = "L(logical)P(physical)"

var pwdSpecParsed *Spec

func init() {
	var err error
	pwdSpecParsed, err = ParseSpec(pwdSpec)
	if err != nil {
		panic("builtins: bad pwd spec: " + err.Error())
	}
	Register(Command{Name: "pwd", Run: runPwd})
}

func runPwd(ctx *Context) int {
	flags, err := pwdSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "pwd:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, pwdSpecParsed) {
		return 0
	}

	posixlyCorrect := false
	if _, ok := lookupEnv(ctx, "POSIXLY_CORRECT"); ok {
		posixlyCorrect = true
	}

	useLogical := posixlyCorrect

	for i := len(flags.Order()) - 1; i >= 0; i-- {
		k := flags.Order()[i]
		if k == "P" || k == "physical" {
			useLogical = false
			break
		}
		if k == "L" || k == "logical" {
			useLogical = true
			break
		}
	}

	physPath, physErr := filepath.EvalSymlinks(ctx.Dir)
	if physErr != nil {
		physPath = ctx.Dir
	}

	if useLogical {
		logicalPath := ctx.Dir
		if envPwd, ok := lookupEnv(ctx, "PWD"); ok && envPwd != "" {
			logicalPath = envPwd
		}
		if isValidLogicalPath(logicalPath, physPath) {
			if len(flags.Operands) > 0 {
				fmt.Fprintln(ctx.Stderr, "pwd: ignoring non-option arguments")
			}
			fmt.Fprintln(ctx.Stdout, logicalPath)
			return 0
		}
	}

	if len(flags.Operands) > 0 {
		fmt.Fprintln(ctx.Stderr, "pwd: ignoring non-option arguments")
	}
	fmt.Fprintln(ctx.Stdout, physPath)
	return 0
}

func isValidLogicalPath(logPath, physPath string) bool {
	if !filepath.IsAbs(logPath) {
		return false
	}
	if strings.Contains(logPath, "/./") || strings.Contains(logPath, "/../") ||
		strings.HasSuffix(logPath, "/.") || strings.HasSuffix(logPath, "/..") {
		return false
	}
	logSt, err1 := os.Stat(logPath)
	if err1 != nil {
		return false
	}
	physSt, err2 := os.Stat(physPath)
	if err2 != nil {
		return false
	}
	return os.SameFile(logSt, physSt)
}
