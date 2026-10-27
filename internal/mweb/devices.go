package mweb

import (
	"fmt"

	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/AmadeusDE/Misa/internal/webadmin"
	"github.com/AmadeusDE/Misa/internal/webui"
)

func init() {
	builtins.Register(builtins.Command{Name: "devices", Run: runDevices})
}

// runDevices opens the device screen on misa --web's store; with arguments it
// runs the same commands as `misa --web-admin devices`.
func runDevices(ctx *builtins.Context) int {
	store, err := openStore()
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "devices:", err)
		return 1
	}
	if len(ctx.Args) > 0 {
		env := webadmin.Env{
			Admin: webui.NewAdmin(store, nil), Out: ctx.Stdout, In: ctx.Stdin,
			Program: "devices", Issuer: title, KeyPath: deviceKeyPath(),
		}
		if err := webadmin.Devices(env, ctx.Args); err != nil {
			fmt.Fprintln(ctx.Stderr, "devices:", err)
			return 1
		}
		return 0
	}
	if err := webadmin.RunTUI(webui.NewAdmin(store, nil)); err != nil {
		fmt.Fprintln(ctx.Stderr, "devices:", err)
		return 1
	}
	return 0
}
