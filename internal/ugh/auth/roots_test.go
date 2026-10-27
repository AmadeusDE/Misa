package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/roots"
)

func TestOpenInExplicitRootsUsesOwnFileWithoutSecretService(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	t.Setenv("HOME", t.TempDir())
	dir := filepath.Join(t.TempDir(), "bot")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := probeKeyring
	t.Cleanup(func() { probeKeyring = old })
	probeKeyring = func() error { return os.ErrNotExist }
	st, err := OpenIn(roots.Roots{Config: dir}, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if st.Backend().Name() != "file+file" {
		t.Fatalf("backend %q", st.Backend().Name())
	}
	if err := st.Put("openrouter", Credential{Type: "api_key", Key: "sk-or-test-0123456789"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); err != nil {
		t.Fatalf("credentials not in the bot's root: %v", err)
	}
}

func TestOpenInDefaultRootsStillProbesKeyring(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	probed := false
	old := probeKeyring
	probeKeyring = func() error { probed = true; return os.ErrNotExist }
	t.Cleanup(func() { probeKeyring = old })
	st, err := OpenIn(roots.Roots{}, "auto")
	if err != nil || !probed || st.Backend().Name() != "file" {
		t.Fatalf("probed=%v err=%v", probed, err)
	}
}
