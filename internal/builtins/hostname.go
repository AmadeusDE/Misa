package builtins

import (
	"fmt"
	"net"
	"os"
	"strings"
)

const hostnameSpec = "a(alias)A(all-fqdns)b(boot)d(domain)f(fqdn)(long)i(ip-address)I(all-ip-addresses)s(short)"

var hostnameSpecParsed *Spec

func init() {
	var err error
	hostnameSpecParsed, err = ParseSpec(hostnameSpec)
	if err != nil {
		panic("builtins: bad hostname spec: " + err.Error())
	}
	Register(Command{Name: "hostname", Run: runHostname})
}

func runHostname(ctx *Context) int {
	flags, err := hostnameSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "hostname:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, hostnameSpecParsed) {
		return 0
	}

	h, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "hostname: %v\n", err)
		return 1
	}

	showShort := flags.Bool("s")
	showDomain := flags.Bool("d")
	showFQDN := flags.Bool("f") || flags.Bool("long")
	showIP := flags.Bool("i")
	showAllIP := flags.Bool("I")

	if showIP || showAllIP {
		ips, err := net.LookupIP(h)
		if err != nil || len(ips) == 0 {
			ips = []net.IP{net.ParseIP("127.0.0.1")}
		}
		if showAllIP {
			var strIPs []string
			for _, ip := range ips {
				strIPs = append(strIPs, ip.String())
			}
			fmt.Fprintln(ctx.Stdout, strings.Join(strIPs, " "))
			return 0
		}
		fmt.Fprintln(ctx.Stdout, ips[0].String())
		return 0
	}

	var short, domain string
	if idx := strings.IndexByte(h, '.'); idx != -1 {
		short = h[:idx]
		domain = h[idx+1:]
	} else {
		short = h
		domain = ""
	}

	if showDomain && showShort {
		// Combination -sd: if no domain, print empty
		if domain == "" {
			return 0
		}
		fmt.Fprintln(ctx.Stdout, domain)
		return 0
	}
	if showDomain {
		if domain == "" {
			return 0
		}
		fmt.Fprintln(ctx.Stdout, domain)
		return 0
	}
	if showShort {
		fmt.Fprintln(ctx.Stdout, short)
		return 0
	}
	if showFQDN {
		fmt.Fprintln(ctx.Stdout, h)
		return 0
	}

	fmt.Fprintln(ctx.Stdout, h)
	return 0
}
