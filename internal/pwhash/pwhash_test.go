package pwhash

import (
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestNormalizeMethod(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"md5", MethodMD5, true},
		{"MD5", MethodMD5, true},
		{"sha256", MethodSHA256, true},
		{"SHA256", MethodSHA256, true},
		{"sha-256", MethodSHA256, true},
		{"sha512", MethodSHA512, true},
		{"sha-512", MethodSHA512, true},
		{"des", "", false},
		{"", "", false},
		{"sha1", "", false},
	}
	for _, c := range cases {
		got, ok := NormalizeMethod(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizeMethod(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestMaxSaltLen(t *testing.T) {
	cases := map[string]int{MethodMD5: 8, MethodSHA256: 16, MethodSHA512: 16, "des": 0, "": 0}
	for method, want := range cases {
		if got := MaxSaltLen(method); got != want {
			t.Errorf("MaxSaltLen(%q) = %d, want %d", method, got, want)
		}
	}
}

func TestValidateSalt(t *testing.T) {
	cases := []struct {
		method, salt string
		ok           bool
	}{
		{MethodMD5, "abcdefgh", true},
		{MethodMD5, "ab.d/09Z", true},
		{MethodMD5, "abcdefghh", false},           // 9 > 8
		{MethodMD5, "", false},                    // empty
		{MethodMD5, "abc$def", false},             // '$' is not a salt character
		{MethodMD5, "rounds=1000$abcdefg", false}, // no rounds= for md5
		{MethodSHA256, "abcdefgh", true},
		{MethodSHA256, "./Aa0Bb1Cc2Dd3Ee", true},
		{MethodSHA256, "./Aa0Bb1Cc2Dd3EeF", false}, // 17 > 16
		{MethodSHA256, "rounds=10000$abcdefgh", true},
		{MethodSHA256, "rounds=abc$abcdefgh", false},
		{MethodSHA512, "short", true},
		{MethodSHA512, "", false},
		{MethodSHA512, "with space", false},
		{MethodSHA512, "rounds=1000$", false}, // no salt after the prefix
		{"des", "ab", false},                  // unknown method
	}
	for _, c := range cases {
		err := ValidateSalt(c.method, c.salt)
		if (err == nil) != c.ok {
			t.Errorf("ValidateSalt(%q, %q) = %v, want ok=%v", c.method, c.salt, err, c.ok)
		}
	}
}

func TestValidateSaltBadRounds(t *testing.T) {
	err := ValidateSalt(MethodSHA256, "rounds=xyz$abcdefgh")
	if !errors.Is(err, ErrBadRounds) {
		t.Fatalf("ValidateSalt bad rounds = %v, want ErrBadRounds", err)
	}
}

func TestRandomSalt(t *testing.T) {
	for _, method := range []string{MethodMD5, MethodSHA256, MethodSHA512} {
		got, err := RandomSalt(method, strings.NewReader(strings.Repeat("A", 32)))
		if err != nil {
			t.Fatalf("RandomSalt(%q): %v", method, err)
		}
		if len(got) != MaxSaltLen(method) {
			t.Errorf("RandomSalt(%q) = %q, want %d characters", method, got, MaxSaltLen(method))
		}
		for i := 0; i < len(got); i++ {
			if !isCryptChar(got[i]) {
				t.Errorf("RandomSalt(%q) = %q contains %q, outside the crypt alphabet", method, got, got[i])
			}
		}
	}
	if _, err := RandomSalt(MethodMD5, strings.NewReader("")); err == nil {
		t.Error("RandomSalt with a short reader returned no error")
	}
	if _, err := RandomSalt(MethodMD5, errReader{}); err == nil {
		t.Error("RandomSalt with a failing reader returned no error")
	}
	if _, err := RandomSalt("des", strings.NewReader(strings.Repeat("A", 32))); !errors.Is(err, ErrUnsupportedMethod) {
		t.Errorf("RandomSalt(des) = %v, want ErrUnsupportedMethod", err)
	}
	// The salt must not be predictable: two reads from the real generator differ.
	a, err := RandomSalt(MethodSHA512, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	b, err := RandomSalt(MethodSHA512, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Errorf("two random salts are identical: %q", a)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestCryptUnsupported(t *testing.T) {
	for _, setting := range []string{"", "abcdefgh", "$2$abcdefgh", "$9$abc", "$md5$abc"} {
		if _, err := Crypt("pw", setting); !errors.Is(err, ErrUnsupportedMethod) {
			t.Errorf("Crypt(%q) = %v, want ErrUnsupportedMethod", setting, err)
		}
	}
	if _, err := CryptMethod("des", "pw", "ab"); !errors.Is(err, ErrUnsupportedMethod) {
		t.Errorf("CryptMethod(des) = %v, want ErrUnsupportedMethod", err)
	}
	if _, err := Crypt("pw", "$5$rounds=x$abcdefgh"); !errors.Is(err, ErrBadRounds) {
		t.Errorf("Crypt with bad rounds = %v, want ErrBadRounds", err)
	}
}

func TestCryptMethodMatchesCrypt(t *testing.T) {
	cases := []struct{ method, salt string }{
		{MethodMD5, "abcdefgh"},
		{MethodMD5, "short"},
		{MethodSHA256, "abcdefgh"},
		{MethodSHA256, "rounds=1000$abcdefgh"},
		{MethodSHA512, "abcdefgh"},
		{MethodSHA512, "rounds=10000$abcdefgh"},
	}
	for _, c := range cases {
		viaSetting, err := Crypt("secret", "$"+methodID(c.method)+"$"+c.salt)
		if err != nil {
			t.Fatalf("Crypt(%q): %v", c.salt, err)
		}
		viaMethod, err := CryptMethod(c.method, "secret", c.salt)
		if err != nil {
			t.Fatalf("CryptMethod(%q): %v", c.method, err)
		}
		if viaSetting != viaMethod {
			t.Errorf("%s salt %q: Crypt = %s, CryptMethod = %s", c.method, c.salt, viaSetting, viaMethod)
		}
	}
}

func methodID(method string) string {
	switch method {
	case MethodMD5:
		return "1"
	case MethodSHA256:
		return "5"
	}
	return "6"
}

func TestMethodCryptRejectsWrongPrefix(t *testing.T) {
	if _, err := MD5Crypt("pw", "$5$abcdefgh"); err == nil {
		t.Error("MD5Crypt accepted a $5$ setting")
	}
	if _, err := SHA256Crypt("pw", "$6$abcdefgh"); err == nil {
		t.Error("SHA256Crypt accepted a $6$ setting")
	}
	if _, err := SHA512Crypt("pw", "$1$abcdefgh"); err == nil {
		t.Error("SHA512Crypt accepted a $1$ setting")
	}
	if _, err := SHA512Crypt("pw", "abcdefgh"); err == nil {
		t.Error("SHA512Crypt accepted a setting without a prefix")
	}
	got, err := MD5Crypt("password", "$1$abcdefgh")
	if err != nil || got != "$1$abcdefgh$G//4keteveJp0qb8z2DxG/" {
		t.Errorf("MD5Crypt = %q, %v", got, err)
	}
}

// TestFullHashAsSetting feeds a full previous hash back in as the setting, the
// way crypt(3) verifies a password: the salt ends at the next '$'.
func TestFullHashAsSetting(t *testing.T) {
	for _, method := range []string{MethodMD5, MethodSHA256, MethodSHA512} {
		settings := map[string]string{
			MethodMD5:    "$1$abcdefgh",
			MethodSHA256: "$5$rounds=1000$abcdefgh",
			MethodSHA512: "$6$abcdefgh",
		}
		first, err := Crypt("password", settings[method])
		if err != nil {
			t.Fatal(err)
		}
		again, err := Crypt("password", first)
		if err != nil {
			t.Fatalf("%s: Crypt with a full hash as the setting: %v", method, err)
		}
		if again != first {
			t.Errorf("%s: re-hashing the stored hash gave %s, want %s", method, again, first)
		}
	}
}

func TestEncodeCryptAndRepeatTo(t *testing.T) {
	digest := make([]byte, 3)
	digest[0], digest[1], digest[2] = 0, 1, 2
	// w = 0<<16 | 1<<8 | 2 = 258 = 0b100000010: the least significant six bits
	// come first, so the characters are index 2 ('0') then index 4 ('2').
	if got, want := encodeCrypt(digest, []encGroup{{0, 1, 2, 2}}, cryptAlphabet), "02"; got != want {
		t.Errorf("encodeCrypt = %q, want %q", got, want)
	}
	if got := encodeCrypt(digest, []encGroup{{lastGroup, lastGroup, lastGroup, 1}}, cryptAlphabet); got != "." {
		t.Errorf("encodeCrypt of padding = %q, want %q", got, ".")
	}
	if got, want := string(repeatTo([]byte("ab"), 5)), "ababa"; got != want {
		t.Errorf("repeatTo = %q, want %q", got, want)
	}
	if got := repeatTo([]byte("ab"), 0); len(got) != 0 {
		t.Errorf("repeatTo(0) = %q, want empty", got)
	}
}
