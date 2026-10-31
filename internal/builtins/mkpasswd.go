// Behaviour adapted from toybox toys/other/mkpasswd.c and the get_salt()
// helper in toybox lib/password.c at commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb (0BSD).
package builtins

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/AmadeusDE/Misa/internal/pwhash"
)

const mkpasswdSpec = "m(method):P#S(salt):"

// mkpasswdMethodList is the "Supported methods:" body, one method per line.
// It is used both by -m help and by --help.
const mkpasswdMethodList = `  md5     MD5-crypt ($1$), salt up to 8 characters
  sha256  SHA-256-crypt ($5$), salt up to 16 characters, may start with "rounds=N$"
  sha512  SHA-512-crypt ($6$, default), salt up to 16 characters, may start with "rounds=N$"
`

// mkpasswdUsage is the hand-written --help text. It follows toybox's wording
// closely, except that the default method is sha512 (toybox defaults to des,
// which Misa does not implement).
const mkpasswdUsage = `usage: mkpasswd [-m METHOD] [-S SALT] [-P FD] [PASSWORD] [SALT]

Print the crypt(3) hash of PASSWORD. Without a PASSWORD operand the password is
read from descriptor FD (-P) or, by default, from standard input (prompting on
the terminal without echoing). SALT is the second operand, -S, or a random salt
of the maximum length METHOD allows.

  -m METHOD  hash method, see below
  -S SALT    use SALT instead of a random one
  -P FD      read the password from file descriptor FD (0 is standard input)
  -h         this help text

Methods:
` + mkpasswdMethodList + `
The default method is sha512. toybox's mkpasswd defaults to des, but DES crypt
is not implemented here, so -m des fails instead of hashing with a scheme that
cannot hold a real password. Salts are never truncated: an over-long salt is an
error, not a silent cut to the maximum length.
`

var mkpasswdSpecParsed *Spec

// mkpasswdRand is the randomness source for random salts; tests swap it out.
var mkpasswdRand io.Reader = rand.Reader

func init() {
	var err error
	mkpasswdSpecParsed, err = ParseSpec(mkpasswdSpec)
	if err != nil {
		panic("builtins: bad mkpasswd spec: " + err.Error())
	}
	Register(Command{Name: "mkpasswd", Run: runMkpasswd})
}

func runMkpasswd(ctx *Context) int {
	flags, err := mkpasswdSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		return mkpasswdErr(ctx, err.Error())
	}
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, mkpasswdUsage)
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintln(ctx.Stdout, "mkpasswd (Misa) 0.1.0")
		return 0
	}

	ops := flags.Operands
	if len(ops) > 2 {
		return mkpasswdErr(ctx, "too many arguments")
	}

	method := pwhash.MethodSHA512
	if name, ok := flags.String("m"); ok {
		if strings.EqualFold(mkpasswdAlnum(name), "help") {
			fmt.Fprint(ctx.Stdout, "Supported methods:\n"+mkpasswdMethodList)
			return 0
		}
		m, err := mkpasswdMethod(name)
		if err != nil {
			return mkpasswdErr(ctx, err.Error())
		}
		method = m
	}

	salt, hasSalt := flags.String("S")
	if len(ops) == 2 {
		if hasSalt {
			return mkpasswdErr(ctx, "duplicate salt")
		}
		salt, hasSalt = ops[1], true
	}
	if hasSalt {
		if err := pwhash.ValidateSalt(method, salt); err != nil {
			return mkpasswdErr(ctx, "bad SALT: "+mkpasswdOneLine(err.Error()))
		}
	} else {
		salt, err = pwhash.RandomSalt(method, mkpasswdRand)
		if err != nil {
			return mkpasswdErr(ctx, "random salt failed: "+mkpasswdOneLine(err.Error()))
		}
	}

	password := ""
	if len(ops) > 0 {
		// Used verbatim: no newline stripping, and an empty operand is a
		// valid (empty) password.
		password = ops[0]
	} else {
		fd, haveFD := flags.Num("P")
		pw, ok := mkpasswdReadPassword(ctx, fd, haveFD)
		if !ok {
			return 1
		}
		password = pw
	}

	hash, err := pwhash.CryptMethod(method, password, salt)
	if err != nil {
		return mkpasswdErr(ctx, mkpasswdOneLine(err.Error()))
	}
	fmt.Fprintln(ctx.Stdout, hash)
	return 0
}

// mkpasswdMethod maps a -m value onto a canonical pwhash method name. Like
// toybox's get_salt it ignores everything that is not alphanumeric and folds
// case, so "sha-256", "sha_256" and "SHA256" all name sha256.
func mkpasswdMethod(name string) (string, error) {
	if m, ok := pwhash.NormalizeMethod(name); ok {
		return m, nil
	}
	if m, ok := pwhash.NormalizeMethod(mkpasswdAlnum(name)); ok {
		return m, nil
	}
	switch strings.ToLower(mkpasswdAlnum(name)) {
	case "des", "descrypt":
		return "", fmt.Errorf("unsupported hash method '%s' (DES crypt is not implemented; use md5, sha256 or sha512)", name)
	}
	return "", fmt.Errorf("unsupported hash method '%s' (supported: md5, sha256, sha512)", name)
}

// mkpasswdAlnum drops every byte that is not [0-9A-Za-z].
func mkpasswdAlnum(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// mkpasswdReadPassword reads the password from descriptor fd (only when
// haveFD, the -P flag was given) or from ctx.Stdin. ok is false after an error
// has been reported; the password itself is never printed.
func mkpasswdReadPassword(ctx *Context, fd int64, haveFD bool) (password string, ok bool) {
	src := ctx.Stdin
	if haveFD && fd != 0 {
		if fd < 0 {
			fmt.Fprintf(ctx.Stderr, "mkpasswd: password read failed: bad -P fd %d\n", fd)
			return "", false
		}
		f := os.NewFile(uintptr(fd), fmt.Sprintf("fd %d", fd))
		if f == nil {
			fmt.Fprintf(ctx.Stderr, "mkpasswd: password read failed: bad -P fd %d\n", fd)
			return "", false
		}
		// Close it so the finalizer os.NewFile installs cannot close the
		// descriptor at some random later point after the number is reused.
		defer f.Close()
		src = f
	}
	if src == nil {
		fmt.Fprintln(ctx.Stderr, "mkpasswd: password read failed: standard input is not available")
		return "", false
	}

	line, err := mkpasswdReadLine(ctx, src)
	if err != nil {
		if haveFD && fd != 0 {
			fmt.Fprintf(ctx.Stderr, "mkpasswd: password read failed: fd %d: %s\n", fd, fmtOsErr(err))
		} else {
			fmt.Fprintf(ctx.Stderr, "mkpasswd: password read failed: %s\n", fmtOsErr(err))
		}
		return "", false
	}
	return line, true
}

// mkpasswdReadLine reads one password: from a terminal without echo (prompting
// on stderr), otherwise one line with a trailing "\n" and "\r" removed. An
// empty input is an empty password, not an error.
func mkpasswdReadLine(ctx *Context, src io.Reader) (string, error) {
	if f, isFile := src.(*os.File); isFile && isTerminalReader(f) {
		fmt.Fprint(ctx.Stderr, "Password: ")
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(ctx.Stderr)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	line, err := bufio.NewReader(src).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}

// mkpasswdOneLine flattens an error message so it stays on a single line.
func mkpasswdOneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, s)
}

func mkpasswdErr(ctx *Context, msg string) int {
	fmt.Fprintf(ctx.Stderr, "mkpasswd: %s\n", mkpasswdOneLine(msg))
	return 1
}
