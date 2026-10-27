package sshclient

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"unicode"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const promptAttempts = 3

// authMethods tries keys (agent first, then identity files), then
// keyboard-interactive, then password, as OpenSSH does by default.
// done closes the agent connection.
func authMethods(t target, agentSocket string, p prompter, out io.Writer) (methods []ssh.AuthMethod, done func()) {
	done = func() {}
	var agentSigners []ssh.Signer
	if agentSocket != "" {
		if conn, err := net.Dial("unix", agentSocket); err == nil {
			done = func() { conn.Close() }
			agentSigners, _ = agent.NewClient(conn).Signers()
		}
	}
	keys := ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
		return append(agentSigners, identitySigners(t.Identities, p, out)...), nil
	})
	if _, cannotAsk := p.(noPrompter); cannotAsk {
		return []ssh.AuthMethod{keys}, done
	}
	interactive := ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
		for _, s := range []string{name, instruction} {
			if s != "" {
				fmt.Fprintln(out, visible(s))
			}
		}
		answers := make([]string, len(questions))
		for i, q := range questions {
			ask := p.Secret
			if echos[i] {
				ask = p.Line
			}
			var err error
			if answers[i], err = ask(visible(q)); err != nil {
				return nil, err
			}
		}
		return answers, nil
	})
	password := ssh.PasswordCallback(func() (string, error) {
		return p.Secret(fmt.Sprintf("%s@%s's password: ", t.User, t.Alias))
	})
	return []ssh.AuthMethod{
		keys,
		ssh.RetryableAuthMethod(interactive, promptAttempts),
		ssh.RetryableAuthMethod(password, promptAttempts),
	}, done
}

// identitySigners loads the private keys that exist among paths. A key with a
// passphrase is unlocked only when the server accepts its public half.
func identitySigners(paths []string, p prompter, out io.Writer) []ssh.Signer {
	var signers []ssh.Signer
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		signer, err := ssh.ParsePrivateKey(data)
		var missing *ssh.PassphraseMissingError
		switch {
		case err == nil:
			signers = append(signers, signer)
		case errors.As(err, &missing):
			key := &lockedKey{path: path, data: data, pub: missing.PublicKey, prompt: p}
			if key.pub == nil {
				key.pub = publicKeyFile(path + ".pub")
			}
			if key.pub != nil {
				signers = append(signers, key)
			} else if s, err := key.unlock(); err == nil {
				signers = append(signers, s)
			}
		default:
			fmt.Fprintf(out, "Load key %q: %v\n", path, err)
		}
	}
	return signers
}

func publicKeyFile(path string) ssh.PublicKey {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	key, _, _, _, err := ssh.ParseAuthorizedKey(data)
	if err != nil {
		return nil
	}
	return key
}

// lockedKey is an encrypted private key that asks for its passphrase the
// first time it signs.
type lockedKey struct {
	path   string
	data   []byte
	pub    ssh.PublicKey
	prompt prompter
	signer ssh.AlgorithmSigner
}

func (k *lockedKey) PublicKey() ssh.PublicKey { return k.pub }

func (k *lockedKey) Sign(rand io.Reader, data []byte) (*ssh.Signature, error) {
	s, err := k.unlock()
	if err != nil {
		return nil, err
	}
	return s.Sign(rand, data)
}

func (k *lockedKey) SignWithAlgorithm(rand io.Reader, data []byte, algorithm string) (*ssh.Signature, error) {
	s, err := k.unlock()
	if err != nil {
		return nil, err
	}
	return s.SignWithAlgorithm(rand, data, algorithm)
}

func (k *lockedKey) unlock() (ssh.AlgorithmSigner, error) {
	if k.signer != nil {
		return k.signer, nil
	}
	for range promptAttempts {
		pass, err := k.prompt.Secret(fmt.Sprintf("Enter passphrase for key '%s': ", k.path))
		if err != nil || pass == "" {
			return nil, fmt.Errorf("key %s left locked", k.path)
		}
		signer, err := ssh.ParsePrivateKeyWithPassphrase(k.data, []byte(pass))
		if err != nil {
			continue
		}
		as, ok := signer.(ssh.AlgorithmSigner)
		if !ok {
			return nil, fmt.Errorf("key %s cannot choose a signature algorithm", k.path)
		}
		k.signer = as
		return as, nil
	}
	return nil, fmt.Errorf("too many wrong passphrases for %s", k.path)
}

// visible shows control characters in server-supplied text as "?" (as
// OpenSSH does with strnvis), so a server cannot drive the local terminal.
func visible(s string) string {
	return strings.Map(func(r rune) rune {
		if r != '\n' && r != '\t' && unicode.IsControl(r) {
			return '?'
		}
		return r
	}, strings.ToValidUTF8(s, "?"))
}
