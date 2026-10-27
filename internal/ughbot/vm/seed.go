package vm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/config"
)

// guestUser is the account the daemon logs in as. It gets this host user's uid so files it
// writes in a shared directory belong to the host user.
const guestUser = "ughbot"

// prepareScript installs the packages and runs the setup commands, then reports to the seed
// server; cloud-init is switched off afterwards, so later boots of the base are plain.
func prepareScript(v config.VM, report string) string {
	var b strings.Builder
	b.WriteString("#!/bin/bash\nset -uxo pipefail\n")
	fmt.Fprintf(&b, "report() { curl -fsS -m 10 %s/\"$1\" || true; }\n", report)
	b.WriteString("trap 'report fail' ERR\nset -e\n")
	b.WriteString("pacman-key --init\npacman-key --populate archlinux\n")
	if len(v.Packages) > 0 {
		b.WriteString("pacman -Syu --noconfirm --needed " + strings.Join(v.Packages, " ") + "\n")
	}
	for _, s := range v.Setup {
		b.WriteString(s + "\n")
	}
	b.WriteString("touch /etc/cloud/cloud-init.disabled\nsync\nreport done\n")
	return b.String()
}

func userData(v config.VM, k *keys, uid int, report string) ([]byte, error) {
	cfg := map[string]any{
		"hostname": "ughbot-vm",
		"users": []any{map[string]any{
			"name": guestUser, "uid": uid, "shell": "/bin/bash", "lock_passwd": true,
			"sudo": "ALL=(ALL) NOPASSWD:ALL", "ssh_authorized_keys": []string{authorizedLine(k.login)},
		}},
		"ssh_pwauth":      false,
		"ssh_deletekeys":  true,
		"ssh_genkeytypes": []string{},
		"ssh_keys":        map[string]string{"ed25519_private": k.hostPEM, "ed25519_public": authorizedLine(k.host)},
		"growpart":        map[string]any{"mode": "auto", "devices": []string{"/"}},
		"resize_rootfs":   true,
		"write_files": []any{map[string]any{
			"path": "/root/ughbot-prepare.sh", "permissions": "0700", "content": prepareScript(v, report),
		}},
		"runcmd":      []any{[]string{"/bin/bash", "/root/ughbot-prepare.sh"}},
		"power_state": map[string]any{"mode": "poweroff", "condition": true, "timeout": 30},
	}
	body, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte("#cloud-config\n"), body...), nil
}

// seed serves cloud-init's NoCloud data to a booting VM, which reaches the host's loopback
// as 10.0.2.2, under a random path, and hears back whether preparing worked.
type seed struct {
	srv    *http.Server
	ln     net.Listener
	token  string
	result chan string
}

// startSeed serves the user-data that build returns for the URL the VM reports back to.
func startSeed(build func(report string) ([]byte, error)) (*seed, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	tok := make([]byte, 16)
	_, _ = rand.Read(tok)
	s := &seed{ln: ln, token: hex.EncodeToString(tok), result: make(chan string, 2)}
	user, err := build(s.guestURL())
	if err != nil {
		ln.Close()
		return nil, err
	}
	meta := fmt.Sprintf("instance-id: ughbot-%s\nlocal-hostname: ughbot-vm\n", s.token[:8])
	mux := http.NewServeMux()
	prefix := "/" + s.token + "/"
	mux.HandleFunc(prefix+"meta-data", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(meta)) })
	mux.HandleFunc(prefix+"user-data", func(w http.ResponseWriter, _ *http.Request) { w.Write(user) })
	mux.HandleFunc(prefix+"vendor-data", func(w http.ResponseWriter, _ *http.Request) {})
	for _, outcome := range []string{"done", "fail"} {
		mux.HandleFunc(prefix+outcome, func(w http.ResponseWriter, _ *http.Request) {
			select {
			case s.result <- outcome:
			default:
			}
		})
	}
	s.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go s.srv.Serve(ln)
	return s, nil
}

func (s *seed) port() int { return s.ln.Addr().(*net.TCPAddr).Port }

// guestURL is the seed as the VM sees it.
func (s *seed) guestURL() string { return fmt.Sprintf("http://10.0.2.2:%d/%s", s.port(), s.token) }

func (s *seed) close() { _ = s.srv.Shutdown(context.Background()) }
