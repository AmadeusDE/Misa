package auth

import (
	"encoding/json"
	"strings"
	"testing"
)

// wincredLimit is Windows Credential Manager's per-entry blob cap, which
// go-keyring enforces (keyring_windows.go).
const wincredLimit = 2560

func serialized(t *testing.T, c *Credentials) int {
	t.Helper()
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	return len(raw)
}

// A ChatGPT/Codex OAuth record is a JWT access token (about 1 to 4 KiB with its
// claims), an opaque refresh token, an expiry and an account id. The keyring
// backend stores the whole document as one entry: a single record fits only
// while the access token stays under about 2.3 KB, and a second provider
// pushes a 2 KiB-token document over wincred's cap. The Windows default is
// therefore the protected file backend.
func TestOAuthRecordSizeVersusWincredCap(t *testing.T) {
	oauth := func(access int) Credential {
		return Credential{
			Type:      "oauth",
			Access:    strings.Repeat("a", access),
			Refresh:   "rt_" + strings.Repeat("r", 96),
			Expires:   1793000000000,
			AccountID: "123e4567-e89b-12d3-a456-426614174000",
		}
	}
	for _, c := range []struct {
		name   string
		access int
		over   bool
	}{
		{"small JWT", 900, false},
		{"2 KiB JWT", 2048, false},
		{"2.4 KB JWT", 2400, true},
		{"large JWT", 4096, true},
	} {
		d := newCredentials()
		d.Providers["codex"] = oauth(c.access)
		n := serialized(t, d)
		t.Logf("%s: access %d bytes, document %d bytes (wincred cap %d)", c.name, c.access, n, wincredLimit)
		if (n > wincredLimit) != c.over {
			t.Errorf("%s: %d bytes, over cap = %v, want %v", c.name, n, n > wincredLimit, c.over)
		}
	}

	d := newCredentials()
	d.Providers["codex"] = oauth(2048)
	d.Providers["openrouter"] = Credential{Type: "api_key", Key: strings.Repeat("k", 100)}
	d.Providers["opencode"] = Credential{Type: "api_key", Key: strings.Repeat("k", 100)}
	t.Logf("codex + two api keys: %d bytes", serialized(t, d))
	if serialized(t, d) <= wincredLimit {
		t.Error("expected the multi-provider document to exceed the wincred cap")
	}
	if serialized(t, d)+1 > maxFile {
		t.Errorf("document does not fit the file store's %d byte limit", maxFile)
	}
}
