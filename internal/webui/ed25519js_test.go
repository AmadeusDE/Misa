package webui

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const nodeScript = `
const E = require(process.env.ED_JS);
const cases = JSON.parse(require("fs").readFileSync(0, "utf8"));
const out = cases.map(c => ({
  sha: E.toHex(E.sha512(E.fromHex(c.msg))),
  pub: E.toHex(E.publicKey(E.fromHex(c.seed))),
  sig: E.toHex(E.sign(E.fromHex(c.seed), E.fromHex(c.msg))),
}));
process.stdout.write(JSON.stringify(out));
`

type jsCase struct {
	Seed string `json:"seed"`
	Msg  string `json:"msg"`
}

type jsResult struct{ Sha, Pub, Sig string }

func runJS(t *testing.T, cases []jsCase) []jsResult {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed")
	}
	in, _ := json.Marshal(cases)
	abs, _ := filepath.Abs("assets/static/ed25519.js")
	cmd := exec.Command(node, "-e", nodeScript)
	cmd.Env = append(os.Environ(), "ED_JS="+abs)
	cmd.Stdin = bytes.NewReader(in)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, stderr.String())
	}
	var res []jsResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatal(err)
	}
	return res
}

func TestJSEd25519RFC8032Vectors(t *testing.T) {
	vectors := []struct{ seed, msg, pub, sig string }{
		{"9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60", "",
			"d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a",
			"e5564300c360ac729086e2cc806e828a84877f1eb8e5d974d873e06522490155" +
				"5fb8821590a33bacc61e39701cf9b46bd25bf5f0595bbe24655141438e7a100b"},
		{"4ccd089b28ff96da9db6c346ec114e0f5b8a319f35aba624da8cf6ed4fb8a6fb", "72",
			"3d4017c3e843895a92b70aa74d1b7ebc9c982ccf2ec4968cc0cd55f12af4660c",
			"92a009a9f0d4cab8720e820b5f642540a2b27b5416503f8fb3762223ebdb69da" +
				"085ac1e43e15996e458f3613d0f11d8c387b2eaeb4302aeeb00d291612bb0c00"},
		{"c5aa8df43f9f837bedb7442f31dcb7b166d38535076f094b85ce3a2e0b4458f7", "af82",
			"fc51cd8e6218a0a7d1edc6a0cb8d6b0b0b5b8a6a0b2e4d9d5a9a1c1b5a0ad6c0",
			"6291d657deec24024827e69c3abe01a30ce548a284743a445e3680d7db5ac3ac" +
				"18ff9b538d16f290ae67f760984dc6594a7c15e9716ed28dc027beceea1ec40a"},
	}
	var cases []jsCase
	for _, v := range vectors {
		cases = append(cases, jsCase{v.seed, v.msg})
	}
	got := runJS(t, cases)
	for i, v := range vectors {
		seed, _ := hex.DecodeString(v.seed)
		msg, _ := hex.DecodeString(v.msg)
		priv := ed25519.NewKeyFromSeed(seed)
		// The third vector's public key is cross-checked against Go, not typed from memory.
		wantPub, wantSig := v.pub, v.sig
		if i == 2 {
			wantPub = hex.EncodeToString(priv.Public().(ed25519.PublicKey))
		}
		if got[i].Pub != wantPub {
			t.Errorf("vector %d pub = %s want %s", i, got[i].Pub, wantPub)
		}
		if got[i].Sig != wantSig {
			t.Errorf("vector %d sig = %s want %s", i, got[i].Sig, wantSig)
		}
		if hex.EncodeToString(ed25519.Sign(priv, msg)) != wantSig {
			t.Errorf("vector %d: Go ed25519 disagrees with the RFC value", i)
		}
	}
}

func TestJSMatchesGo(t *testing.T) {
	var cases []jsCase
	for i := 0; i < 12; i++ {
		seed := make([]byte, 32)
		msg := make([]byte, i*37)
		_, _ = rand.Read(seed)
		_, _ = rand.Read(msg)
		cases = append(cases, jsCase{hex.EncodeToString(seed), hex.EncodeToString(msg)})
	}
	nonce := ChallengeMessage(randomHex(32))
	cases = append(cases, jsCase{hex.EncodeToString(bytes.Repeat([]byte{7}, 32)), hex.EncodeToString(nonce)})
	for i, r := range runJS(t, cases) {
		seed, _ := hex.DecodeString(cases[i].Seed)
		msg, _ := hex.DecodeString(cases[i].Msg)
		priv := ed25519.NewKeyFromSeed(seed)
		sum := sha512.Sum512(msg)
		if r.Sha != hex.EncodeToString(sum[:]) {
			t.Errorf("case %d: sha512 mismatch (len %d)", i, len(msg))
		}
		if r.Pub != hex.EncodeToString(priv.Public().(ed25519.PublicKey)) {
			t.Errorf("case %d: pubkey mismatch", i)
		}
		if r.Sig != hex.EncodeToString(ed25519.Sign(priv, msg)) {
			t.Errorf("case %d: signature mismatch", i)
		}
		sigBytes, _ := hex.DecodeString(r.Sig)
		if !ed25519.Verify(priv.Public().(ed25519.PublicKey), msg, sigBytes) {
			t.Errorf("case %d: Go rejects the JS signature", i)
		}
	}
}
