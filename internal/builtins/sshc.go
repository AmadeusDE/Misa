package builtins

import (
	"fmt"
	"os"
	"strings"

	"github.com/AmadeusDE/Misa/internal/sshclient"
)

const sshcUsage = "usage: sshc [-tT] [-F configfile] [-i identity_file] [-l login_name]\n" +
	"            [-o option] [-p port] destination [command ...]\n"

var sshcSpec = mustSpec("^F:i:l:o:p:tT")

func init() {
	Register(Command{Name: "sshc", Run: runSSHC})
}

func mustSpec(spec string) *Spec {
	s, err := ParseSpec(spec)
	if err != nil {
		panic("builtins: bad spec " + spec + ": " + err.Error())
	}
	return s
}

// sshcOptions maps an OpenSSH-style command line onto sshclient.Options.
func sshcOptions(args []string) (sshclient.Options, error) {
	flags, err := sshcSpec.ParseArgs(args)
	if err != nil {
		return sshclient.Options{}, err
	}
	if len(flags.Operands) == 0 {
		return sshclient.Options{}, fmt.Errorf("no destination")
	}
	o := sshclient.Options{
		Destination: flags.Operands[0],
		Command:     flags.Operands[1:],
		Identities:  flags.AllStrings("i"),
		Settings:    flags.AllStrings("o"),
		ForceTTY:    flags.Bool("t"),
		NoTTY:       flags.Bool("T"),
	}
	o.User, _ = flags.ExplicitString("l")
	o.Port, _ = flags.ExplicitString("p")
	o.ConfigFile, _ = flags.ExplicitString("F")
	return o, nil
}

func runSSHC(ctx *Context) int {
	o, err := sshcOptions(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "sshc: %v\n%s", err, sshcUsage)
		return 255
	}
	return sshclient.Run(o, envValue(ctx.Env, "TERM"), ctx.Stdin, ctx.Stdout, ctx.Stderr)
}

func envValue(environ []string, key string) string {
	for _, kv := range environ {
		if v, ok := strings.CutPrefix(kv, key+"="); ok {
			return v
		}
	}
	return os.Getenv(key)
}
