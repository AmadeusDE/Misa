package auth

import (
	"os"
	"path/filepath"
	"strings"
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
	if err != nil || st.Backend().Name() != "file" {
		t.Fatalf("err=%v backend=%v", err, st)
	}
	if filePreferred {
		if probed {
			t.Fatal("android auto probed the keyring")
		}
		return
	}
	if !probed {
		t.Fatal("auto did not probe the keyring")
	}
}

func TestAutoFileBackendUsesTempConfig(t *testing.T) {
	home := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	oldPref := filePreferred
	filePreferred = true
	probed := false
	oldProbe := probeKeyring
	probeKeyring = func() error {
		probed = true
		return nil
	}
	t.Cleanup(func() {
		filePreferred = oldPref
		probeKeyring = oldProbe
	})

	st, err := Open("auto")
	if err != nil {
		t.Fatal(err)
	}
	if probed || st.Backend().Name() != "file" {
		t.Fatalf("probed=%v backend=%s", probed, st.Backend().Name())
	}
	if err := st.Put("openrouter", Credential{Type: "api_key", Key: "sk-or-v1-abcdef0123456789"}); err != nil {
		t.Fatal(err)
	}
	path := st.Backend().(interface{ Path() string }).Path()
	if !strings.HasPrefix(path, cfg+string(os.PathSeparator)) {
		t.Fatalf("credentials path %s is outside the temp config dir", path)
	}
	stt, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if stt.Mode().Perm() != 0o600 {
		t.Fatalf("file perm %o, want 0600", stt.Mode().Perm())
	}
	dir, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if dir.Mode().Perm() != 0o700 {
		t.Fatalf("dir perm %o, want 0700", dir.Mode().Perm())
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != FileName {
		t.Fatalf("dir entries %+v, want only %s", entries, FileName)
	}
}
