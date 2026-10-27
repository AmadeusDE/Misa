package sshclient

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var errHostKey = errors.New("host key verification failed")

// hostKeys checks server keys against known_hosts files. New keys are
// trusted per StrictHostKeyChecking and written to the first file.
type hostKeys struct {
	files  []string
	strict string
	prompt prompter
	out    io.Writer
	db     ssh.HostKeyCallback
}

func newHostKeys(files []string, strict string, p prompter, out io.Writer) (*hostKeys, error) {
	h := &hostKeys{files: files, strict: strict, prompt: p, out: out}
	return h, h.reload()
}

func (h *hostKeys) reload() error {
	var existing []string
	for _, f := range h.files {
		if _, err := os.Stat(f); err == nil {
			existing = append(existing, f)
		}
	}
	db, err := knownhosts.New(existing...)
	if err != nil {
		return err
	}
	h.db = db
	return nil
}

func (h *hostKeys) Callback(hostname string, remote net.Addr, key ssh.PublicKey) error {
	err := h.db(hostname, remote, key)
	var ke *knownhosts.KeyError
	if !errors.As(err, &ke) {
		return err
	}
	if len(ke.Want) > 0 {
		h.warnChanged(hostname, key, ke.Want[0])
		return errHostKey
	}
	return h.trust(hostname, remote, key)
}

func (h *hostKeys) warnChanged(hostname string, key ssh.PublicKey, known knownhosts.KnownKey) {
	fmt.Fprintf(h.out, "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n"+
		"@    WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!     @\n"+
		"@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n"+
		"Someone could be eavesdropping on you right now, or the host key has just been changed.\n"+
		"The %s key sent by %s is\n%s.\n"+
		"Offending key in %s:%d\n",
		key.Type(), hostname, ssh.FingerprintSHA256(key), known.Filename, known.Line)
}

func (h *hostKeys) trust(hostname string, remote net.Addr, key ssh.PublicKey) error {
	switch h.strict {
	case "yes", "true":
		fmt.Fprintf(h.out, "No %s host key is known for %s and you have requested strict checking.\n", key.Type(), hostname)
		return errHostKey
	case "accept-new", "no", "off", "false":
	default:
		if ok, err := h.ask(hostname, remote, key); err != nil || !ok {
			return errHostKey
		}
	}
	if len(h.files) == 0 {
		return nil
	}
	if err := appendKnownHost(h.files[0], hostname, key); err != nil {
		fmt.Fprintf(h.out, "Failed to add the host to the list of known hosts (%s): %v\n", h.files[0], err)
		return nil
	}
	fmt.Fprintf(h.out, "Warning: Permanently added '%s' (%s) to the list of known hosts.\n", knownhosts.Normalize(hostname), key.Type())
	return h.reload()
}

func (h *hostKeys) ask(hostname string, remote net.Addr, key ssh.PublicKey) (bool, error) {
	if h.prompt == nil {
		return false, errHostKey
	}
	fmt.Fprintf(h.out, "The authenticity of host '%s (%s)' can't be established.\n%s key fingerprint is %s.\n",
		knownhosts.Normalize(hostname), remote, key.Type(), ssh.FingerprintSHA256(key))
	question := "Are you sure you want to continue connecting (yes/no)? "
	for {
		answer, err := h.prompt.Line(question)
		if err != nil {
			return false, err
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "yes":
			return true, nil
		case "no":
			return false, nil
		}
		question = "Please type 'yes' or 'no': "
	}
}

func appendKnownHost(file, hostname string, key ssh.PublicKey) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f, knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// Algorithms lists the host key algorithms of the keys already known for addr,
// so the server is asked for a key that can be verified. Nil means no
// preference: the host is new.
func (h *hostKeys) Algorithms(addr string) []string {
	err := h.db(addr, &net.TCPAddr{IP: net.IPv4zero}, probeKey{})
	var ke *knownhosts.KeyError
	if !errors.As(err, &ke) {
		return nil
	}
	var algos []string
	seen := map[string]bool{}
	for _, k := range ke.Want {
		for _, a := range algorithmsFor(k.Key.Type()) {
			if !seen[a] {
				seen[a] = true
				algos = append(algos, a)
			}
		}
	}
	return algos
}

func algorithmsFor(keyType string) []string {
	switch keyType {
	case ssh.KeyAlgoRSA:
		return []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA}
	case ssh.CertAlgoRSAv01:
		return []string{ssh.CertAlgoRSASHA512v01, ssh.CertAlgoRSASHA256v01, ssh.CertAlgoRSAv01}
	}
	return []string{keyType}
}

// probeKey matches no known key, so a lookup with it reports every key known
// for the host.
type probeKey struct{}

func (probeKey) Type() string                        { return "misa-probe" }
func (probeKey) Marshal() []byte                     { return []byte("misa-probe") }
func (probeKey) Verify([]byte, *ssh.Signature) error { return errors.New("probe key") }
