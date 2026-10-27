package mweb

import (
	"cmp"
	"errors"
	"flag"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/AmadeusDE/Misa/internal/config"
)

// ChildEnv marks the misa the browser terminal runs, so it never starts another server.
const ChildEnv = "MISA_WEB_CHILD"

// Options says where the web UI listens and what its terminal runs.
type Options struct {
	Addr      string
	Interface string
	Port      int
	Command   []string
}

// ParseFlags layers --web-addr, --web-interface and --web-port over the config's web block.
// An address given on the command line replaces a configured interface and vice versa.
func ParseFlags(args []string, cfg config.WebConfig) (Options, error) {
	o := Options{Addr: cfg.Addr, Interface: cfg.Interface, Port: cfg.Port, Command: cfg.Command}
	fs := flag.NewFlagSet("misa --web", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	addr := fs.String("web-addr", "", "listen address host:port (development)")
	iface := fs.String("web-interface", "", "listen on this interface's address (production, e.g. wg0)")
	port := fs.Int("web-port", 0, "port used with an interface (default 8787)")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() > 0 {
		return o, errors.New("unexpected argument " + fs.Arg(0))
	}
	if *addr != "" {
		o.Addr, o.Interface = *addr, ""
	}
	if *iface != "" {
		o.Interface, o.Addr = *iface, ""
	}
	if *port != 0 {
		o.Port = *port
	}
	if o.Addr == "" && o.Interface == "" {
		o.Addr = net.JoinHostPort("127.0.0.1", strconv.Itoa(cmp.Or(o.Port, defaultPort)))
	}
	return o, o.validate()
}

// defaultPort is used with an interface, or on loopback when nothing is set.
const defaultPort = 8787

func (o Options) validate() error {
	if o.Port < 0 || o.Port > 65535 {
		return errors.New("port out of range")
	}
	if o.Command != nil && (len(o.Command) == 0 || strings.TrimSpace(o.Command[0]) == "") {
		return errors.New("web.command must name a program")
	}
	return nil
}

// Argv is what the browser terminal runs: web.command, else misa itself.
func (o Options) Argv(exe string) []string {
	if len(o.Command) > 0 {
		return o.Command
	}
	return []string{exe}
}
