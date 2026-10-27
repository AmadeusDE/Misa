// Behaviour adapted from uutils coreutils hostid (MIT).
package builtins

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strings"
)

func init() { Register(Command{Name: "hostid", Run: runHostid}) }
func runHostid(ctx *Context) int {
	for _, a := range ctx.Args {
		if a == "--help" {
			fmt.Fprintln(ctx.Stdout, "Usage: hostid [OPTION]...\nPrint the numeric identifier of the current host.")
			return 0
		}
		if a == "--version" {
			fmt.Fprintln(ctx.Stdout, "hostid (Misa) 0.1.0")
			return 0
		}
		if strings.HasPrefix(a, "-") {
			fmt.Fprintf(ctx.Stderr, "hostid: unrecognized option '%s'\n", a)
			return 1
		}
	}
	if len(ctx.Args) > 0 {
		fmt.Fprintln(ctx.Stderr, "hostid: extra operand")
		return 1
	}
	if data, err := os.ReadFile(resolvePath(ctx, "/etc/hostid")); err == nil && len(data) >= 4 {
		fmt.Fprintf(ctx.Stdout, "%08x\n", binary.NativeEndian.Uint32(data[:4]))
		return 0
	}
	host, err := os.Hostname()
	if err == nil {
		ips, _ := net.LookupIP(host)
		for _, ip := range ips {
			if v := ip.To4(); v != nil {
				n := binary.BigEndian.Uint32(v)
				fmt.Fprintf(ctx.Stdout, "%08x\n", (n<<16)|(n>>16))
				return 0
			}
		}
	}
	fmt.Fprintln(ctx.Stdout, "00000000")
	return 0
}
