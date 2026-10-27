package webui

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strings"
)

const (
	challengePrefix = "webui-login-v1:"
	sshKeyType      = "ssh-ed25519"
	authorizedIDLen = 16
)

// ChallengeMessage is exactly what a client signs for a login nonce.
func ChallengeMessage(nonce string) []byte { return []byte(challengePrefix + nonce) }

func SignChallenge(priv ed25519.PrivateKey, nonce string) []byte {
	return ed25519.Sign(priv, ChallengeMessage(nonce))
}

// AuthorizedKeyID is the device id used when logging in with a key from the
// authorized_keys file.
func AuthorizedKeyID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return "ak-" + hex.EncodeToString(sum[:])[:authorizedIDLen]
}

type AuthorizedKey struct {
	PubKey  ed25519.PublicKey
	Comment string
}

func (k AuthorizedKey) ID() string { return AuthorizedKeyID(k.PubKey) }

func MarshalAuthorizedKey(pub ed25519.PublicKey, comment string) string {
	var blob bytes.Buffer
	for _, part := range [][]byte{[]byte(sshKeyType), pub} {
		_ = binary.Write(&blob, binary.BigEndian, uint32(len(part)))
		blob.Write(part)
	}
	line := sshKeyType + " " + base64.StdEncoding.EncodeToString(blob.Bytes())
	if comment != "" {
		line += " " + comment
	}
	return line + "\n"
}

func parseAuthorizedLine(line string) (AuthorizedKey, bool) {
	fields := strings.Fields(line)
	for i, f := range fields {
		if f != sshKeyType || i+1 >= len(fields) {
			continue
		}
		blob, err := base64.StdEncoding.DecodeString(fields[i+1])
		if err != nil {
			return AuthorizedKey{}, false
		}
		typ, rest, ok := sshString(blob)
		key, _, ok2 := sshString(rest)
		if !ok || !ok2 || string(typ) != sshKeyType || len(key) != ed25519.PublicKeySize {
			return AuthorizedKey{}, false
		}
		return AuthorizedKey{PubKey: ed25519.PublicKey(key), Comment: strings.Join(fields[i+2:], " ")}, true
	}
	return AuthorizedKey{}, false
}

func sshString(b []byte) (s, rest []byte, ok bool) {
	if len(b) < 4 {
		return nil, nil, false
	}
	n := binary.BigEndian.Uint32(b)
	if uint64(len(b)-4) < uint64(n) {
		return nil, nil, false
	}
	return b[4 : 4+n], b[4+n:], true
}

// ParseAuthorizedKeys reads ssh-ed25519 lines, skipping comments, blanks and
// other key types.
func ParseAuthorizedKeys(r io.Reader) ([]AuthorizedKey, error) {
	var keys []AuthorizedKey
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, ok := parseAuthorizedLine(line); ok {
			keys = append(keys, k)
		}
	}
	return keys, sc.Err()
}

func loadAuthorizedKeys(path string) []AuthorizedKey {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	keys, _ := ParseAuthorizedKeys(f)
	return keys
}

// LoadOrCreateKey reads an ed25519 seed (base64) from path, creating the file
// (0600) when absent.
func LoadOrCreateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		seed := make([]byte, ed25519.SeedSize)
		if _, err := rand.Read(seed); err != nil {
			return nil, err
		}
		enc := base64.StdEncoding.EncodeToString(seed) + "\n"
		if err := os.WriteFile(path, []byte(enc), 0o600); err != nil {
			return nil, err
		}
		return ed25519.NewKeyFromSeed(seed), nil
	}
	if err != nil {
		return nil, err
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("webui: key file is not a base64 ed25519 seed")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}
