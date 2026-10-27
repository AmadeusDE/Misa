package sshclient

import (
	"bufio"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// settings holds the ssh_config(5) keywords sshc understands. As in OpenSSH
// the first value obtained for each keyword wins, so the command line is
// applied before any config file.
type settings struct {
	HostName              string
	User                  string
	Port                  string
	StrictHostKeyChecking string
	UserKnownHostsFile    string
	IdentityFiles         []string
}

func (s *settings) set(key, value string) {
	first := func(field *string) {
		if *field == "" {
			*field = value
		}
	}
	switch strings.ToLower(key) {
	case "hostname":
		first(&s.HostName)
	case "user":
		first(&s.User)
	case "port":
		first(&s.Port)
	case "stricthostkeychecking":
		first(&s.StrictHostKeyChecking)
	case "userknownhostsfile":
		first(&s.UserKnownHostsFile)
	case "identityfile":
		s.IdentityFiles = append(s.IdentityFiles, value)
	}
}

const maxIncludeDepth = 8

// readConfig applies the file at path for host. Relative Include patterns
// resolve against dir, as OpenSSH resolves them against ~/.ssh.
func readConfig(path, host string, s *settings, dir string, depth int) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	parseConfig(f, host, s, func(pattern string) {
		if depth >= maxIncludeDepth {
			return
		}
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(dir, pattern)
		}
		matches, _ := filepath.Glob(pattern)
		for _, m := range matches {
			readConfig(m, host, s, dir, depth+1)
		}
	})
}

// parseConfig applies the Host blocks that match host. Match blocks are
// skipped, since sshc cannot evaluate their criteria.
func parseConfig(r io.Reader, host string, s *settings, include func(pattern string)) {
	active := true
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		key, value := splitConfigLine(sc.Text())
		switch strings.ToLower(key) {
		case "":
		case "host":
			active = hostMatches(host, strings.Fields(value))
		case "match":
			active = false
		case "include":
			if active && include != nil {
				for _, p := range strings.Fields(value) {
					include(p)
				}
			}
		default:
			if active {
				s.set(key, value)
			}
		}
	}
}

func splitConfigLine(line string) (key, value string) {
	line = strings.TrimSpace(line)
	if line == "" || line[0] == '#' {
		return "", ""
	}
	end := strings.IndexAny(line, " \t=")
	if end < 0 {
		return line, ""
	}
	key = line[:end]
	value = strings.TrimSpace(line[end:])
	value = strings.TrimSpace(strings.TrimPrefix(value, "="))
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}
	return key, value
}

// hostMatches applies a Host line's patterns: any positive match selects the
// block unless a negated (!) pattern also matches.
func hostMatches(host string, patterns []string) bool {
	host = strings.ToLower(host)
	matched := false
	for _, p := range patterns {
		negated := strings.HasPrefix(p, "!")
		p = strings.ToLower(strings.TrimPrefix(p, "!"))
		if ok, _ := path.Match(p, host); ok {
			if negated {
				return false
			}
			matched = true
		}
	}
	return matched
}

// expandTokens replaces ssh_config %-tokens; %% is a literal percent.
func expandTokens(s string, tokens map[byte]string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '%' || i+1 == len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		if v, ok := tokens[s[i]]; ok {
			b.WriteString(v)
		} else if s[i] == '%' {
			b.WriteByte('%')
		} else {
			b.WriteByte('%')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
