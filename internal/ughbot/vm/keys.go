package vm

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// keys are the daemon's login key for its VMs and the host key the VMs are given, so the
// daemon knows exactly which key to expect.
type keys struct {
	login   ssh.Signer
	host    ssh.Signer
	hostPEM string
}

func loadKeys(dir string) (*keys, error) {
	login, _, err := loadKey(filepath.Join(dir, "id_ed25519"))
	if err != nil {
		return nil, err
	}
	host, hostPEM, err := loadKey(filepath.Join(dir, "host_ed25519"))
	if err != nil {
		return nil, err
	}
	return &keys{login: login, host: host, hostPEM: hostPEM}, nil
}

func loadKey(path string) (ssh.Signer, string, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		_, priv, gerr := ed25519.GenerateKey(rand.Reader)
		if gerr != nil {
			return nil, "", gerr
		}
		block, merr := ssh.MarshalPrivateKey(priv, "ughbot vm")
		if merr != nil {
			return nil, "", merr
		}
		raw = pem.EncodeToMemory(block)
		if err = os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
			err = os.WriteFile(path, raw, 0o600)
		}
	}
	if err != nil {
		return nil, "", err
	}
	s, err := ssh.ParsePrivateKey(raw)
	return s, string(raw), err
}

func authorizedLine(s ssh.Signer) string {
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(s.PublicKey())))
}
