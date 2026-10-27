package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/AmadeusDE/Misa/internal/ughbot/config"
	"github.com/AmadeusDE/Misa/internal/ughbot/webhost"
	"github.com/AmadeusDE/Misa/internal/webadmin"
	"github.com/AmadeusDE/Misa/internal/webui"
)

// adminEnv opens the device store directly, so it works whether or not the daemon runs.
func adminEnv(name string, args []string, out io.Writer, in io.Reader) (webadmin.Env, []string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	cfgPath := fs.String("c", config.DefaultPath(), "config file")
	if err := fs.Parse(args); err != nil {
		return webadmin.Env{}, nil, err
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return webadmin.Env{}, nil, err
	}
	store, err := webhost.OpenStore(cfg)
	if err != nil {
		return webadmin.Env{}, nil, err
	}
	return webadmin.Env{
		Admin: webui.NewAdmin(store, nil), Out: out, In: in,
		Program: "ughbot devices", Issuer: "ughbot", KeyPath: webhost.KeyPath(cfg),
	}, fs.Args(), nil
}

func devicesCmd(args []string, out io.Writer, in io.Reader) error {
	cfgArgs, rest := splitSub(args)
	env, _, err := adminEnv("devices", cfgArgs, out, in)
	if err != nil {
		return err
	}
	return webadmin.Devices(env, rest)
}

func sessionsCmd(args []string, out io.Writer) error {
	cfgArgs, rest := splitSub(args)
	env, _, err := adminEnv("sessions", cfgArgs, out, nil)
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		return fmt.Errorf("usage: ughbot sessions list|kill <prefix>")
	}
	return webadmin.Sessions(env, rest)
}

// splitSub separates leading flags (-c cfg) from the subcommand and its arguments.
func splitSub(args []string) (flags, rest []string) {
	for i := 0; i < len(args); i++ {
		if args[i] == "-c" && i+1 < len(args) {
			flags = append(flags, args[i], args[i+1])
			i++
			continue
		}
		return flags, args[i:]
	}
	return flags, nil
}
