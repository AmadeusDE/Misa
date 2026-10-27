package sshclient

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

type scripted struct {
	answers []string
	asked   []string
}

func (s *scripted) next(prompt string) (string, error) {
	s.asked = append(s.asked, prompt)
	if len(s.answers) == 0 {
		return "", errors.New("no more answers")
	}
	a := s.answers[0]
	s.answers = s.answers[1:]
	return a, nil
}

func (s *scripted) Line(prompt string) (string, error)   { return s.next(prompt) }
func (s *scripted) Secret(prompt string) (string, error) { return s.next(prompt) }

func newEd25519(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

var remoteAddr = &net.TCPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 22}

func newTestHostKeys(t *testing.T, strict string, p prompter) (*hostKeys, string, *bytes.Buffer) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "ssh", "known_hosts")
	var out bytes.Buffer
	h, err := newHostKeys([]string{file}, strict, p, &out)
	if err != nil {
		t.Fatal(err)
	}
	return h, file, &out
}

func TestAcceptNewRemembersKey(t *testing.T) {
	h, file, out := newTestHostKeys(t, "accept-new", nil)
	key := newEd25519(t)
	if err := h.Callback("box:22", remoteAddr, key); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Permanently added") {
		t.Errorf("out = %q", out)
	}
	data, _ := os.ReadFile(file)
	if !strings.HasPrefix(string(data), "box ssh-ed25519 ") {
		t.Errorf("known_hosts = %q", data)
	}

	again, _, _ := newTestHostKeys(t, "yes", nil)
	again.files = []string{file}
	again.reload()
	if err := again.Callback("box:22", remoteAddr, key); err != nil {
		t.Errorf("remembered key rejected: %v", err)
	}
}

func TestNonStandardPortIsBracketed(t *testing.T) {
	h, file, _ := newTestHostKeys(t, "accept-new", nil)
	h.Callback("box:2222", remoteAddr, newEd25519(t))
	data, _ := os.ReadFile(file)
	if !strings.HasPrefix(string(data), "[box]:2222 ") {
		t.Errorf("known_hosts = %q", data)
	}
}

func TestChangedKeyIsRefused(t *testing.T) {
	h, _, out := newTestHostKeys(t, "accept-new", nil)
	h.Callback("box:22", remoteAddr, newEd25519(t))
	h.reload()
	if err := h.Callback("box:22", remoteAddr, newEd25519(t)); err == nil {
		t.Fatal("changed key accepted")
	}
	if !strings.Contains(out.String(), "HOST IDENTIFICATION HAS CHANGED") {
		t.Errorf("out = %q", out)
	}
}

func TestStrictRefusesUnknown(t *testing.T) {
	h, file, _ := newTestHostKeys(t, "yes", nil)
	if err := h.Callback("box:22", remoteAddr, newEd25519(t)); err == nil {
		t.Fatal("unknown key accepted")
	}
	if _, err := os.Stat(file); err == nil {
		t.Error("known_hosts written")
	}
}

func TestAskPrompts(t *testing.T) {
	no := &scripted{answers: []string{"no"}}
	h, _, _ := newTestHostKeys(t, "ask", no)
	if err := h.Callback("box:22", remoteAddr, newEd25519(t)); err == nil {
		t.Error("declined key accepted")
	}
	if len(no.asked) != 1 || !strings.Contains(no.asked[0], "(yes/no") {
		t.Errorf("asked %q", no.asked)
	}

	yes := &scripted{answers: []string{"maybe", "yes"}}
	h, file, _ := newTestHostKeys(t, "ask", yes)
	if err := h.Callback("box:22", remoteAddr, newEd25519(t)); err != nil {
		t.Errorf("accepted key refused: %v", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Error("known_hosts not written")
	}
}

func TestAlgorithmsFollowKnownKeys(t *testing.T) {
	h, _, _ := newTestHostKeys(t, "accept-new", nil)
	if got := h.Algorithms("box:22"); got != nil {
		t.Errorf("unknown host algorithms = %q", got)
	}

	rsaKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	pub, _ := ssh.NewPublicKey(&rsaKey.PublicKey)
	h.Callback("box:22", remoteAddr, pub)

	want := []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA}
	if got := h.Algorithms("box:22"); !reflect.DeepEqual(got, want) {
		t.Errorf("algorithms = %q, want %q", got, want)
	}
}
