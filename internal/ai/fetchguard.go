package ai

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// fetchClient serves fetch_url. The model picks its URLs, so it dials public
// addresses only: a page it reads must not steer it into local services.
var fetchClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 10 * time.Second, Control: publicOnly}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
	},
}

// publicOnly refuses connections to loopback, private, link-local and other
// non-public addresses. It runs on the resolved address, so DNS names and
// redirects that point inward are refused too.
func publicOnly(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return err
	}
	ip := ap.Addr().Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || cgnat.Contains(ip) {
		return fmt.Errorf("fetch_url refuses non-public address %s", ip)
	}
	return nil
}

var cgnat = netip.MustParsePrefix("100.64.0.0/10")
