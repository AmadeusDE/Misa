package sshclient

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

// Options is an sshc command line.
type Options struct {
	Env         []string // nil inherits the process environment; non-nil is authoritative.
	Destination string
	Command     []string
	User        string
	Port        string
	Identities  []string
	Settings    []string // -o Key=Value
	ConfigFile  string   // -F; replaces the user and system config files
	ForceTTY    bool
	NoTTY       bool
}

// target is where and as whom to connect, with config files applied.
type target struct {
	Alias      string
	Addr       string
	User       string
	Identities []string
	KnownHosts []string
	Strict     string
}

func resolve(o Options, e env) (target, error) {
	destUser, host, destPort, err := splitDestination(o.Destination)
	if err != nil {
		return target{}, err
	}

	var s settings
	for _, kv := range o.Settings {
		key, value := splitConfigLine(kv)
		s.set(key, value)
	}
	s.set("user", o.User)
	s.set("port", o.Port)
	for _, id := range o.Identities {
		s.set("identityfile", id)
	}
	s.set("user", destUser)
	s.set("port", destPort)

	sshDir := filepath.Join(e.Home, ".ssh")
	if o.ConfigFile != "" {
		readConfig(o.ConfigFile, host, &s, sshDir, 0)
	} else {
		readConfig(filepath.Join(sshDir, "config"), host, &s, sshDir, 0)
		if e.SystemConfig != "" {
			readConfig(e.SystemConfig, host, &s, filepath.Dir(e.SystemConfig), 0)
		}
	}

	t := target{Alias: host, User: s.User, Strict: strings.ToLower(s.StrictHostKeyChecking)}
	if t.User == "" {
		t.User = e.LocalUser
	}
	if t.Strict == "" {
		t.Strict = "ask"
	}
	hostName := host
	if s.HostName != "" {
		hostName = expandTokens(s.HostName, map[byte]string{'h': host})
	}
	port := s.Port
	if port == "" {
		port = "22"
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return target{}, fmt.Errorf("bad port %q", port)
	}
	t.Addr = net.JoinHostPort(hostName, port)

	tokens := map[byte]string{'d': e.Home, 'u': e.LocalUser, 'r': t.User, 'h': hostName}
	expand := func(p string) string { return expandHome(expandTokens(p, tokens), e.Home) }

	ids := s.IdentityFiles
	if len(ids) == 0 {
		ids = []string{"~/.ssh/id_ed25519", "~/.ssh/id_ecdsa", "~/.ssh/id_rsa"}
	}
	for _, id := range ids {
		t.Identities = append(t.Identities, expand(id))
	}

	known := strings.Fields(s.UserKnownHostsFile)
	if len(known) == 0 {
		known = []string{"~/.ssh/known_hosts"}
	}
	for _, k := range known {
		t.KnownHosts = append(t.KnownHosts, expand(k))
	}
	return t, nil
}

// splitDestination parses [user@]host and ssh://[user@]host[:port].
func splitDestination(dest string) (user, host, port string, err error) {
	if strings.HasPrefix(dest, "ssh://") {
		u, perr := url.Parse(dest)
		if perr != nil {
			return "", "", "", perr
		}
		if u.User != nil {
			user = u.User.Username()
		}
		host, port = u.Hostname(), u.Port()
	} else if at := strings.LastIndex(dest, "@"); at >= 0 {
		user, host = dest[:at], dest[at+1:]
		if user == "" {
			return "", "", "", errors.New("empty user in destination")
		}
	} else {
		host = dest
	}
	if host == "" {
		return "", "", "", errors.New("no host in destination")
	}
	return user, host, port, nil
}

func expandHome(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}
