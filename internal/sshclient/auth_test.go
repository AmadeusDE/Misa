package sshclient

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

func writeKey(t *testing.T, dir, name, passphrase string) ssh.PublicKey {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	var block *pem.Block
	var err error
	if passphrase == "" {
		block, err = ssh.MarshalPrivateKey(priv, "")
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte(passphrase))
	}
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, name), pem.EncodeToMemory(block), 0o600)
	key, _ := ssh.NewPublicKey(pub)
	return key
}

func TestIdentitySigners(t *testing.T) {
	dir := t.TempDir()
	plain := writeKey(t, dir, "plain", "")
	locked := writeKey(t, dir, "locked", "hunter2")
	p := &scripted{answers: []string{"wrong", "hunter2"}}

	signers := identitySigners([]string{
		filepath.Join(dir, "missing"),
		filepath.Join(dir, "plain"),
		filepath.Join(dir, "locked"),
	}, p, os.Stderr)
	if len(signers) != 2 {
		t.Fatalf("signers = %d, want 2", len(signers))
	}
	if string(signers[0].PublicKey().Marshal()) != string(plain.Marshal()) {
		t.Error("plain key mismatch")
	}
	if string(signers[1].PublicKey().Marshal()) != string(locked.Marshal()) {
		t.Error("locked key public half mismatch")
	}
	if len(p.asked) != 0 {
		t.Fatalf("asked for a passphrase before signing: %q", p.asked)
	}

	sig, err := signers[1].Sign(rand.Reader, []byte("data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := locked.Verify([]byte("data"), sig); err != nil {
		t.Errorf("signature: %v", err)
	}
	if len(p.asked) != 2 {
		t.Errorf("asked %q, want a retry after the wrong passphrase", p.asked)
	}
	if _, err := signers[1].Sign(rand.Reader, []byte("again")); err != nil || len(p.asked) != 2 {
		t.Errorf("second sign asked again or failed: %v %q", err, p.asked)
	}
}

func TestVisibleHidesControlCharacters(t *testing.T) {
	if got := visible("Password for \x1b]0;pwned\a you:\n"); got != "Password for ?]0;pwned? you:\n" {
		t.Errorf("visible = %q", got)
	}
}
